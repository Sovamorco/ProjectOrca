package orca

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"ProjectOrca/models"
	pb "ProjectOrca/proto"

	"github.com/sovamorco/errorx"
	"google.golang.org/protobuf/types/known/durationpb"
)

func (o *Orca) GetTracks(ctx context.Context, in *pb.GetTracksRequest) (*pb.GetTracksReply, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	var tracks []*models.RemoteTrack

	err = o.store.
		NewSelect().
		Model(&tracks).
		Where("bot_id = ?", bot.ID).
		Where("guild_id = ?", guild.ID).
		Order("ord_key").
		Offset(int(in.Start)).
		Limit(int(in.End - in.Start)).
		Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, errorx.Decorate(err, "get tracks")
	}

	res := make([]*pb.TrackData, len(tracks))
	for i, track := range tracks {
		res[i] = track.ToProto()
	}

	return &pb.GetTracksReply{
		Tracks:  res,
		Looping: guild.Loop,
		Paused:  guild.Paused,
	}, nil
}

func (o *Orca) GetQueueState(ctx context.Context, _ *pb.GuildOnlyRequest) (*pb.GetQueueStateReply, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	var qlen int

	var remaining time.Duration

	var track models.RemoteTrack

	q := o.store.
		NewSelect().
		Model(&track).
		Where("bot_id = ?", bot.ID).
		Where("guild_id = ?", guild.ID).
		ColumnExpr("COUNT(*)")

	// subtract position of every track if queue is not looping, because we want remaining duration
	if guild.Loop {
		q = q.ColumnExpr("SUM(duration)")
	} else {
		q = q.ColumnExpr("SUM(duration) - SUM(pos)")
	}

	err = q.Scan(ctx, &qlen, &remaining)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, errorx.Decorate(err, "get queue state")
	}

	return &pb.GetQueueStateReply{
		TotalTracks: int64(qlen),
		Remaining:   durationpb.New(remaining),
		Looping:     guild.Loop,
		Paused:      guild.Paused,
	}, nil
}
