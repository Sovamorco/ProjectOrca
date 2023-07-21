package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"ProjectOrca/utils"

	"google.golang.org/protobuf/types/known/emptypb"

	"ProjectOrca/migrations"
	"ProjectOrca/models"
	pb "ProjectOrca/proto"
	"ProjectOrca/store"

	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type DeathrattleMessage struct {
	Deceased string `json:"deceased"`
}

type ResyncTarget int

type ResyncMessage struct {
	Bot     string         `json:"bot"`
	Guild   string         `json:"guild"`
	Targets []ResyncTarget `json:"targets"`
	// the only way I found to properly do seeking
	// if I don't pass it here storeLoop overwrites changes made by the seek call
	SeekPos time.Duration `json:"seek_pos"`
}

const (
	MigrationsTimeout   = time.Second * 300
	DeathrattlesChannel = "deathrattles"
	ResyncsChannel      = "resync"

	edgeOrdKeyDiff    = 100
	defaultPrevOrdKey = 0
	defaultNextOrdKey = defaultPrevOrdKey + edgeOrdKeyDiff
)

// resync targets.
const (
	ResyncTargetCurrent ResyncTarget = iota
	ResyncTargetGuild
)

var ErrInternal = status.Error(codes.Internal, "internal error")

type orcaServer struct {
	pb.UnimplementedOrcaServer `exhaustruct:"optional"`

	id     string
	states []*models.Bot
	logger *zap.SugaredLogger
	store  *store.Store
}

func newOrcaServer(logger *zap.SugaredLogger, st *store.Store) *orcaServer {
	serverLogger := logger.Named("server")

	return &orcaServer{
		id:     uuid.New().String(),
		states: make([]*models.Bot, 0),
		logger: serverLogger,
		store:  st,
	}
}

func (o *orcaServer) gracefulShutdown(ctx context.Context) {
	for _, state := range o.states {
		state.GracefulShutdown()
	}

	_, err := o.store.
		NewUpdate().
		Model((*models.RemoteBot)(nil)).
		Where("locker = ?", o.id).
		Set("locker = NULL").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error unlocking bots: %+v", err)
	}

	o.store.Unsubscribe(ctx)

	o.sendDeathrattle(ctx)

	o.store.GracefulShutdown()
}

func (o *orcaServer) initFromStore(ctx context.Context) error {
	_, err := o.store.
		NewUpdate().
		Model((*models.RemoteBot)(nil)).
		Where("locker IS NULL").
		Set("locker = ?", o.id).
		Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "lock states")
	}

	states := make([]*models.RemoteBot, 0)

	err = o.store.
		NewSelect().
		Model(&states).
		Where("locker = ?", o.id).
		Scan(ctx)
	if err != nil {
		return errorx.Decorate(err, "list bot states")
	}

	for _, state := range states {
		if s := o.getStateByToken(state.Token); s != nil {
			// state already controlled, no need to init
			continue
		}

		localState, err := models.NewBot(o.logger, o.store, state.Token)
		if err != nil {
			o.logger.Errorf("Could not create local state for remote state %s: %+v", state.ID, err)
		}

		o.states = append(o.states, localState)

		go localState.FullResync(ctx)
	}

	return nil
}

func (o *orcaServer) getStateByToken(token string) *models.Bot {
	for _, state := range o.states {
		if state.GetToken() == token {
			return state
		}
	}

	return nil
}

// do not use for authentication-related purposes.
func (o *orcaServer) getStateByID(id string) *models.Bot {
	for _, state := range o.states {
		if state.GetID() == id {
			return state
		}
	}

	return nil
}

func (o *orcaServer) parseIncomingContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		o.logger.Debug("missing metadata")

		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	tokenS := md.Get("token")
	if len(tokenS) != 1 {
		o.logger.Debug("missing token")

		return "", status.Error(codes.Unauthenticated, "missing token")
	}

	token := tokenS[0]

	return token, nil
}

func (o *orcaServer) authenticate(ctx context.Context) (*models.RemoteBot, error) {
	token, err := o.parseIncomingContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse context")
	}

	var s models.RemoteBot

	err = o.store.
		NewSelect().
		Model(&s).
		Where("token = ?", token).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return o.register(ctx, token)
		}

		return nil, errorx.Decorate(err, "get state from store")
	}

	return &s, nil
}

