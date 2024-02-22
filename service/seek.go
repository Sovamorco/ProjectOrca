package orca

import (
	"context"

	"ProjectOrca/models"
	pb "ProjectOrca/proto"
)

func (o *Orca) Seek(ctx context.Context, in *pb.SeekRequest) (*pb.SeekReply, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	o.logger.Infof("Seeking to %.2fs in guild %s", in.Position.AsDuration().Seconds(), in.GuildID)

	_, err = o.store.
		NewUpdate().
		Model((*models.RemoteTrack)(nil)).
		ModelTableExpr("tracks").
		TableExpr("(?) as curr", guild.CurrentTrackQuery(o.store)).
		Set("pos = ?", in.Position.AsDuration()).
		Where("tracks.id = curr.id").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error changing track position: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
	if err != nil {
		o.logger.Errorf("Error sending resync target")

		return nil, ErrInternal
	}

	return &pb.SeekReply{}, nil
}
