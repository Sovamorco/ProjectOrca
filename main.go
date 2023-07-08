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
)

func newOrcaServer(logger *zap.SugaredLogger, session *discordgo.Session) *orcaServer {
	return &orcaServer{
		state: newState(logger, session),
	}
}

type orcaServer struct {
	pb.UnimplementedOrcaServer
	state *State
}

func (o *orcaServer) Play(ctx context.Context, in *pb.PlayRequest) (*pb.PlayReply, error) {
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