func (o *orcaServer) authenticateWithGuild(ctx context.Context, guildID string) (
	*models.RemoteBot,
	*models.RemoteGuild,
	error,
) {
	bot, err := o.authenticate(ctx)
	if err != nil {
		return nil, nil, errorx.Decorate(err, "authenticate bot")
	}

	var r models.RemoteGuild
	r.ID = guildID
	r.BotID = bot.ID

	err = o.store.
		NewSelect().
		Model(&r).
		WherePK().
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			g := models.NewRemoteGuild(bot.ID, guildID)

			_, err = o.store.
				NewInsert().
				Model(g).
				Exec(ctx)
			if err != nil {
				return nil, nil, errorx.Decorate(err, "create guild")
			}

			return bot, g, nil
		}

		return nil, nil, errorx.Decorate(err, "get guild from store")
	}

	return bot, &r, nil
}

func (o *orcaServer) register(ctx context.Context, token string) (*models.RemoteBot, error) {
	state, err := models.NewBot(o.logger, o.store, token)
	if err != nil {
		return nil, errorx.Decorate(err, "create local bot state")
	}

	r := models.NewRemoteBot(state.GetID(), state.GetToken(), o.id)

	_, err = o.store.
		NewInsert().
		Model(r).
		Exec(ctx)
	if err != nil {
		state.GracefulShutdown()

		return nil, errorx.Decorate(err, "store remote bot state")
	}

	o.states = append(o.states, state)

	return r, nil
}

func (o *orcaServer) Join(ctx context.Context, in *pb.JoinRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, status.Error(codes.Unauthenticated, "failed to authenticate bot")
	}

	if guild.ChannelID == in.ChannelID {
		return &emptypb.Empty{}, nil
	}

	_, err = guild.UpdateQuery(o.store).
		Set("channel_id = ?", in.ChannelID).
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating guild channel id: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync request: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) Leave(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, status.Error(codes.Unauthenticated, "failed to authenticate bot")
	}

	if guild.ChannelID == "" {
		return &emptypb.Empty{}, nil
	}

	_, err = guild.
		UpdateQuery(o.store).
		Set("channel_id = NULL").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating guild: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync request: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) Play(ctx context.Context, in *pb.PlayRequest) (*pb.PlayReply, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, status.Error(codes.Unauthenticated, "failed to authenticate bot")
	}

	tracks, err := models.NewRemoteTracks(o.logger, bot.ID, guild.ID, in.Url)
	if err != nil {
		o.logger.Errorf("Error getting remote tracks: %+v", err)

		return nil, ErrInternal
	}

	qlen, err := o.store.
		NewSelect().
		Model((*models.RemoteTrack)(nil)).
		Where("bot_id = ? AND guild_id = ?", bot.ID, guild.ID).
		Count(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		o.logger.Errorf("Error getting tracks from store: %+v", err)

		return nil, ErrInternal
	}

	prevOrdKey, nextOrdKey := o.chooseOrdKeyRange(ctx, qlen, int(in.Position))

	models.SetOrdKeys(tracks, prevOrdKey, nextOrdKey)

	_, err = o.store.
		NewInsert().
		Model(&tracks).
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error storing tracks: %+v", err)

		return nil, ErrInternal
	}

	res := make([]*pb.TrackData, len(tracks))
	for i, track := range tracks {
		res[i] = track.ToProto()
	}

	// that concerns current track
	if qlen == 0 || in.Position == 0 {
		err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)

		if err != nil {
			o.logger.Errorf("Error sending resync message: %+v", err)

			return nil, ErrInternal
		}
	}

	return &pb.PlayReply{
		Tracks: res,
	}, nil
}

