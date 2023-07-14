package main

import (
	"ProjectOrca/migrations"
	"ProjectOrca/models"
	pb "ProjectOrca/proto"
	"ProjectOrca/store"
	"ProjectOrca/utils"
	"context"
	"fmt"
	"github.com/joomcode/errorx"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const MigrationsTimeout = time.Second * 300

func newOrcaServer(logger *zap.SugaredLogger, st *store.Store) *orcaServer {
	serverLogger := logger.Named("server")
	return &orcaServer{
		states: make(map[string]*models.BotState),
		logger: serverLogger,
		store:  st,
	}
}

type orcaServer struct {
	pb.UnimplementedOrcaServer
	states map[string]*models.BotState
	logger *zap.SugaredLogger
	store  *store.Store
}

func (o *orcaServer) gracefulShutdown() {
	for _, state := range o.states {
		state.GracefulShutdown()
	}
	o.store.GracefulShutdown()
}

func (o *orcaServer) initFromStore(ctx context.Context) error {
	states := make([]*models.BotState, 0)
	err := o.store.
		NewSelect().
		Model(&states).
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
	state, ok := o.states[botID]
	if !ok {
		o.logger.Debug("not registered")
		return nil, status.Error(codes.Unauthenticated, "not registered")
	}
	if state.StateToken != token {
		o.logger.Debug("invalid token")
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
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
	state, ok := o.states[botID]
	if ok {
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
	state, err = models.NewState(o.logger, in.Token, o.store, newToken)
	if err != nil {
		return nil, errorx.Decorate(err, "create state")
	}
	o.states[botID] = state
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
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "local",
		DB:       "orca",
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
