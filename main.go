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
	"strings"
	"sync"
	"syscall"
	"time"

	"ProjectOrca/vk"

	"ProjectOrca/spotify"
	"ProjectOrca/ytdl"

	"github.com/hashicorp/vault-client-go"

	"github.com/go-redsync/redsync/v4"

	"ProjectOrca/extractor"
	"google.golang.org/protobuf/types/known/emptypb"

	"ProjectOrca/migrations"
	"ProjectOrca/models"
	pb "ProjectOrca/proto"
	"ProjectOrca/store"

	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"github.com/redis/go-redis/v9"
	gvault "github.com/sovamorco/gommon/vault"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ResyncTarget int

type ResyncMessage struct {
	Bot     string         `json:"bot"`
	Guild   string         `json:"guild"`
	Targets []ResyncTarget `json:"targets"`
}

const (
	MigrationsTimeout = time.Second * 300
	ResyncsChannel    = "resync"

	edgeOrdKeyDiff    = 100
	defaultPrevOrdKey = 0
	defaultNextOrdKey = defaultPrevOrdKey + edgeOrdKeyDiff
)

// resync targets.
const (
	ResyncTargetCurrent ResyncTarget = iota
	ResyncTargetGuild
)

var (
	ErrFailedToAuthenticate = status.Error(codes.Unauthenticated, "failed to authenticate bot")
	ErrInternal             = status.Error(codes.Internal, "internal error")
)

type orcaServer struct {
	pb.UnimplementedOrcaServer `exhaustruct:"optional"`

	// static values
	id         string
	logger     *zap.SugaredLogger
	store      *store.Store
	extractors *extractor.Extractors

	// concurrency-safe
	states sync.Map
}

func newOrcaServer(
	ctx context.Context, logger *zap.SugaredLogger, st *store.Store, config *Config,
) (*orcaServer, error) {
	serverLogger := logger.Named("server")

	orca := &orcaServer{
		id:         uuid.New().String(),
		logger:     serverLogger,
		store:      st,
		extractors: extractor.NewExtractors(),

		states: sync.Map{},
	}

	if config.Spotify != nil {
		s, err := spotify.New(
			ctx,
			config.Spotify.ClientID,
			config.Spotify.ClientSecret,
		)
		if err != nil {
			return nil, errorx.Decorate(err, "init spotify module")
		}

		orca.extractors.AddExtractor(s)
	}

	if config.VK != nil {
		v, err := vk.New(
			ctx,
			orca.logger,
			config.VK.Token,
		)
		if err != nil {
			return nil, errorx.Decorate(err, "init vk module")
		}

		orca.extractors.AddExtractor(v)
	}

	orca.extractors.AddExtractor(ytdl.New(orca.logger))

	orca.store.Subscribe(ctx, orca.handleResync, fmt.Sprintf("%s:%s", config.Redis.Prefix, ResyncsChannel))
	orca.store.Subscribe(ctx, orca.handleKeyDel,
		fmt.Sprintf("__keyevent@0__:del"),
		fmt.Sprintf("__keyevent@0__:expired"),
	)

	err := orca.initFromStore(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "init from store")
	}

	return orca, nil
}

func (o *orcaServer) gracefulShutdown(ctx context.Context) {
	o.logger.Info("Shutting down")

	o.states.Range(func(_, value any) bool {
		o.logger.Debug("test")

		state, _ := value.(*models.Bot)

		o.logger.Debug("Shutting down state ", state)
		o.logger.Debugf("ID %s", state.GetID())

		state.GracefulShutdown()

		return true
	})

	o.store.Unsubscribe(ctx)

	o.store.GracefulShutdown(ctx)
}

func (o *orcaServer) initFromStore(ctx context.Context) error {
	var bots []*models.RemoteBot

	err := o.store.
		NewSelect().
		Model(&bots).
		Column("id", "token").
		Scan(ctx)
	if err != nil {
		return errorx.Decorate(err, "select bots")
	}

	for _, state := range bots {
		err = o.store.Lock(ctx, state.ID)
		if err != nil {
			var et *redsync.ErrTaken

			if !errors.As(err, &et) {
				o.logger.Errorf("Error locking state: %+v", err)
			}

			continue
		}

		o.logger.Infof("Locked state %s", state.ID)

		if s := o.getStateByToken(state.Token); s != nil {
			// state already controlled, no need to init
			continue
		}

		localState, err := models.NewBot(o.logger, o.store, o.extractors, state.Token)
		if err != nil {
			o.logger.Errorf("Could not create local state for remote state %s: %+v", state.ID, err)
		}

		o.states.Store(localState.GetID(), localState)

		go localState.FullResync(ctx)
	}

	return nil
}