func (o *orcaServer) Skip(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, status.Error(codes.Unauthenticated, "failed to authenticate bot")
	}

	o.logger.Infof("Skipping track in guild %s", in.GuildID)

	var current models.RemoteTrack

	err = o.store.
		NewSelect().
		Model(&current).
		Where("bot_id = ? AND guild_id = ?", bot.ID, guild.ID).
		Order("ord_key").
		Limit(1).
		Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		o.logger.Errorf("Error getting current track: %+v", err)

		return nil, ErrInternal
	}

	err = current.DeleteOrRequeue(ctx, o.store)
	if err != nil {
		o.logger.Errorf("Error deleting or requeuing current track: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
	if err != nil {
		o.logger.Errorf("Error sending resync message: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) Stop(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, status.Error(codes.Unauthenticated, "failed to authenticate bot")
	}

	o.logger.Infof("Stopping playback in guild %s", in.GuildID)

	_, err = o.store.
		NewDelete().
		Model((*models.RemoteTrack)(nil)).
		Where("bot_id = ? AND guild_id = ?", bot.ID, guild.ID).
		Exec(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		o.logger.Errorf("Error deleting all tracks: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
	if err != nil {
		o.logger.Errorf("Error sending resync message: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) Seek(ctx context.Context, in *pb.SeekRequest) (*pb.SeekReply, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, status.Error(codes.Unauthenticated, "failed to authenticate bot")
	}

	o.logger.Infof("Seeking to %.2fs in guild %s", in.Position.AsDuration().Seconds(), in.GuildID)

	_, err = o.store.
		NewUpdate().
		Model((*models.RemoteTrack)(nil)).
		ModelTableExpr("tracks").
		TableExpr("(?) as curr", guild.CurrentTrackQuery(o.store)).
		Set("tracks.pos = ?", in.Position.AsDuration()).
		Where("tracks.id = curr.id").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error changing track position: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResyncSeek(ctx, bot.ID, guild.ID, in.Position.AsDuration(), ResyncTargetCurrent)
	if err != nil {
		o.logger.Errorf("Error sending resync target")

		return nil, ErrInternal
	}

	return &pb.SeekReply{}, nil
}

func (o *orcaServer) Loop(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	_, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, status.Error(codes.Unauthenticated, "failed to authenticate bot")
	}

	_, err = guild.UpdateQuery(o.store).
		Set("`loop` = NOT `loop`").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating loop state: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) ShuffleQueue(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, status.Error(codes.Unauthenticated, "failed to authenticate bot")
	}

	_, err = o.store.
		NewUpdate().
		Model((*models.RemoteTrack)(nil)).
		ModelTableExpr("tracks").
		TableExpr(
			"(?) AS curr",
			o.store.
				NewSelect().
				Model((*models.RemoteTrack)(nil)).
				Column("id", "ord_key").
				Where("bot_id = ? AND guild_id = ?", bot.ID, guild.ID).
				Order("ord_key").
				Limit(1),
		).
		Set("tracks.ord_key = RAND() * ? + 1 + curr.ord_key", edgeOrdKeyDiff).
		Where("bot_id = ? AND guild_id = ? AND tracks.id != curr.id", bot.ID, guild.ID).
		Exec(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &emptypb.Empty{}, nil
		}

		o.logger.Errorf("Error updating ord keys: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) GetTracks(ctx context.Context, in *pb.GetTracksRequest) (*pb.GetTracksReply, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, status.Error(codes.Unauthenticated, "failed to authenticate bot")
	}

	var tracks []*models.RemoteTrack

	countQuery := o.store.
		NewSelect().
		Model(&tracks).
		Where("bot_id = ? AND guild_id = ?", bot.ID, guild.ID)

	qlen, err := countQuery.Count(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		o.logger.Errorf("Error getting queue length: %+v", err)

		return nil, ErrInternal
	}

	err = countQuery.
		Order("ord_key").
		Offset(int(in.Start)).
		Limit(int(in.End - in.Start)).
		Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		o.logger.Errorf("Error getting tracks: %+v", err)

		return nil, ErrInternal
	}

	res := make([]*pb.TrackData, len(tracks))
	for i, track := range tracks {
		res[i] = track.ToProto()
	}

	return &pb.GetTracksReply{
		Tracks:      res,
		TotalTracks: int64(qlen),
		Looping:     guild.Loop,
	}, nil
}

func (o *orcaServer) Pause(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, status.Error(codes.Unauthenticated, "failed to authenticate bot")
	}

	_, err = guild.
		UpdateQuery(o.store).
		Set("paused = true").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating pause state: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) Resume(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, status.Error(codes.Unauthenticated, "failed to authenticate bot")
	}

	_, err = guild.
		UpdateQuery(o.store).
		Set("paused = false").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating pause state: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func main() {
	coreLogger, err := zap.NewDevelopment(zap.IncreaseLevel(zapcore.DebugLevel))
	if err != nil {
		panic(err)
	}

	logger := coreLogger.Sugar()

	err = run(context.Background(), logger)
	if err != nil {
		logger.Fatalf("%+v", err)
	}
}

func doMigrate(ctx context.Context, logger *zap.SugaredLogger, db *bun.DB) error {
	logger = logger.Named("migrate")

	migrateContext, migrateCancel := context.WithTimeout(ctx, MigrationsTimeout)
	defer migrateCancel()

	m, err := migrations.NewMigrations()
	if err != nil {
		return errorx.Decorate(err, "get migrations")
	}

	migrator := migrate.NewMigrator(db, m)

	err = migrator.Init(migrateContext)
	if err != nil {
		return errorx.Decorate(err, "migrate init")
	}

	if err = migrator.Lock(ctx); err != nil {
		return errorx.Decorate(err, "lock")
	}

	defer func(migrator *migrate.Migrator, ctx context.Context) {
		err := migrator.Unlock(ctx)
		if err != nil {
			logger.Errorf("Error unlocking migrator: %+v", err)
		}
	}(migrator, ctx)

	group, err := migrator.Migrate(ctx)
	if err != nil {
		return errorx.Decorate(err, "migrate")
	}

	if group.IsZero() {
		logger.Debug("No migrations to run")
	} else {
		logger.Infof("Ran %d migrations", len(group.Migrations))
	}

	return nil
}

func run(ctx context.Context, logger *zap.SugaredLogger) error { //nolint:funlen // fixme
	st, err := store.NewStore(logger, &store.Config{
		DB: store.DBConfig{
			Host:     "127.0.0.1",
			Port:     3306, //nolint:gomnd // TODO: proper config
			User:     "root",
			Password: "local",
			DB:       "orca",
		},
		Broker: store.RedisConfig{
			Host:  "127.0.0.1",
			Port:  6379, //nolint:gomnd // TODO: proper config
			Token: "local",
			DB:    0,
		},
	})
	if err != nil {
		return errorx.Decorate(err, "create store")
	}

	err = doMigrate(ctx, logger, st.DB)
	if err != nil {
		return errorx.Decorate(err, "do migrations")
	}

	port := 8590

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return errorx.Decorate(err, "listen")
	}

	orca := newOrcaServer(logger, st)

	err = orca.initFromStore(ctx)
	if err != nil {
		return errorx.Decorate(err, "init from store")
	}

	psdr := orca.store.Subscribe(ctx, DeathrattlesChannel)
	go orca.handleDeathrattle(ctx, psdr.Channel())

	psrs := orca.store.Subscribe(ctx, ResyncsChannel)
	go orca.handleResync(ctx, psrs.Channel())

	grpcServer := grpc.NewServer()
	pb.RegisterOrcaServer(grpcServer, orca)

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- grpcServer.Serve(lis)
	}()

	defer grpcServer.GracefulStop()

	defer orca.gracefulShutdown(ctx)

	logger.Infof("Started gRPC server on port %d", port)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	select {
	case err = <-serverErrors:
		logger.Errorf("Error listening for gRPC calls: %+v", err)

		return errorx.Decorate(err, "listen")
	case sig := <-sc:
		logger.Infof("Exiting with signal %v", sig)

		return nil
	}
}

