package orca

import (
	"context"

	"ProjectOrca/models"
	pb "ProjectOrca/proto"

	"github.com/sovamorco/errorx"
)

func (o *Orca) Seek(ctx context.Context, in *pb.SeekRequest) (*pb.SeekReply, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	_, err = o.store.
		NewUpdate().
		Model((*models.RemoteTrack)(nil)).
		ModelTableExpr("tracks").
		TableExpr("(?) as curr", guild.CurrentTrackQuery(o.store)).
		Set("pos = ?", in.Position.AsDuration()).
		Where("tracks.id = curr.id").
		Exec(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "seek to position")
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
	if err != nil {
		return nil, errorx.Decorate(err, "send resync")
	}

	return &pb.SeekReply{}, nil
}