func (o *orcaServer) getStateByToken(token string) *models.Bot {
	var res *models.Bot

	o.states.Range(func(_, value any) bool {
		state, _ := value.(*models.Bot)
		if state.GetToken() == token {
			res = state

			return false
		}

		return true
	})

	return res
}

// do not use for authentication-related purposes.
func (o *orcaServer) getStateByID(id string) *models.Bot {
	val, ok := o.states.Load(id)
	if !ok {
		return nil
	}

	state, _ := val.(*models.Bot)

	return state
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
	state, err := models.NewBot(o.logger, o.store, o.extractors, token)
	if err != nil {
		return nil, errorx.Decorate(err, "create local bot state")
	}

	r := models.NewRemoteBot(state.GetID(), state.GetToken())

	_, err = o.store.
		NewInsert().
		Model(r).
		Exec(ctx)
	if err != nil {
		state.GracefulShutdown()

		return nil, errorx.Decorate(err, "store remote bot state")
	}

	o.states.Store(state.GetID(), state)

	return r, nil
}

func (o *orcaServer) Join(ctx context.Context, in *pb.JoinRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
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

		return nil, ErrFailedToAuthenticate
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

		return nil, ErrFailedToAuthenticate
	}

	tracksData, affectedFirst, err := o.addTracks(ctx, bot, guild, in.Url, int(in.Position))
	if err != nil {
		o.logger.Errorf("Error adding tracks: %+v", err)

		return nil, ErrInternal
	}

	// that concerns current track and guild voice state
	if affectedFirst {
		if guild.ChannelID != in.ChannelID {
			guild.ChannelID = in.ChannelID

			_, err = guild.UpdateQuery(o.store).Column("channel_id").Exec(ctx)
			if err != nil {
				o.logger.Errorf("Error updating guild channel id: %+v", err)

				return nil, ErrInternal
			}
		}

		err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent, ResyncTargetGuild)

		if err != nil {
			o.logger.Errorf("Error sending resync message: %+v", err)

			return nil, ErrInternal
		}
	}

	return &pb.PlayReply{
		Tracks: tracksData,
	}, nil
}

func (o *orcaServer) addTracks(
	ctx context.Context,
	bot *models.RemoteBot,
	guild *models.RemoteGuild,
	url string,
	position int,
) ([]*pb.TrackData, bool, error) {
	tracks, err := models.NewRemoteTracks(ctx, bot.ID, guild.ID, url, o.extractors)
	if err != nil {
		o.logger.Errorf("Error getting remote tracks: %+v", err)

		return nil, false, ErrInternal
	}

	qlen, err := guild.TracksQuery(o.store).Count(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		o.logger.Errorf("Error getting tracks from store: %+v", err)

		return nil, false, ErrInternal
	}

	prevOrdKey, nextOrdKey, affectedFirst := o.chooseOrdKeyRange(ctx, qlen, position)

	models.SetOrdKeys(tracks, prevOrdKey, nextOrdKey)

	_, err = o.store.
		NewInsert().
		Model(&tracks).
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error storing tracks: %+v", err)

		return nil, false, ErrInternal
	}

	res := make([]*pb.TrackData, len(tracks))
	for i, track := range tracks {
		res[i] = track.ToProto()
	}

	return res, affectedFirst, nil
}

