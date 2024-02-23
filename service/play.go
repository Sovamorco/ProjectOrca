package orca

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ProjectOrca/models"
	"ProjectOrca/models/notifications"

	pb "ProjectOrca/proto"

	"github.com/joomcode/errorx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultPrevOrdKey = 0
	defaultNextOrdKey = defaultPrevOrdKey + edgeOrdKeyDiff

	edgeOrdKeyDiff = 100

	maxPlayReplyTracks = 100

	queueSizeLimit = 500_000
)

var ErrQueueTooLarge = status.Error(codes.InvalidArgument,
	fmt.Sprintf("queue can have at most %d tracks", queueSizeLimit))

func (o *Orca) Play(ctx context.Context, in *pb.PlayRequest) (*pb.PlayReply, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	tracksData, total, affectedFirst, err := o.addTracks(ctx, bot, guild, in.Url, int(in.Position))
	if err != nil {
		if errors.Is(err, ErrQueueTooLarge) {
			return nil, ErrQueueTooLarge
		}

		o.logger.Errorf("Error adding tracks: %+v", err)

		return nil, ErrInternal
	}

	// that concerns current track and guild voice state
	if affectedFirst {
		err = o.queueStartResync(ctx, guild, bot.ID, in.ChannelID)
		if err != nil {
			return nil, err
		}
	} else {
		go notifications.SendQueueNotificationLog(ctx, o.logger, o.store, bot.ID, guild.ID)
	}

	return &pb.PlayReply{
		Tracks: tracksData,
		Total:  int64(total),
	}, nil
}

func (o *Orca) addTracks(
	ctx context.Context,
	bot *models.RemoteBot,
	guild *models.RemoteGuild,
	url string,
	position int,
) ([]*pb.TrackData, int, bool, error) {
	tracks, err := models.NewRemoteTracks(ctx, bot.ID, guild.ID, url, o.extractors)
	if err != nil {
		return nil, 0, false, errorx.Decorate(err, "get remote tracks")
	}

	qlen, err := guild.TracksQuery(o.store).Count(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, 0, false, errorx.Decorate(err, "count tracks")
	}

	if qlen+len(tracks) > queueSizeLimit {
		return nil, 0, false, ErrQueueTooLarge
	}

	prevOrdKey, nextOrdKey, affectedFirst := o.chooseOrdKeyRange(ctx, guild, qlen, position)

	models.SetOrdKeys(tracks, prevOrdKey, nextOrdKey)

	_, err = o.store.
		NewInsert().
		Model(&tracks).
		Exec(ctx)
	if err != nil {
		return nil, 0, false, errorx.Decorate(err, "store tracks")
	}

	res := make([]*pb.TrackData, min(len(tracks), maxPlayReplyTracks))
	for i, track := range tracks[:len(res)] {
		res[i] = track.ToProto()
	}

	return res, len(tracks), affectedFirst, nil
}

func (o *Orca) chooseOrdKeyRange(
	ctx context.Context, guild *models.RemoteGuild, qlen, position int,
) (float64, float64, bool) {
	// special case - if no tracks in queue - insert all tracks with ordkeys from 0 to 100
	if qlen == 0 {
		return defaultPrevOrdKey, defaultNextOrdKey, true
	}

	// if position is negative, interpret that as index from end (wrap around)
	// e.g. -1 means put track as last, -2 means put track as before last, etc.
	if position < 0 {
		position = qlen + position + 1
	}

	// position has to be at least 0 and at most len(queue)
	position = max(0, min(qlen, position))

	rows, err := guild.TracksQuery(o.store).
		Column("ord_key").
		Order("ord_key").
		Offset(max(position-1, 0)). // have a separate check for position 0 later
		Limit(2).                   //nolint:gomnd // previous and next track in theory
		Rows(ctx)
	if err != nil {
		o.logger.Errorf("Error getting ordkeys from store: %+v", err)

		return defaultPrevOrdKey, defaultNextOrdKey, true
	}

	defer func() {
		err = rows.Close()
		if err != nil {
			o.logger.Errorf("Error closing rows: %+v", err)
		}
	}()

	return o.chooseOrdKeyRangeFromRows(position, rows)
}

func (o *Orca) chooseOrdKeyRangeFromRows(position int, rows *sql.Rows) (float64, float64, bool) {
	var prevOrdKey, nextOrdKey float64

	rows.Next()

	if err := rows.Err(); err != nil {
		o.logger.Errorf("Error getting next row: %+v", err)

		return defaultPrevOrdKey, defaultNextOrdKey, true
	}

	// special case - there is no track preceding the position, prevOrdKey should be nextOrdKey - 100
	if position == 0 {
		err := rows.Scan(&nextOrdKey)
		if err != nil {
			o.logger.Errorf("Error scanning track 0 from store: %+v", err)

			return defaultPrevOrdKey, defaultNextOrdKey, true
		}

		rows.Next() // skip second scanned row to not get error on close

		return nextOrdKey - edgeOrdKeyDiff, nextOrdKey, true
	}

	err := rows.Scan(&prevOrdKey)
	if err != nil {
		o.logger.Errorf("Error scanning prev track from store: %+v", err)

		return defaultPrevOrdKey, defaultNextOrdKey, true
	}

	// special case - there is no track succeeding the position, nextOrdKey should be prevOrdKey + 100
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			o.logger.Errorf("Error getting next row: %+v", err)

			return defaultPrevOrdKey, defaultNextOrdKey, true
		}

		nextOrdKey = prevOrdKey + edgeOrdKeyDiff
	} else {
		err = rows.Scan(&nextOrdKey)
		if err != nil {
			o.logger.Errorf("Error scanning next track from store: %+v", err)

			return defaultPrevOrdKey, defaultNextOrdKey, true
		}
	}

	return prevOrdKey, nextOrdKey, false
}
