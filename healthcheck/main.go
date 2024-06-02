package main

import (
	"context"
	"os"

	"github.com/rs/zerolog/log"

	orca "ProjectOrca/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.Dial(os.Getenv("ORCA_HEALTH_ADDRESS"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	defer func() {
		err = conn.Close()
		if err != nil {
			log.Error().Err(err).Msg("Error closing connection")
		}
	}()

	cc := orca.NewOrcaClient(conn)

	_, err = cc.Health(context.Background(), &orca.HealthRequest{})
	if err != nil {
		panic(err)
	}
}
