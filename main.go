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
	"reflect"
	"sync"
	"syscall"
	"time"

	"ProjectOrca/migrations"
	"ProjectOrca/models"
	"ProjectOrca/store"
	"ProjectOrca/utils"

	pb "ProjectOrca/proto"

	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type key int

const (
	MigrationsTimeout   = time.Second * 300
	DeathrattlesChannel = "deathrattles"

	KeyState key = iota
	KeyBotID
	KeyFullMethod
)

var (
	ErrInternal  = status.Error(codes.Internal, "internal error")
	ErrNoAddress = errors.New("could not get address")
)

type DeathrattleMessage struct {
	Deceased string `json:"deceased"`
}

func newOrcaServer(logger *zap.SugaredLogger, st *store.Store, address string) *orcaServer {
	serverLogger := logger.Named("server")

	return &orcaServer{
		id:      uuid.New().String(),
		address: address,
		states:  make(map[string]*models.BotState),
		logger:  serverLogger,
		store:   st,
	}
}

type orcaServer struct {
	pb.UnimplementedOrcaServer `exhaustruct:"optional"`

	id      string
	address string
	states  map[string]*models.BotState
	logger  *zap.SugaredLogger
	store   *store.Store
}

func (o *orcaServer) gracefulShutdown(ctx context.Context) {
	for _, state := range o.states {
		state.GracefulShutdown()
	}

	_, err := o.store.
		NewUpdate().
		Model((*models.BotState)(nil)).
		Where("locker = ?", o.id).
		Set("locker = NULL").
		Set("locker_address = NULL").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error unlocking bots: %+v", err)
	}

	o.store.Unsubscribe(ctx)

	b, err := json.Marshal(DeathrattleMessage{
		Deceased: o.id,
	})
	if err != nil {
		o.logger.Errorf("Error marshalling deathrattle: %+v", err)
	} else {
		err = o.store.Publish(ctx, DeathrattlesChannel, b).Err()
		if err != nil {
			o.logger.Errorf("Error publishing deathrattle: %+v", err)
		}
	}

	o.store.GracefulShutdown()
}

func (o *orcaServer) initFromStore(ctx context.Context) error {
	_, err := o.store.
		NewUpdate().
		Model((*models.BotState)(nil)).
		Where("locker IS NULL OR locker_address = ?", o.address).
		Set("locker = ?", o.id).
		Set("locker_address = ?", o.address).
		Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "lock states")
	}

	states := make([]*models.BotState, 0)

	err = o.store.
		NewSelect().
		Model(&states).
		Where("locker = ?", o.id).
		Relation("Guilds").
		Relation("Guilds.Queue").
		Relation("Guilds.Queue.Tracks", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("ord_key ASC")
		}).
		Scan(ctx)
	if err != nil {
		return errorx.Decorate(err, "list bot states")
	}

	for _, state := range states {
		if _, ok := o.states[state.ID]; ok {
			// state already controlled, no need to init
			continue
		}

		err = state.Restore(ctx, o.logger, o.store)
		if err != nil {
			return errorx.Decorate(err, "restore state")
		}

		o.states[state.ID] = state
	}

	return nil
}

func (o *orcaServer) parseIncomingContext(ctx context.Context) (string, string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		o.logger.Debug("missing metadata")

		return "", "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	tokenS := md.Get("token")
	if len(tokenS) != 1 {
		o.logger.Debug("missing token")

		return "", "", status.Error(codes.Unauthenticated, "missing token")
	}

	token := tokenS[0]

	botIDS := md.Get("botID")
	if len(botIDS) != 1 {
		o.logger.Debug("missing bot id")

		return "", "", status.Error(codes.Unauthenticated, "missing bot id")
	}

	botID := botIDS[0]

	return botID, token, nil
}

func (o *orcaServer) getStateAuth(ctx context.Context, botID string) (*models.BotState, error) {
	state := new(models.BotState)
	state.ID = botID

	err := o.store.NewSelect().Model(state).WherePK().Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			o.logger.Debug("not registered")

			return nil, status.Error(codes.Unauthenticated, "not registered")
		}

		o.logger.Errorf("Error getting state from store: %+v", err)

		return nil, ErrInternal
	}

	return state, nil
}

