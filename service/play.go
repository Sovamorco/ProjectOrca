package orca

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ProjectOrca/models"
	"ProjectOrca/models/notifications"
	"ProjectOrca/utils"

	pb "ProjectOrca/proto"

	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
)

const (
	defaultPrevOrdKey = 0
	defaultNextOrdKey = defaultPrevOrdKey + edgeOrdKeyDiff

	edgeOrdKeyDiff = 100

	maxPlayReplyTracks = 100

	queueSizeLimit = 500_000
)

var ErrQueueTooLarge = utils.MustCreateStatus(codes.OutOfRange,
	fmt.Sprintf("queue can have at most %d tracks", queueSizeLimit), &pb.ErrorCodeWrapper{
		Code: pb.ErrorCode_ErrQueueTooLarge,
	}).Err()

func (o *Orca) Play(ctx context.Context, in *pb.PlayRequest) (*pb.PlayReply, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	tracksData, total, affectedFirst, err := o.addTracks(ctx, bot, guild, in.Url, int(in.Position))
	if err != nil {
		return nil, errorx.Decorate(err, "add tracks")
	}

	// that concerns current track and guild voice state
	if affectedFirst {
		err = o.queueStartResync(ctx, guild, bot.ID, in.ChannelID)
		if err != nil {
			return nil, errorx.Decorate(err, "resync guild")
		}
	} else {
		go notifications.SendQueueNotificationLog(context.WithoutCancel(ctx), o.store, bot.ID, guild.ID)
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
	logger := zerolog.Ctx(ctx)

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
		Limit(2).                   //nolint:mnd // previous and next track in theory
		Rows(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Error getting ordkeys from store")

		return defaultPrevOrdKey, defaultNextOrdKey, true
	}

	defer func() {
		err = rows.Close()
		if err != nil {
			logger.Error().Err(err).Msg("Error closing rows")
		}
	}()

	return o.chooseOrdKeyRangeFromRows(ctx, position, rows)
}

func (o *Orca) chooseOrdKeyRangeFromRows(ctx context.Context, position int, rows *sql.Rows) (float64, float64, bool) {
	logger := zerolog.Ctx(ctx)

	var prevOrdKey, nextOrdKey float64

	rows.Next()

	if err := rows.Err(); err != nil {
		logger.Error().Err(err).Msg("Error getting next row")

		return defaultPrevOrdKey, defaultNextOrdKey, true
	}

	// special case - there is no track preceding the position, prevOrdKey should be nextOrdKey - 100
	if position == 0 {
		err := rows.Scan(&nextOrdKey)
		if err != nil {
			logger.Error().Err(err).Msg("Error scanning track 0 from store")

			return defaultPrevOrdKey, defaultNextOrdKey, true
		}

		rows.Next() // skip second scanned row to not get error on close

		return nextOrdKey - edgeOrdKeyDiff, nextOrdKey, true
	}

	err := rows.Scan(&prevOrdKey)
	if err != nil {
		logger.Error().Err(err).Msg("Error scanning prev track from store")

		return defaultPrevOrdKey, defaultNextOrdKey, true
	}

	// special case - there is no track succeeding the position, nextOrdKey should be prevOrdKey + 100
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			logger.Error().Err(err).Msg("Error getting next row")

			return defaultPrevOrdKey, defaultNextOrdKey, true
		}

		nextOrdKey = prevOrdKey + edgeOrdKeyDiff
	} else {
		err = rows.Scan(&nextOrdKey)
		if err != nil {
			logger.Error().Err(err).Msg("Error scanning next track from store")

			return defaultPrevOrdKey, defaultNextOrdKey, true
		}
	}

	return prevOrdKey, nextOrdKey, false
}
