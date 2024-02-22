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

func (o *Orca) ShuffleQueue(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	_, err = o.store.
		NewUpdate().
		Model((*models.RemoteTrack)(nil)).
		ModelTableExpr("tracks").
		TableExpr(
			"(?) AS curr",
			guild.CurrentTrackQuery(o.store).
				Column("id", "ord_key"),
		).
		Set("ord_key = RANDOM() * ? + 1 + curr.ord_key", edgeOrdKeyDiff).
		Where("bot_id = ?", bot.ID).
		Where("guild_id = ?", guild.ID).
		Where("tracks.id != curr.id").
		Exec(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &emptypb.Empty{}, nil
		}

		o.logger.Errorf("Error updating ord keys: %+v", err)

		return nil, ErrInternal
	}

	notifications.SendQueueNotificationLog(ctx, o.logger, o.store, bot.ID, guild.ID)

	return &emptypb.Empty{}, nil
}
