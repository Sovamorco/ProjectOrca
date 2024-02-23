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

func (o *Orca) Remove(ctx context.Context, in *pb.RemoveRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	// should be at least 0
	position := max(0, int(in.Position))

	_, err = o.store.NewDelete().
		Model((*models.RemoteTrack)(nil)).
		ModelTableExpr("tracks").
		TableExpr("(?) AS target", guild.PositionTrackQuery(o.store, position).Column("id")).
		Where("tracks.id = target.id").
		Exec(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// consider it successfully deleted
			return &emptypb.Empty{}, nil
		}

		o.logger.Errorf("Error deleting track from queue: %+v", err)

		return nil, ErrInternal
	}

	if position == 0 {
		err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
		if err != nil {
			o.logger.Errorf("Error sending resync: %+v", err)

			return nil, ErrInternal
		}
	} else {
		go notifications.SendQueueNotificationLog(ctx, o.logger, o.store, bot.ID, guild.ID)
	}

	return &emptypb.Empty{}, nil
}
