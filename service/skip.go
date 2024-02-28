package orca

import (
	"context"
	"database/sql"
	"errors"

	"ProjectOrca/models"
	pb "ProjectOrca/proto"

	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Skip(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	logger := o.logger.With(
		zap.String("bot_id", bot.ID),
		zap.String("guild_id", guild.ID),
	)

	var current models.RemoteTrack

	err = guild.CurrentTrackQuery(o.store).Scan(ctx, &current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Errorf("Error getting current track: %+v", err)

		return nil, ErrInternal
	}

	err = current.DeleteOrRequeue(ctx, o.store)
	if err != nil {
		logger.Errorf("Error deleting or requeuing current track: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
	if err != nil {
		logger.Errorf("Error sending resync message: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}
