package orca

import (
	"context"

	"ProjectOrca/models"
	"ProjectOrca/models/notifications"
	pb "ProjectOrca/proto"

	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) updatePauseState(
	ctx context.Context,
	bot *models.RemoteBot, guild *models.RemoteGuild,
	paused bool,
) error {
	_, err := guild.
		UpdateQuery(o.store).
		Set("paused = ?", paused).
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating pause state: %+v", err)

		return ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync: %+v", err)

		return ErrInternal
	}

	go notifications.SendQueueNotificationLog(context.WithoutCancel(ctx), o.logger, o.store, bot.ID, guild.ID)

	return nil
}

func (o *Orca) Pause(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	err = o.updatePauseState(ctx, bot, guild, true)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (o *Orca) Resume(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	err = o.updatePauseState(ctx, bot, guild, false)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
