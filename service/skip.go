package orca

import (
	"context"
	"database/sql"
	"errors"

	"ProjectOrca/models"
	pb "ProjectOrca/proto"

	"github.com/joomcode/errorx"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Skip(ctx context.Context, _ *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	var current models.RemoteTrack

	err = guild.CurrentTrackQuery(o.store).Scan(ctx, &current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, errorx.Decorate(err, "fetch current track")
	}

	err = current.DeleteOrRequeue(ctx, o.store)
	if err != nil {
		return nil, errorx.Decorate(err, "delete current track")
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
	if err != nil {
		return nil, errorx.Decorate(err, "send resync")
	}

	return &emptypb.Empty{}, nil
}
