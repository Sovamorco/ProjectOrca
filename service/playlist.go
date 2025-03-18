package orca

import (
	"context"
	"fmt"
	"time"

	"ProjectOrca/extractor"
	"ProjectOrca/models"
	"ProjectOrca/models/notifications"
	pb "ProjectOrca/proto"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/sovamorco/errorx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	playlistSizeLimit = 100_000
)

var ErrPlaylistTooLarge = status.Error(codes.InvalidArgument,
	fmt.Sprintf("playlist can have at most %d tracks", playlistSizeLimit))

func (o *Orca) SavePlaylist(ctx context.Context, in *pb.SavePlaylistRequest) (*emptypb.Empty, error) {
	_, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	qlen, err := guild.TracksQuery(o.store).Count(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "get queue length")
	}

	if qlen == 0 {
		return nil, status.Error(codes.InvalidArgument, models.ErrEmptyQueue.Error())
	} else if qlen > playlistSizeLimit {
		return nil, ErrPlaylistTooLarge
	}

	err = o.savePlaylist(ctx, guild, in.UserID, in.Name)
	if err != nil {
		return nil, errorx.Decorate(err, "save playlist")
	}

	return &emptypb.Empty{}, nil
}

func (o *Orca) savePlaylist(ctx context.Context, guild *models.RemoteGuild, userID, name string) error {
	pl := models.Playlist{
		ID:     uuid.New().String(),
		UserID: userID,
		Name:   name,
	}

	tx, err := o.store.Begin()
	if err != nil {
		return errorx.Decorate(err, "begin transcation")
	}

	_, err = tx.
		NewInsert().
		Model(&pl).
		Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "store playlist")
	}

	_, err = tx.
		NewInsert().
		Model((*models.PlaylistTrack)(nil)).
		TableExpr(
			"(?) as queue",
			guild.
				TracksQuery(o.store).
				ColumnExpr("gen_random_uuid(), ?", pl.ID).
				Column("duration", "ord_key", "title", "extraction_url",
					"stream_url", "http_headers", "live", "display_url"),
		).
		Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "store playlist tracks")
	}

	err = tx.Commit()
	if err != nil {
		return errorx.Decorate(err, "commit transaction")
	}

	return nil
}

func (o *Orca) LoadPlaylist(ctx context.Context, in *pb.LoadPlaylistRequest) (*pb.PlayReply, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	qmax := 0.

	qlen, err := guild.TracksQuery(o.store).Count(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "get current queue length")
	}

	err = guild.TracksQuery(o.store).
		ColumnExpr("MAX(ord_key)").
		Scan(ctx, &qmax)
	if err != nil {
		return nil, errorx.Decorate(err, "get current max ord_key")
	}

	qempty := qlen == 0

	protoTracks, total, err := o.addPlaylistTracks(ctx, bot.ID, guild.ID, in.PlaylistID, qlen, qmax)
	if err != nil {
		return nil, errorx.Decorate(err, "add playlist tracks")
	}

	if qempty {
		err = o.queueStartResync(ctx, guild, bot.ID, in.ChannelID)
		if err != nil {
			return nil, err
		}
	} else {
		go notifications.SendQueueNotificationLog(context.WithoutCancel(ctx), o.store, bot.ID, guild.ID)
	}

	return &pb.PlayReply{
		Tracks: protoTracks,
		Total:  int64(total),
	}, nil
}

func (o *Orca) addPlaylistTracks(
	ctx context.Context,
	botID, guildID, playlistID string,
	qlen int,
	qmax float64,
) ([]*pb.TrackData, int, error) {
	logger := zerolog.Ctx(ctx)

	playlistTracks, playlistN, err := o.getPlaylistPreview(ctx, qlen, playlistID)
	if err != nil {
		return nil, 0, errorx.Decorate(err, "get playlist preview")
	}

	logger.Debug().Int("len", playlistN).Msg("Loading playlist")

	baseOrdKey := qmax - playlistTracks[0].OrdKey + edgeOrdKeyDiff

	_, err = o.store.
		NewInsert().
		Model((*models.RemoteTrack)(nil)).
		TableExpr(
			"(?) as playlist",
			o.store.
				NewSelect().
				Model((*models.PlaylistTrack)(nil)).
				Where("playlist_id = ?", playlistID).
				ColumnExpr("gen_random_uuid(), ?, ?, ?", botID, guildID, 0).
				Column("duration").
				ColumnExpr("? + \"ord_key\"", baseOrdKey).
				Column("title", "extraction_url",
					"stream_url", "http_headers", "live", "display_url"),
		).
		Exec(ctx)
	if err != nil {
		return nil, 0, errorx.Decorate(err, "insert playlist into queue")
	}

	tracksData := make([]*pb.TrackData, len(playlistTracks))

	for i, track := range playlistTracks {
		if i < maxPlayReplyTracks {
			tracksData[i] = &pb.TrackData{
				Title:      track.Title,
				DisplayURL: track.DisplayURL,
				Live:       track.Live,
				Position:   durationpb.New(0),
				Duration:   durationpb.New(track.Duration),
			}
		}
	}

	return tracksData, playlistN, nil
}

func (o *Orca) getPlaylistPreview(
	ctx context.Context,
	qlen int,
	playlistID string,
) ([]*models.PlaylistTrack, int, error) {
	var playlistTracks []*models.PlaylistTrack

	playlistN, err := o.store.
		NewSelect().
		Model(&playlistTracks).
		Where("playlist_id = ?", playlistID).
		Order("ord_key").
		Limit(maxPlayReplyTracks).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, errorx.Decorate(err, "get playlist tracks")
	} else if playlistN == 0 {
		return nil, 0, extractor.ErrNoResults
	}

	if qlen+playlistN > queueSizeLimit {
		return nil, 0, ErrQueueTooLarge
	}

	return playlistTracks, playlistN, nil
}

func (o *Orca) ListPlaylists(ctx context.Context, in *pb.ListPlaylistsRequest) (*pb.ListPlaylistsReply, error) {
	_, err := parseBotContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse bot context")
	}

	var playlists []*struct {
		ID            string
		Name          string
		TotalTracks   int64
		TotalDuration time.Duration
	}

	err = o.store.
		NewSelect().
		Model((*models.Playlist)(nil)).
		ModelTableExpr("playlists").
		ColumnExpr("playlists.id, playlists.name").
		ColumnExpr("(?) AS total_tracks",
			o.store.
				NewSelect().
				Model((*models.PlaylistTrack)(nil)).
				Where("playlist_id = playlists.id").
				ColumnExpr("COUNT(*)"),
		).
		ColumnExpr("(?) AS total_duration",
			o.store.
				NewSelect().
				Model((*models.PlaylistTrack)(nil)).
				Where("playlist_id = playlists.id").
				ColumnExpr("SUM(duration)"),
		).
		Where("user_id = ?", in.UserID).
		Scan(ctx, &playlists)
	if err != nil {
		return nil, errorx.Decorate(err, "select playlists for user_id = \"%s\"", in.UserID)
	}

	res := make([]*pb.Playlist, len(playlists))

	for i, pl := range playlists {
		res[i] = &pb.Playlist{
			Id:            pl.ID,
			Name:          pl.Name,
			TotalTracks:   pl.TotalTracks,
			TotalDuration: durationpb.New(pl.TotalDuration),
		}
	}

	return &pb.ListPlaylistsReply{
		Playlists: res,
	}, nil
}
