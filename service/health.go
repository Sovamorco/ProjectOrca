package orca

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Health(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