func (o *orcaServer) authLogInterceptor(
	ctx context.Context,
	req any,
	serverInfo *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	o.logger.Debugf("Request to %s", serverInfo.FullMethod)

	ctx = context.WithValue(ctx, KeyFullMethod, serverInfo.FullMethod)

	botID, token, err := o.parseIncomingContext(ctx)
	if err != nil {
		return nil, err
	}

	ctx = context.WithValue(ctx, KeyBotID, botID)

	if _, ok := req.(*pb.RegisterRequest); token == "" && ok {
		res, err := handler(ctx, req)
		if err != nil {
			o.logger.Debugf("Returning error: %+v", err)
		}

		return res, err
	}

	state, err := o.getStateAuth(ctx, botID)
	if err != nil {
		return nil, err
	}

	if state.StateToken != token {
		o.logger.Debug("invalid token")

		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	if state.Locker != o.id {
		res, err := o.forwardRequest(ctx, serverInfo.FullMethod, state.LockerAddress, req)
		if err != nil {
			o.logger.Debugf("Returning error: %+v", err)
		}

		return res, err
	}

	state = o.states[botID]

	res, err := handler(context.WithValue(ctx, KeyState, state), req)
	if err != nil {
		o.logger.Debugf("Returning error: %+v", err)
	}

	return res, err
}

func (o *orcaServer) registerNew(
	ctx context.Context,
	in *pb.RegisterRequest,
	botID, token string,
) (*pb.RegisterReply, error) {
	state, err := models.NewState(ctx, o.logger, o.store, in.Token, token, o.id, o.address)
	if err != nil {
		o.logger.Errorf("Error creating state: %+v", err)

		return nil, ErrInternal
	}

	o.states[botID] = state

	return &pb.RegisterReply{
		Token: token,
	}, nil
}

func (o *orcaServer) reregister(
	ctx context.Context,
	in *pb.RegisterRequest,
	existing *models.BotState,
	token string,
) (*pb.RegisterReply, error) {
	if existing.Locker != o.id {
		fullMethod, err := o.getString(ctx, KeyFullMethod)
		if err != nil {
			return nil, err
		}

		replI, err := o.forwardRequest(ctx, fullMethod, existing.LockerAddress, in)
		if replI == nil {
			return nil, err
		}

		repl, ok := replI.(*pb.RegisterReply)
		if !ok {
			o.logger.Errorf("reply from forwarded request is %s, not *pb.RegisterReply", reflect.TypeOf(replI))

			return nil, ErrInternal
		}

		return repl, err
	}

	existing = o.states[existing.ID]

	o.logger.Infof("Bot %s already registered, reregestering", existing.ID)

	if in.Token != existing.Token {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	existing.StateToken = token

	_, err := o.store.NewUpdate().Model(existing).WherePK().Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error getting guild state: %+v", err)

		return nil, ErrInternal
	}

	return &pb.RegisterReply{
		Token: token,
	}, nil
}

func (o *orcaServer) Register(ctx context.Context, in *pb.RegisterRequest) (*pb.RegisterReply, error) {
	botID, err := o.getString(ctx, KeyBotID)
	if err != nil {
		return nil, err
	}

	o.logger.Infof("Received registration request for bot %s", botID)

	newToken, err := utils.GenerateSecureToken()
	if err != nil {
		o.logger.Errorf("Error generating token: %+v", err)

		return nil, ErrInternal
	}

	state := new(models.BotState)
	state.ID = botID

	err = o.store.NewSelect().Model(state).WherePK().Scan(ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			o.logger.Errorf("Error getting state from store: %+v", err)

			return nil, ErrInternal
		}

		return o.registerNew(ctx, in, botID, newToken)
	}

	return o.reregister(ctx, in, state, newToken)
}

