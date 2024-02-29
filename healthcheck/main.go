package main

import (
	"context"
	"log/slog"
	"os"

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
			slog.Error("Error closing connection: ", err)
		}
	}()

	cc := orca.NewOrcaClient(conn)

	_, err = cc.Health(context.Background(), &orca.HealthRequest{})
	if err != nil {
		panic(err)
	}
}
