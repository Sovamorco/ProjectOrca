package orca

import (
	"context"

	"ProjectOrca/models/notifications"
	pb "ProjectOrca/proto"

	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Loop(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	_, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	_, err = guild.UpdateQuery(o.store).
		Set("loop = NOT loop").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating loop state: %+v", err)

		return nil, ErrInternal
	}

	go notifications.SendQueueNotificationLog(ctx, o.logger, o.store, guild.BotID, guild.ID)

	return &emptypb.Empty{}, nil
}
