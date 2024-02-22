package orca

import (
	"context"

	"ProjectOrca/models/notifications"
	pb "ProjectOrca/proto"

	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Pause(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	_, err = guild.
		UpdateQuery(o.store).
		Set("paused = true").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating pause state: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync: %+v", err)

		return nil, ErrInternal
	}

	notifications.SendQueueNotificationLog(ctx, o.logger, o.store, bot.ID, guild.ID)

	return &emptypb.Empty{}, nil
}

func (o *Orca) Resume(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	_, err = guild.
		UpdateQuery(o.store).
		Set("paused = false").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating pause state: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync: %+v", err)

		return nil, ErrInternal
	}

	notifications.SendQueueNotificationLog(ctx, o.logger, o.store, bot.ID, guild.ID)

	return &emptypb.Empty{}, nil
}
