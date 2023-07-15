package main

import (
	"ProjectOrca/migrations"
	"ProjectOrca/models"
	pb "ProjectOrca/proto"
	"ProjectOrca/store"
	"ProjectOrca/utils"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
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
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	MigrationsTimeout   = time.Second * 300
	DeathrattlesChannel = "deathrattles"
)

type DeathrattleMessage struct {
	Deceased string
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
	pb.UnimplementedOrcaServer

	id      string
	address string
	states  map[string]*models.BotState
	logger  *zap.SugaredLogger
	store   *store.Store
}

func (o *orcaServer) gracefulShutdown() {
	for _, state := range o.states {
		state.GracefulShutdown()
	}
	_, err := o.store.
		NewUpdate().
		Model((*models.BotState)(nil)).
		Where("locker = ?", o.id).
		Set("locker = NULL").
		Set("locker_address = NULL").
		Exec(context.TODO())
	if err != nil {
		o.logger.Errorf("Error unlocking bots: %+v", err)
	}

	o.store.Unsubscribe()
	b, err := json.Marshal(DeathrattleMessage{
		Deceased: o.id,
	})
	if err != nil {
		o.logger.Errorf("Error marshalling deathrattle: %+v", err)
	} else {
		err = o.store.Publish(context.TODO(), DeathrattlesChannel, b).Err()
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
		Where("locker IS NULL").
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
		err = state.Restore(o.logger, o.store)
		if err != nil {
			return errorx.Decorate(err, "restore state")
		}
		o.states[state.ID] = state
	}
	return nil
}

