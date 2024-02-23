package orca

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"ProjectOrca/models"
	pb "ProjectOrca/proto"

	"google.golang.org/protobuf/types/known/durationpb"
)

func (o *Orca) GetCurrent(ctx context.Context, in *pb.GuildOnlyRequest) (*pb.GetCurrentReply, error) {
	_, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	var track models.RemoteTrack

	err = guild.CurrentTrackQuery(o.store).Scan(ctx, &track)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		o.logger.Errorf("Error getting current track: %+v", err)

		return nil, ErrInternal
	}

	return &pb.GetCurrentReply{
		Track:   track.ToProto(),
		Looping: guild.Loop,
		Paused:  guild.Paused,
	}, nil
}

func (o *Orca) GetTracks(ctx context.Context, in *pb.GetTracksRequest) (*pb.GetTracksReply, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	var tracks []*models.RemoteTrack

	var qlen int

	var remaining time.Duration

	q := o.store.
		NewSelect().
		Model(&tracks).
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
		o.logger.Errorf("Error getting queue length: %+v", err)

		return nil, ErrInternal
	}

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
		o.logger.Errorf("Error getting tracks: %+v", err)

		return nil, ErrInternal
	}

	res := make([]*pb.TrackData, len(tracks))
	for i, track := range tracks {
		res[i] = track.ToProto()
	}

	return &pb.GetTracksReply{
		Tracks:      res,
		TotalTracks: int64(qlen),
		Looping:     guild.Loop, Paused: guild.Paused,
		Remaining: durationpb.New(remaining),
	}, nil
}
