package orca

import (
	"context"
	"database/sql"
	"errors"

	"ProjectOrca/models"
	"ProjectOrca/models/notifications"
	pb "ProjectOrca/proto"

	"github.com/joomcode/errorx"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Remove(ctx context.Context, in *pb.RemoveRequest) (*emptypb.Empty, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
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

		return nil, errorx.Decorate(err, "remove track")
	}

	if position == 0 {
		err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
		if err != nil {
			return nil, errorx.Decorate(err, "send resync")
		}
	} else {
		go notifications.SendQueueNotificationLog(context.WithoutCancel(ctx), o.store, bot.ID, guild.ID)
	}

	return &emptypb.Empty{}, nil
}
