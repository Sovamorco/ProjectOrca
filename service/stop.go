package orca

import (
	"context"
	"database/sql"
	"errors"

	"ProjectOrca/models"
	"ProjectOrca/models/notifications"
	pb "ProjectOrca/proto"

	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Stop(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	o.logger.Infof("Stopping playback in guild %s", in.GuildID)

	_, err = o.store.
		NewDelete().
		Model((*models.RemoteTrack)(nil)).
		Where("bot_id = ?", bot.ID).
		Where("guild_id = ?", guild.ID).
		Exec(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		o.logger.Errorf("Error deleting all tracks: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
	if err != nil {
		o.logger.Errorf("Error sending resync message: %+v", err)

		return nil, ErrInternal
	}

	notifications.SendQueueNotificationLog(ctx, o.logger, o.store, bot.ID, guild.ID)

	return &emptypb.Empty{}, nil
}