func (o *orcaServer) Play(ctx context.Context, in *pb.PlayRequest) (*pb.PlayReply, error) {
	state, err := o.getState(ctx)
	if err != nil {
		return nil, err
	}

	o.logger.Infof("Playing \"%s\" in channel %s guild %s", in.Url, in.ChannelID, in.GuildID)

	gs, err := state.GetOrCreateGuildState(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error getting guild state: %+v", err)

		return nil, ErrInternal
	}

	trackData, err := gs.PlayTrack(ctx, in.ChannelID, in.Url, int(in.Position))
	if err != nil {
		o.logger.Errorf("Error playing track: %+v", err)

		return nil, ErrInternal
	}

	return &pb.PlayReply{
		Track: trackData,
	}, nil
}

func (o *orcaServer) Skip(ctx context.Context, in *pb.GuildOnlyRequest) (*pb.GuildOnlyReply, error) {
	state, err := o.getState(ctx)
	if err != nil {
		return nil, err
	}

	o.logger.Infof("Skipping track in guild %s", in.GuildID)

	gs, err := state.GetOrCreateGuildState(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error getting guild state: %+v", err)

		return nil, ErrInternal
	}

	err = gs.Skip()
	if err != nil {
		o.logger.Errorf("Error skipping: %+v", err)

		return nil, ErrInternal
	}

	return &pb.GuildOnlyReply{}, nil
}

func (o *orcaServer) Stop(ctx context.Context, in *pb.GuildOnlyRequest) (*pb.GuildOnlyReply, error) {
	state, err := o.getState(ctx)
	if err != nil {
		return nil, err
	}

	o.logger.Infof("Stopping playback in guild %s", in.GuildID)

	gs, err := state.GetOrCreateGuildState(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error getting guild state: %+v", err)

		return nil, ErrInternal
	}

	err = gs.Stop(ctx)
	if err != nil {
		o.logger.Errorf("Error stopping: %+v", err)

		return nil, ErrInternal
	}

	return &pb.GuildOnlyReply{}, nil
}

func (o *orcaServer) Seek(ctx context.Context, in *pb.SeekRequest) (*pb.SeekReply, error) {
	state, err := o.getState(ctx)
	if err != nil {
		return nil, err
	}

	o.logger.Infof("Seeking to %.2fs in guild %s", in.Position.AsDuration().Seconds(), in.GuildID)

	gs, err := state.GetOrCreateGuildState(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error getting guild state: %+v", err)

		return nil, ErrInternal
	}

	tc := in.Position.AsDuration()

	err = gs.Seek(tc)
	if err != nil {
		if errors.Is(err, models.ErrSeekLive) {
			return nil, status.Error(codes.InvalidArgument, "cannot seek live")
		}

		o.logger.Errorf("Error seeking: %+v", err)

		return nil, ErrInternal
	}

	return &pb.SeekReply{}, nil
}

func (o *orcaServer) GetTracks(ctx context.Context, in *pb.GetTracksRequest) (*pb.GetTracksReply, error) {
	state, err := o.getState(ctx)
	if err != nil {
		return nil, err
	}

	gs, err := state.GetOrCreateGuildState(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error getting guild state: %+v", err)

		return nil, ErrInternal
	}

	if gs.Queue == nil {
		return &pb.GetTracksReply{
			Tracks:      nil,
			TotalTracks: 0,
			Looping:     false,
		}, nil
	}

	gs.Queue.RLock()

	qlen := len(gs.Queue.Tracks)
	// start has to be at least 0 and at most qlen
	start := min(max(int(in.Start), 0), qlen)
	// end has to be at least start and at most qlen
	end := min(max(int(in.End), start), qlen)

	tracks := gs.Queue.Tracks[start:end]
	looping := gs.Queue.Loop

	gs.Queue.RUnlock()

	res := make([]*pb.TrackData, 0, len(tracks))

	for _, track := range tracks {
		res = append(res, track.ToProto())
	}

	return &pb.GetTracksReply{
		Tracks:      res,
		TotalTracks: int64(qlen),
		Looping:     looping,
	}, nil
}

func (o *orcaServer) Pause(ctx context.Context, in *pb.GuildOnlyRequest) (*pb.GuildOnlyReply, error) {
	state, err := o.getState(ctx)
	if err != nil {
		return nil, err
	}

	gs, err := state.GetOrCreateGuildState(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error getting guild state: %+v", err)

		return nil, ErrInternal
	}

	err = gs.Pause(ctx)
	if err != nil {
		o.logger.Errorf("Error pausing: %+v", err)

		return nil, ErrInternal
	}

	return &pb.GuildOnlyReply{}, nil
}

