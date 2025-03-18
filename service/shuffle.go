package orca

import (
	"context"
	"database/sql"
	"errors"

	"ProjectOrca/models"
	"ProjectOrca/models/notifications"
	pb "ProjectOrca/proto"

	"github.com/sovamorco/errorx"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) ShuffleQueue(ctx context.Context, _ *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
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

		return nil, errorx.Decorate(err, "shuffle queue")
	}

	go notifications.SendQueueNotificationLog(context.WithoutCancel(ctx), o.store, bot.ID, guild.ID)

	return &emptypb.Empty{}, nil
}