func (o *orcaServer) handleDeathrattle(ctx context.Context, ch <-chan *redis.Message) {
	logger := o.logger.Named("sub_deathrattles")

	var wg sync.WaitGroup

	for msg := range ch {
		msg := msg

		wg.Add(1)

		go func() {
			defer wg.Done()

			logger.Debugf("Received deathrattle: %s", msg.Payload)

			dm := new(DeathrattleMessage)

			err := json.Unmarshal([]byte(msg.Payload), dm)
			if err != nil {
				logger.Errorf("Error unmarshalling deathrattle: %+v", err)

				return
			}

			o.logger.Infof("Handling deathrattle from %s", dm.Deceased)

			err = o.initFromStore(ctx)
			if err != nil {
				logger.Errorf("Error reinitializing after deathrattle: %+v", err)
			}
		}()
	}

	wg.Wait()
}

func (o *orcaServer) sendDeathrattle(ctx context.Context) {
	b, err := json.Marshal(DeathrattleMessage{
		Deceased: o.id,
	})
	if err != nil {
		o.logger.Errorf("Error marshalling deathrattle: %+v", err)

		return
	}

	err = o.store.Publish(ctx, DeathrattlesChannel, b).Err()
	if err != nil {
		o.logger.Errorf("Error publishing deathrattle: %+v", err)
	}
}

func (o *orcaServer) doResync(ctx context.Context, managed *models.Bot, r *ResyncMessage) {
	var wg sync.WaitGroup

	for _, target := range r.Targets {
		target := target

		wg.Add(1)

		go func() {
			defer wg.Done()

			switch target {
			case ResyncTargetGuild:
				err := managed.ResyncGuild(ctx, r.Guild)
				if err != nil {
					o.logger.Errorf("Error resyncing guild: %+v", err)
				}
			case ResyncTargetCurrent:
				managed.ResyncGuildTrack(ctx, r.Guild, r.SeekPos)
			}
		}()
	}

	wg.Wait()
}