func (o *orcaServer) Resume(ctx context.Context, in *pb.GuildOnlyRequest) (*pb.GuildOnlyReply, error) {
	state, err := o.getState(ctx)
	if err != nil {
		return nil, err
	}

	gs, err := state.GetOrCreateGuildState(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error getting guild state: %+v", err)

		return nil, ErrInternal
	}

	err = gs.Resume(ctx)
	if err != nil {
		o.logger.Errorf("Error resuming: %+v", err)

		return nil, ErrInternal
	}

	return &pb.GuildOnlyReply{}, nil
}

func (o *orcaServer) Loop(ctx context.Context, in *pb.GuildOnlyRequest) (*pb.GuildOnlyReply, error) {
	state, err := o.getState(ctx)
	if err != nil {
		return nil, err
	}

	gs, err := state.GetOrCreateGuildState(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error getting guild state: %+v", err)

		return nil, ErrInternal
	}

	if gs.Queue == nil {
		return nil, status.Error(codes.InvalidArgument, "nothing playing")
	}

	gs.Queue.Loop = !gs.Queue.Loop

	_, err = o.store.NewUpdate().Model(gs.Queue).WherePK().Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error storing queue: %+v", err)

		return nil, ErrInternal
	}

	return &pb.GuildOnlyReply{}, nil
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

	localAddr, err := getLocalAddress("127.0.0.0", port)
	if err != nil {
		return errorx.Decorate(err, "get local address")
	}

	orca := newOrcaServer(logger, st, localAddr)

	err = orca.initFromStore(ctx)
	if err != nil {
		return errorx.Decorate(err, "init from store")
	}

	ps := orca.store.Subscribe(ctx, DeathrattlesChannel)
	go orca.handleDeathrattle(ctx, ps.Channel())

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(orca.authLogInterceptor),
	)
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

func (o *orcaServer) forwardRequest(ctx context.Context, call, recip string, req any) (any, error) {
	o.logger.Debugf("Forwarding request %s to %s", call, recip)

	conn, err := grpc.Dial(
		recip,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, errorx.Decorate(err, "connect to recipient")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	var res proto.Message

	switch req.(type) {
	case *pb.PlayRequest:
		res = new(pb.PlayReply)
	case *pb.RegisterRequest:
		res = new(pb.RegisterReply)
	case *pb.SeekRequest:
		res = new(pb.SeekReply)
	case *pb.GuildOnlyReply:
		res = new(pb.GuildOnlyReply)
	case *pb.GetTracksRequest:
		res = new(pb.GetTracksReply)
	}

	err = conn.Invoke(ctx, call, req, res)

	return res, err //nolint:wrapcheck
}

func (o *orcaServer) handleDeathrattle(ctx context.Context, ch <-chan *redis.Message) {
	logger := o.logger.Named("sub_deathrattles")

	var wg sync.WaitGroup

	for msg := range ch {
		msg := msg

		wg.Add(1)

		go func() {
			defer wg.Done()

			logger.Infof("Received deathrattle: %s", msg.Payload)

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

func (o *orcaServer) getState(ctx context.Context) (*models.BotState, error) {
	stateI := ctx.Value(KeyState)

	state, ok := stateI.(*models.BotState)
	if !ok {
		o.logger.Errorf("state is type %s, not *models.BotState", reflect.TypeOf(stateI))

		return nil, ErrInternal
	}

	return state, nil
}

func (o *orcaServer) getString(ctx context.Context, k key) (string, error) {
	vi := ctx.Value(k)

	v, ok := vi.(string)
	if !ok {
		o.logger.Errorf("value is type %s, not string", reflect.TypeOf(vi))

		return "", ErrInternal
	}

	return v, nil
}

// getLocalAddress gets the local address from specific network with port.
func getLocalAddress(network string, port int) (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", errorx.Decorate(err, "get interface addresses")
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && ipNet.IP.Mask(ipNet.Mask).String() == network {
			return fmt.Sprintf("%s:%d", ipNet.IP.String(), port), nil
		}
	}

	return "", ErrNoAddress
}
