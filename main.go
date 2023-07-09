package main

import (
	pb "ProjectOrca/proto"
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func newOrcaServer(logger *zap.SugaredLogger, session *discordgo.Session) *orcaServer {
	stateLogger := logger.With("label", "state")
	serverLogger := logger.With("label", "server")
	return &orcaServer{
		state:  newState(stateLogger, session),
		logger: serverLogger,
	}
}

type orcaServer struct {
	pb.UnimplementedOrcaServer
	state  *State
	logger *zap.SugaredLogger
}

func (o *orcaServer) Play(_ context.Context, in *pb.PlayRequest) (*pb.PlayReply, error) {
	o.logger.Infof("Playing \"%s\" in channel %s guild %s", in.Url, in.ChannelID, in.GuildID)
	gs, ok := o.state.Guilds[in.GuildID]
	if !ok {
		gs = o.state.newGuildState(in.GuildID)
	}
	trackData, err := gs.playSound(in.ChannelID, in.Url)
	if err != nil {
		return nil, err
	}
	return &pb.PlayReply{
		Track: trackData,
	}, nil
}

func (o *orcaServer) Stop(_ context.Context, in *pb.StopRequest) (*pb.StopReply, error) {
	o.logger.Infof("Stopping playback in guild %s", in.GuildID)
	gs, ok := o.state.Guilds[in.GuildID]
	if !ok {
		gs = o.state.newGuildState(in.GuildID)
	}
	err := gs.stop()
	if err != nil {
		return nil, err
	}
	return &pb.StopReply{}, nil
}

func (o *orcaServer) Seek(_ context.Context, in *pb.SeekRequest) (*pb.SeekReply, error) {
	o.logger.Infof("Seeking to %.2fs in guild %s", in.Position, in.GuildID)
	gs, ok := o.state.Guilds[in.GuildID]
	if !ok {
		gs = o.state.newGuildState(in.GuildID)
	}
	tc := time.Duration(in.Position * float32(time.Second))
	err := gs.seek(tc)
	if err != nil {
		return nil, err
	}
	return &pb.SeekReply{}, nil
}

func (o *orcaServer) Volume(_ context.Context, in *pb.VolumeRequest) (*pb.VolumeReply, error) {
	o.logger.Infof("Setting guild %s volume to %.2f", in.GuildID, in.Volume)
	gs, ok := o.state.Guilds[in.GuildID]
	if !ok {
		gs = o.state.newGuildState(in.GuildID)
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
	// temporary
	tf, err := os.Open(".token")
	if err != nil {
		return err
	}
	token, err := io.ReadAll(tf)
	if err != nil {
		return err
	}

	rcb, err := discordgo.New("Bot " + string(token))
	if err != nil {
		return err
	}

	state := newState(logger, rcb)
	logger.Info(state)

	rcb.Identify.Intents = discordgo.IntentGuildVoiceStates
	err = rcb.Open()
	if err != nil {
		return err
	}

	defer func() {
		err := rcb.Close()
		if err != nil {
			logger.Error("Error closing bot: ", err)
		}
	}()

	logger.Infof("Started the bot")

	port := 8590
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	orca := newOrcaServer(logger, rcb)
	pb.RegisterOrcaServer(grpcServer, orca)

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- grpcServer.Serve(lis)
	}()
	defer func() {
		grpcServer.GracefulStop()
	}()

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