func (o *orcaServer) Skip(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	o.logger.Infof("Skipping track in guild %s", in.GuildID)

	var current models.RemoteTrack

	err = guild.CurrentTrackQuery(o.store).Scan(ctx, &current)
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

		return nil, ErrFailedToAuthenticate
	}

	o.logger.Infof("Stopping playback in guild %s", in.GuildID)

	_, err = o.store.
		NewDelete().
		Model((*models.RemoteTrack)(nil)).
		Where("bot_id = ?", bot.ID).
		Where("guild_id = ?", guild.ID).
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

		return nil, ErrFailedToAuthenticate
	}

	o.logger.Infof("Seeking to %.2fs in guild %s", in.Position.AsDuration().Seconds(), in.GuildID)

	_, err = o.store.
		NewUpdate().
		Model((*models.RemoteTrack)(nil)).
		ModelTableExpr("tracks").
		TableExpr("(?) as curr", guild.CurrentTrackQuery(o.store)).
		Set("pos = ?", in.Position.AsDuration()).
		Where("tracks.id = curr.id").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error changing track position: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
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

		return nil, ErrFailedToAuthenticate
	}

	_, err = guild.UpdateQuery(o.store).
		Set("loop = NOT loop").
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

		return nil, ErrFailedToAuthenticate
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
				Where("bot_id = ?", bot.ID).
				Where("guild_id = ?", guild.ID).
				Order("ord_key").
				Limit(1),
		).
		Set("ord_key = RANDOM() * ? + 1 + curr.ord_key", edgeOrdKeyDiff).
		Where("bot_id = ?", bot.ID).
		Where("guild_id = ?", guild.ID).
		Where("tracks.id != curr.id").
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

		return nil, ErrFailedToAuthenticate
	}

	var tracks []*models.RemoteTrack

	countQuery := o.store.
		NewSelect().
		Model(&tracks).
		Where("bot_id = ?", bot.ID).
		Where("guild_id = ?", guild.ID)

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

		return nil, ErrFailedToAuthenticate
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

		return nil, ErrFailedToAuthenticate
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

func (o *orcaServer) Remove(ctx context.Context, in *pb.RemoveRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	// should be at least 0
	position := max(0, int(in.Position))

	_, err = o.store.NewDelete().
		Model((*models.RemoteTrack)(nil)).
		ModelTableExpr("tracks").
		TableExpr("(?) AS target", guild.PositionTrackQuery(o.store, position).Column("id")).
		Where("tracks.id = target.id").
		Exec(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// consider it successfully deleted
			return &emptypb.Empty{}, nil
		}

		o.logger.Errorf("Error deleting track from queue: %+v", err)

		return nil, ErrInternal
	}

	if position == 0 {
		err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
		if err != nil {
			o.logger.Errorf("Error sending resync: %+v", err)

			return nil, ErrInternal
		}
	}

	return &emptypb.Empty{}, nil
}

func main() {
	coreLogger, err := zap.NewDevelopment(zap.IncreaseLevel(zapcore.DebugLevel))
	if err != nil {
		panic(err)
	}

	logger := coreLogger.Sugar()

	ctx := context.Background()

	var vc *vault.Client

	var config *Config

	if os.Getenv("PRODUCTION") == "true" {
		vc, err = gvault.ClientFromEnv(ctx)
		if err != nil {
			logger.Fatalf("Error creating vault client: %+v", err)
		}

		config, err = loadConfig(ctx, vc)
	} else {
		config, err = loadConfigDev(ctx)
	}

	if err != nil {
		logger.Fatalf("Error loading config: %+v", err)
	}

	err = run(ctx, logger, vc, config)
	if err != nil {
		logger.Fatalf("Error running: %+v", err)
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

	migrator := migrate.NewMigrator(db, m, migrate.WithMarkAppliedOnSuccess(true))

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

func run(
	ctx context.Context, logger *zap.SugaredLogger, vc *vault.Client, config *Config,
) error {
	st := store.NewStore(ctx, logger, &store.Config{
		DB:     config.DB,
		Broker: config.Redis,
	}, vc)

	err := doMigrate(ctx, logger, st.DB)
	if err != nil {
		return errorx.Decorate(err, "do migrations")
	}

	port := 8590

	var lc net.ListenConfig

	lis, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", config.Port))
	if err != nil {
		return errorx.Decorate(err, "listen")
	}

	orca, err := newOrcaServer(ctx, logger, st, config)
	if err != nil {
		return errorx.Decorate(err, "create orca server")
	}

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
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err = <-serverErrors:
		logger.Errorf("Error listening for gRPC calls: %+v", err)

		return errorx.Decorate(err, "listen")
	case sig := <-sc:
		logger.Infof("Exiting with signal %v", sig)

		return nil
	}
}

func (o *orcaServer) doResync(ctx context.Context, logger *zap.SugaredLogger, managed *models.Bot, r *ResyncMessage) {
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
					logger.Errorf("Error resyncing guild: %+v", err)
				}
			case ResyncTargetCurrent:
				managed.ResyncGuildTrack(ctx, r.Guild)
			}
		}()
	}

	wg.Wait()
}