func (o *orcaServer) authLogInterceptor(ctx context.Context, req any, serverInfo *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	o.logger.Debugf("Request to %s", serverInfo.FullMethod)
	ctx = context.WithValue(ctx, "full_method", serverInfo.FullMethod)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		o.logger.Debug("missing metadata")
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	tokenS := md.Get("token")
	if len(tokenS) != 1 {
		o.logger.Debug("missing token")
		return nil, status.Error(codes.Unauthenticated, "missing token")
	}
	token := tokenS[0]
	botIDS := md.Get("botID")
	if len(botIDS) != 1 {
		o.logger.Debug("missing bot id")
		return nil, status.Error(codes.Unauthenticated, "missing bot id")
	}
	botID := botIDS[0]
	ctx = context.WithValue(ctx, "botID", botID)
	if _, ok := req.(*pb.RegisterRequest); token == "" && ok {
		res, err := handler(ctx, req)
		if err != nil {
			o.logger.Debugf("Returning error: %+v", err)
		}
		return res, err
	}

	state := new(models.BotState)
	state.ID = botID
	err = o.store.NewSelect().Model(state).WherePK().Scan(ctx)
	if err != nil {
		if errors.As(err, &sql.ErrNoRows) {
			o.logger.Debug("not registered")
			return nil, status.Error(codes.Unauthenticated, "not registered")
		}
		o.logger.Errorf("Error getting state from store: %+v", err)
		return nil, status.Error(codes.Internal, "internal error")
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
	res, err := handler(context.WithValue(ctx, "state", state), req)
	if err != nil {
		o.logger.Debugf("Returning error: %+v", err)
	}
	return res, err
}

func (o *orcaServer) Register(ctx context.Context, in *pb.RegisterRequest) (*pb.RegisterReply, error) {
	botID := ctx.Value("botID").(string)
	o.logger.Infof("Received registration request for bot %s", botID)
	newToken, err := utils.GenerateSecureToken()
	if err != nil {
		return nil, err
	}
	state := new(models.BotState)
	state.ID = botID
	err = o.store.NewSelect().Model(state).WherePK().Scan(ctx)
	if err != nil {
		if !errors.As(err, &sql.ErrNoRows) {
			o.logger.Errorf("Error getting state from store: %+v", err)
			return nil, status.Error(codes.Internal, "internal error")
		}
		state, err = models.NewState(o.logger, in.Token, o.store, newToken, o.id, o.address)
		if err != nil {
			return nil, errorx.Decorate(err, "create state")
		}
		o.states[botID] = state
		return &pb.RegisterReply{
			Token: newToken,
		}, nil
	}
	if state.Locker != o.id {
		repl, err := o.forwardRequest(ctx, ctx.Value("full_method").(string), state.LockerAddress, in)
		if repl == nil {
			return nil, err
		}
		return repl.(*pb.RegisterReply), err
	}
	state = o.states[state.ID]
	o.logger.Infof("Bot %s already registered, reregestering", botID)
	if in.Token != state.Token {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	state.StateToken = newToken
	_, err = o.store.NewUpdate().Model(state).WherePK().Exec(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "store new token")
	}
	return &pb.RegisterReply{
		Token: newToken,
	}, nil
}

func (o *orcaServer) Play(ctx context.Context, in *pb.PlayRequest) (*pb.PlayReply, error) {
	state := ctx.Value("state").(*models.BotState)
	o.logger.Infof("Playing \"%s\" in channel %s guild %s", in.Url, in.ChannelID, in.GuildID)
	gs, err := state.GetOrCreateGuildState(in.GuildID)
	if err != nil {
		return nil, errorx.Decorate(err, "get guild state")
	}
	trackData, err := gs.PlaySound(in.ChannelID, in.Url)
	if err != nil {
		return nil, err
	}
	return &pb.PlayReply{
		Track: trackData,
	}, nil
}

func (o *orcaServer) Skip(ctx context.Context, in *pb.SkipRequest) (*pb.SkipReply, error) {
	state := ctx.Value("state").(*models.BotState)
	o.logger.Infof("Skipping track in guild %s", in.GuildID)
	gs, err := state.GetOrCreateGuildState(in.GuildID)
	if err != nil {
		return nil, errorx.Decorate(err, "get guild state")
	}
	err = gs.Skip()
	if err != nil {
		return nil, err
	}
	return &pb.SkipReply{}, nil
}

func (o *orcaServer) Stop(ctx context.Context, in *pb.StopRequest) (*pb.StopReply, error) {
	state := ctx.Value("state").(*models.BotState)
	o.logger.Infof("Stopping playback in guild %s", in.GuildID)
	gs, err := state.GetOrCreateGuildState(in.GuildID)
	if err != nil {
		return nil, errorx.Decorate(err, "get guild state")
	}
	err = gs.Stop()
	if err != nil {
		return nil, err
	}
	return &pb.StopReply{}, nil
}

func (o *orcaServer) Seek(ctx context.Context, in *pb.SeekRequest) (*pb.SeekReply, error) {
	state := ctx.Value("state").(*models.BotState)
	o.logger.Infof("Seeking to %.2fs in guild %s", in.Position, in.GuildID)
	gs, err := state.GetOrCreateGuildState(in.GuildID)
	if err != nil {
		return nil, errorx.Decorate(err, "get guild state")
	}
	tc := time.Duration(in.Position * float32(time.Second))
	err = gs.Seek(tc)
	if err != nil {
		return nil, err
	}
	return &pb.SeekReply{}, nil
}

func (o *orcaServer) Volume(ctx context.Context, in *pb.VolumeRequest) (*pb.VolumeReply, error) {
	state := ctx.Value("state").(*models.BotState)
	o.logger.Infof("Setting guild %s volume to %.2f", in.GuildID, in.Volume)
	gs, err := state.GetOrCreateGuildState(in.GuildID)
	if err != nil {
		return nil, errorx.Decorate(err, "get guild state")
	}
	gs.TargetVolume = in.Volume / 100
	_, err = o.store.NewUpdate().Model(gs).WherePK().Exec(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "store target volume")
	}
	return &pb.VolumeReply{}, nil
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
	migrator := migrate.NewMigrator(db, migrations.Migrations)
	err := migrator.Init(migrateContext)
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

func run(ctx context.Context, logger *zap.SugaredLogger) error {
	st, err := store.NewStore(logger, &store.Config{
		DB: store.DBConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "local",
			DB:       "orca",
		},
		Broker: store.RedisConfig{
			Host:  "127.0.0.1",
			Port:  6379,
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

	port := 8591
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
	go orca.handleDeathrattle(ps.Channel())

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(orca.authLogInterceptor),
	)
	pb.RegisterOrcaServer(grpcServer, orca)

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- grpcServer.Serve(lis)
	}()
	defer grpcServer.GracefulStop()
	defer orca.gracefulShutdown()

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
	case *pb.SkipRequest:
		res = new(pb.SkipReply)
	case *pb.StopRequest:
		res = new(pb.StopReply)
	case *pb.VolumeRequest:
		res = new(pb.VolumeReply)
	}
	err = conn.Invoke(ctx, call, req, res)
	return res, err
}

func (o *orcaServer) handleDeathrattle(ch <-chan *redis.Message) {
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
			err = o.initFromStore(context.TODO())
			if err != nil {
				logger.Errorf("Error reinitializing after deathrattle: %+v", err)
			}
		}()
	}

	wg.Wait()
}

// getLocalAddress gets the local address from specific network with port
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
	return "", errors.New("could not get address")
}
