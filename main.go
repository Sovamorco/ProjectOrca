package main

import (
	pb "ProjectOrca/proto"
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
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

func newOrcaServer(logger *zap.SugaredLogger) *orcaServer {
	serverLogger := logger.With("label", "server")
	return &orcaServer{
		states: make(map[string]*BotState),
		logger: serverLogger,
	}
}

type orcaServer struct {
	pb.UnimplementedOrcaServer
	states map[string]*BotState
	logger *zap.SugaredLogger
}

func (o *orcaServer) gracefulShutdown() {
	for _, state := range o.states {
		state.gracefulShutdown()
	}
}

func (o *orcaServer) authInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	tokenS := md.Get("token")
	if len(tokenS) != 1 {
		return nil, status.Error(codes.Unauthenticated, "missing token")
	}
	token := tokenS[0]
	botIDS := md.Get("botID")
	if len(botIDS) != 1 {
		return nil, status.Error(codes.Unauthenticated, "missing bot id")
	}
	botID := botIDS[0]
	ctx = context.WithValue(ctx, "botID", botID)
	if _, ok := req.(*pb.RegisterRequest); token == "" && ok {
		return handler(ctx, req)
	}
	state, ok := o.states[botID]
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not registered")
	}
	if state.Token != token {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	return handler(context.WithValue(ctx, "state", state), req)
}

func (o *orcaServer) Register(ctx context.Context, in *pb.RegisterRequest) (*pb.RegisterReply, error) {
	botID := ctx.Value("botID").(string)
	o.logger.Infof("Received registration request for bot %s", botID)
	newToken, err := generateSecureToken()
	if err != nil {
		return nil, err
	}
	state, ok := o.states[botID]
	if ok {
		o.logger.Infof("Bot %s already registered, reregestering", botID)
		if in.Token != state.Session.Identify.Token {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}
		state.Token = newToken
		return &pb.RegisterReply{
			Token: newToken,
		}, nil
	}
	session, err := discordgo.New(in.Token)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = discordgo.IntentGuildVoiceStates
	err = session.Open()
	if err != nil {
		return nil, err
	}
	stateLogger := o.logger.With("label", "state", "botID", botID)
	stateLogger.Infof("Started the bot")
	o.states[botID] = newState(stateLogger, session, newToken)
	return &pb.RegisterReply{
		Token: newToken,
	}, nil
}

func (o *orcaServer) Play(ctx context.Context, in *pb.PlayRequest) (*pb.PlayReply, error) {
	state := ctx.Value("state").(*BotState)
	o.logger.Infof("Playing \"%s\" in channel %s guild %s", in.Url, in.ChannelID, in.GuildID)
	gs, ok := state.Guilds[in.GuildID]
	if !ok {
		gs = state.newGuildState(in.GuildID)
	}
	trackData, err := gs.playSound(in.ChannelID, in.Url)
	if err != nil {
		return nil, err
	}
	return &pb.PlayReply{
		Track: trackData,
	}, nil
}

func (o *orcaServer) Skip(ctx context.Context, in *pb.SkipRequest) (*pb.SkipReply, error) {
	state := ctx.Value("state").(*BotState)
	o.logger.Infof("Skipping track in guild %s", in.GuildID)
	gs, ok := state.Guilds[in.GuildID]
	if !ok {
		gs = state.newGuildState(in.GuildID)
	}
	err := gs.skip()
	if err != nil {
		return nil, err
	}
	return &pb.SkipReply{}, nil
}

func (o *orcaServer) Stop(ctx context.Context, in *pb.StopRequest) (*pb.StopReply, error) {
	state := ctx.Value("state").(*BotState)
	o.logger.Infof("Stopping playback in guild %s", in.GuildID)
	gs, ok := state.Guilds[in.GuildID]
	if !ok {
		gs = state.newGuildState(in.GuildID)
	}
	err := gs.stop()
	if err != nil {
		return nil, err
	}
	return &pb.StopReply{}, nil
}

func (o *orcaServer) Seek(ctx context.Context, in *pb.SeekRequest) (*pb.SeekReply, error) {
	state := ctx.Value("state").(*BotState)
	o.logger.Infof("Seeking to %.2fs in guild %s", in.Position, in.GuildID)
	gs, ok := state.Guilds[in.GuildID]
	if !ok {
		gs = state.newGuildState(in.GuildID)
	}
	tc := time.Duration(in.Position * float32(time.Second))
	err := gs.seek(tc)
	if err != nil {
		return nil, err
	}
	return &pb.SeekReply{}, nil
}

func (o *orcaServer) Volume(ctx context.Context, in *pb.VolumeRequest) (*pb.VolumeReply, error) {
	state := ctx.Value("state").(*BotState)
	o.logger.Infof("Setting guild %s volume to %.2f", in.GuildID, in.Volume)
	gs, ok := state.Guilds[in.GuildID]
	if !ok {
		gs = state.newGuildState(in.GuildID)
	}
	gs.TargetVolume = in.Volume / 100
	return &pb.VolumeReply{}, nil
}

func main() {
	coreLogger, err := zap.NewDevelopment(zap.IncreaseLevel(zapcore.DebugLevel))
	if err != nil {
		panic(err)
	}
	logger := coreLogger.Sugar()
	err = run(logger)
	if err != nil {
		logger.Fatal(err)
	}
}

func run(logger *zap.SugaredLogger) error {
	port := 8590
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	orca := newOrcaServer(logger)
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(orca.authInterceptor),
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
		logger.Error("Error listening for gRPC calls: ", err)
		return err
	case sig := <-sc:
		logger.Infof("Exiting with signal %v", sig)
		return nil
	}
}