func (o *orcaServer) handleResync(ctx context.Context, ch <-chan *redis.Message) {
	logger := o.logger.Named("sub_resync")

	var wg sync.WaitGroup

	for msg := range ch {
		msg := msg

		wg.Add(1)

		go func() {
			defer wg.Done()

			logger.Debugf("Received resync: %s", msg.Payload)

			r := new(ResyncMessage)

			err := json.Unmarshal([]byte(msg.Payload), r)
			if err != nil {
				logger.Errorf("Error unmarshalling resync: %+v", err)

				return
			}

			managed := o.getStateByID(r.Bot)
			if managed == nil {
				logger.Debugf("Resync message for non-managed recipient %s, skipping", r.Bot)

				return
			}

			o.doResync(ctx, managed, r)
		}()
	}

	wg.Wait()
}

func (o *orcaServer) sendResync(ctx context.Context, botID, guildID string, targets ...ResyncTarget) error {
	return o.sendResyncSeek(ctx, botID, guildID, utils.MinDuration, targets...)
}

func (o *orcaServer) sendResyncSeek(
	ctx context.Context,
	botID, guildID string,
	seekPos time.Duration,
	targets ...ResyncTarget,
) error {
	b, err := json.Marshal(ResyncMessage{
		Bot:     botID,
		Guild:   guildID,
		Targets: targets,
		SeekPos: seekPos,
	})
	if err != nil {
		return errorx.Decorate(err, "marshal resync message")
	}

	err = o.store.Publish(ctx, ResyncsChannel, b).Err()
	if err != nil {
		return errorx.Decorate(err, "publish resync message")
	}

	return nil
}

func (o *orcaServer) chooseOrdKeyRange(ctx context.Context, qlen, position int) (float64, float64) {
	// special case - if no tracks in queue - insert all tracks with ordkeys from 0 to 100
	if qlen == 0 {
		return defaultPrevOrdKey, defaultNextOrdKey
	}

	// if position is negative, interpret that as index from end (wrap around)
	// e.g. -1 means put track as last, -2 means put track as before last, etc.
	if position < 0 {
		position = qlen + position + 1
	}

	// position has to be at least 0 and at most len(queue)
	position = max(0, min(qlen, position))

	rows, err := o.store.
		NewSelect().
		Model((*models.RemoteTrack)(nil)).
		Column("ord_key").
		Order("ord_key").
		Offset(max(position-1, 0)). // have a separate check for position 0 later
		Limit(2).                   //nolint:gomnd // previous and next track in theory
		Rows(ctx)
	if err == nil {
		err = rows.Err()
	}

	if err != nil {
		o.logger.Errorf("Error getting ordkeys from store: %+v", err)

		return defaultPrevOrdKey, defaultNextOrdKey
	}

	defer func() {
		err = rows.Close()
		if err != nil {
			o.logger.Errorf("Error closing rows: %+v", err)
		}
	}()

	return o.chooseOrdKeyRangeFromRows(position, rows)
}

func (o *orcaServer) chooseOrdKeyRangeFromRows(position int, rows *sql.Rows) (float64, float64) {
	var prevOrdKey, nextOrdKey float64

	rows.Next()

	// special case - there is no track preceding the position, prevOrdKey should be nextOrdKey - 100
	if position == 0 {
		err := rows.Scan(&nextOrdKey)
		if err != nil {
			o.logger.Errorf("Error scanning track 0 from store: %+v", err)

			return defaultPrevOrdKey, defaultNextOrdKey
		}

		return nextOrdKey - edgeOrdKeyDiff, nextOrdKey
	}

	err := rows.Scan(&prevOrdKey)
	if err != nil {
		o.logger.Errorf("Error scanning prev track from store: %+v", err)

		return defaultPrevOrdKey, defaultNextOrdKey
	}

	// special case - there is no track succeeding the position, nextOrdKey should be prevOrdKey + 100
	if !rows.Next() {
		nextOrdKey = prevOrdKey + edgeOrdKeyDiff
	} else {
		err = rows.Scan(&nextOrdKey)
		if err != nil {
			o.logger.Errorf("Error scanning next track from store: %+v", err)

			return defaultPrevOrdKey, defaultNextOrdKey
		}
	}

	return prevOrdKey, nextOrdKey
}
