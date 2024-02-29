package orca

import (
	"context"

	"ProjectOrca/models/notifications"
	pb "ProjectOrca/proto"

	"github.com/joomcode/errorx"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Loop(ctx context.Context, _ *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	_, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	_, err = guild.UpdateQuery(o.store).
		Set("loop = NOT loop").
		Exec(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "update guild loop state")
	}

	go notifications.SendQueueNotificationLog(context.WithoutCancel(ctx), o.store, guild.BotID, guild.ID)

	return &emptypb.Empty{}, nil
}