func (o *orcaServer) handleResync(ctx context.Context, logger *zap.SugaredLogger, msg *redis.Message) error {
	r := new(ResyncMessage)

	err := json.Unmarshal([]byte(msg.Payload), r)
	if err != nil {
		return errorx.Decorate(err, "unmarshal resync")
	}

	managed := o.getStateByID(r.Bot)
	if managed == nil {
		logger.Debugf("Resync message for non-managed recipient %s, skipping", r.Bot)

		return nil
	}

	o.doResync(ctx, logger, managed, r)

	return nil
}

func (o *orcaServer) sendResync(ctx context.Context, botID, guildID string, targets ...ResyncTarget) error {
	b, err := json.Marshal(ResyncMessage{
		Bot:     botID,
		Guild:   guildID,
		Targets: targets,
	})
	if err != nil {
		return errorx.Decorate(err, "marshal resync message")
	}

	err = o.store.Publish(ctx, fmt.Sprintf("%s:%s", o.store.RedisPrefix, ResyncsChannel), b).Err()
	if err != nil {
		return errorx.Decorate(err, "publish resync message")
	}

	return nil
}

func (o *orcaServer) handleKeyDel(ctx context.Context, logger *zap.SugaredLogger, msg *redis.Message) error {
	val := msg.Payload
	pref := fmt.Sprintf("%s:", o.store.RedisPrefix)

	if !strings.HasPrefix(val, pref) {
		return nil
	}

	val = strings.TrimPrefix(val, pref)

	logger.Infof("Lock for %s expired, trying takeover", val)

	err := o.initFromStore(ctx)
	if err != nil {
		return errorx.Decorate(err, "init after takeover")
	}

	return nil
}

func (o *orcaServer) chooseOrdKeyRange(ctx context.Context, qlen, position int) (float64, float64, bool) {
	// special case - if no tracks in queue - insert all tracks with ordkeys from 0 to 100
	if qlen == 0 {
		return defaultPrevOrdKey, defaultNextOrdKey, true
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
	if err != nil {
		o.logger.Errorf("Error getting ordkeys from store: %+v", err)

		return defaultPrevOrdKey, defaultNextOrdKey, true
	}

	defer func() {
		err = rows.Close()
		if err != nil {
			o.logger.Errorf("Error closing rows: %+v", err)
		}
	}()

	return o.chooseOrdKeyRangeFromRows(position, rows)
}

func (o *orcaServer) chooseOrdKeyRangeFromRows(position int, rows *sql.Rows) (float64, float64, bool) {
	var prevOrdKey, nextOrdKey float64

	rows.Next()

	if err := rows.Err(); err != nil {
		o.logger.Errorf("Error getting next row: %+v", err)

		return defaultPrevOrdKey, defaultNextOrdKey, true
	}

	// special case - there is no track preceding the position, prevOrdKey should be nextOrdKey - 100
	if position == 0 {
		err := rows.Scan(&nextOrdKey)
		if err != nil {
			o.logger.Errorf("Error scanning track 0 from store: %+v", err)

			return defaultPrevOrdKey, defaultNextOrdKey, true
		}

		rows.Next() // skip second scanned row to not get error on close

		return nextOrdKey - edgeOrdKeyDiff, nextOrdKey, true
	}

	err := rows.Scan(&prevOrdKey)
	if err != nil {
		o.logger.Errorf("Error scanning prev track from store: %+v", err)

		return defaultPrevOrdKey, defaultNextOrdKey, true
	}

	// special case - there is no track succeeding the position, nextOrdKey should be prevOrdKey + 100
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			o.logger.Errorf("Error getting next row: %+v", err)

			return defaultPrevOrdKey, defaultNextOrdKey, true
		}

		nextOrdKey = prevOrdKey + edgeOrdKeyDiff
	} else {
		err = rows.Scan(&nextOrdKey)
		if err != nil {
			o.logger.Errorf("Error scanning next track from store: %+v", err)

			return defaultPrevOrdKey, defaultNextOrdKey, true
		}
	}

	return prevOrdKey, nextOrdKey, false
}
