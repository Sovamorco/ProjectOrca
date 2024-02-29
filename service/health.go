package orca

import (
	"context"

	pb "ProjectOrca/proto"

	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Health(_ context.Context, _ *pb.HealthRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
