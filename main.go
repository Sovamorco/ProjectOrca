package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"ProjectOrca/vk"

	"ProjectOrca/spotify"
	"ProjectOrca/ytdl"

	"github.com/hashicorp/vault-client-go"

	"github.com/go-redsync/redsync/v4"

	"ProjectOrca/extractor"

	"google.golang.org/protobuf/types/known/emptypb"

	"ProjectOrca/migrations"
	"ProjectOrca/models"
	pb "ProjectOrca/proto"
	"ProjectOrca/store"

	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"github.com/redis/go-redis/v9"
	gvault "github.com/sovamorco/gommon/vault"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ResyncTarget int

type ResyncMessage struct {
	Bot     string         `json:"bot"`
	Guild   string         `json:"guild"`
	Targets []ResyncTarget `json:"targets"`
}

const (
	MigrationsTimeout = time.Second * 300
	ResyncsChannel    = "resync"

	edgeOrdKeyDiff    = 100
	defaultPrevOrdKey = 0
	defaultNextOrdKey = defaultPrevOrdKey + edgeOrdKeyDiff

	maxPlayReplyTracks = 100

	queueSizeLimit = 500_000

	playlistSizeLimit = 100_000
)

// resync targets.
const (
	ResyncTargetCurrent ResyncTarget = iota
	ResyncTargetGuild
)

var (
	ErrFailedToAuthenticate = status.Error(codes.Unauthenticated, "failed to authenticate bot")
	ErrInternal             = status.Error(codes.Internal, "internal error")

	ErrQueueTooLarge = status.Error(codes.InvalidArgument,
		fmt.Sprintf("queue can have at most %d tracks", queueSizeLimit))

	ErrPlaylistTooLarge = status.Error(codes.InvalidArgument,
		fmt.Sprintf("playlist can have at most %d tracks", playlistSizeLimit))
)

type orcaServer struct {
	pb.UnimplementedOrcaServer `exhaustruct:"optional"`

	// static values
	id         string
	logger     *zap.SugaredLogger
	store      *store.Store
	extractors *extractor.Extractors

	// concurrency-safe
	states sync.Map
}

func newOrcaServer(
	ctx context.Context, logger *zap.SugaredLogger, st *store.Store, config *Config,
) (*orcaServer, error) {
	serverLogger := logger.Named("server")

	orca := &orcaServer{
		id:         uuid.New().String(),
		logger:     serverLogger,
		store:      st,
		extractors: extractor.NewExtractors(),

		states: sync.Map{},
	}

	if config.Spotify != nil {
		s, err := spotify.New(
			ctx,
			config.Spotify.ClientID,
			config.Spotify.ClientSecret,
		)
		if err != nil {
			return nil, errorx.Decorate(err, "init spotify module")
		}

		orca.extractors.AddExtractor(s)
	}

	if config.VK != nil {
		v, err := vk.New(
			ctx,
			orca.logger,
			config.VK.Token,
		)
		if err != nil {
			return nil, errorx.Decorate(err, "init vk module")
		}

		orca.extractors.AddExtractor(v)
	}

	orca.extractors.AddExtractor(ytdl.New(orca.logger))

	orca.store.Subscribe(ctx, orca.handleResync, fmt.Sprintf("%s:%s", config.Redis.Prefix, ResyncsChannel))
	orca.store.Subscribe(ctx, orca.handleKeyDel,
		"__keyevent@0__:del",
		"__keyevent@0__:expired",
	)

	err := orca.initFromStore(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "init from store")
	}

	return orca, nil
}

func (o *orcaServer) gracefulShutdown(ctx context.Context) {
	o.logger.Info("Shutting down")

	o.states.Range(func(_, value any) bool {
		state, _ := value.(*models.Bot)

		state.GracefulShutdown()

		return true
	})

	o.store.Unsubscribe(ctx)

	o.store.GracefulShutdown(ctx)
}

func (o *orcaServer) initFromStore(ctx context.Context) error {
	var bots []*models.RemoteBot

	err := o.store.
		NewSelect().
		Model(&bots).
		Column("id", "token").
		Scan(ctx)
	if err != nil {
		return errorx.Decorate(err, "select bots")
	}

	for _, state := range bots {
		err = o.store.Lock(ctx, state.ID)
		if err != nil {
			var et *redsync.ErrTaken

			if !errors.As(err, &et) {
				o.logger.Errorf("Error locking state: %+v", err)
			}

			continue
		}

		o.logger.Infof("Locked state %s", state.ID)

		if s := o.getStateByToken(state.Token); s != nil {
			// state already controlled, no need to init
			continue
		}

		localState, err := models.NewBot(o.logger, o.store, o.extractors, state.Token)
		if err != nil {
			o.logger.Errorf("Could not create local state for remote state %s: %+v", state.ID, err)
		}

		o.states.Store(localState.GetID(), localState)

		go localState.FullResync(ctx)
	}

	return nil
}

func (o *orcaServer) getStateByToken(token string) *models.Bot {
	var res *models.Bot

	o.states.Range(func(_, value any) bool {
		state, _ := value.(*models.Bot)
		if state.GetToken() == token {
			res = state

			return false
		}

		return true
	})

	return res
}

// do not use for authentication-related purposes.
func (o *orcaServer) getStateByID(id string) *models.Bot {
	val, ok := o.states.Load(id)
	if !ok {
		return nil
	}

	state, _ := val.(*models.Bot)

	return state
}

func (o *orcaServer) parseIncomingContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		o.logger.Debug("missing metadata")

		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	tokenS := md.Get("token")
	if len(tokenS) != 1 {
		o.logger.Debug("missing token")

		return "", status.Error(codes.Unauthenticated, "missing token")
	}

	token := tokenS[0]

	return token, nil
}

func (o *orcaServer) authenticate(ctx context.Context) (*models.RemoteBot, error) {
	token, err := o.parseIncomingContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse context")
	}

	var s models.RemoteBot

	err = o.store.
		NewSelect().
		Model(&s).
		Where("token = ?", token).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return o.register(ctx, token)
		}

		return nil, errorx.Decorate(err, "get state from store")
	}

	return &s, nil
}

func (o *orcaServer) authenticateWithGuild(ctx context.Context, guildID string) (
	*models.RemoteBot,
	*models.RemoteGuild,
	error,
) {
	bot, err := o.authenticate(ctx)
	if err != nil {
		return nil, nil, errorx.Decorate(err, "authenticate bot")
	}

	var r models.RemoteGuild
	r.ID = guildID
	r.BotID = bot.ID

	err = o.store.
		NewSelect().
		Model(&r).
		WherePK().
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			g := models.NewRemoteGuild(bot.ID, guildID)

			_, err = o.store.
				NewInsert().
				Model(g).
				Exec(ctx)
			if err != nil {
				return nil, nil, errorx.Decorate(err, "create guild")
			}

			return bot, g, nil
		}

		return nil, nil, errorx.Decorate(err, "get guild from store")
	}

	return bot, &r, nil
}

func (o *orcaServer) register(ctx context.Context, token string) (*models.RemoteBot, error) {
	state, err := models.NewBot(o.logger, o.store, o.extractors, token)
	if err != nil {
		return nil, errorx.Decorate(err, "create local bot state")
	}

	r := models.NewRemoteBot(state.GetID(), state.GetToken())

	_, err = o.store.
		NewInsert().
		Model(r).
		Exec(ctx)
	if err != nil {
		state.GracefulShutdown()

		return nil, errorx.Decorate(err, "store remote bot state")
	}

	o.states.Store(state.GetID(), state)

	return r, nil
}

func (o *orcaServer) Join(ctx context.Context, in *pb.JoinRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	if guild.ChannelID == in.ChannelID {
		return &emptypb.Empty{}, nil
	}

	_, err = guild.UpdateQuery(o.store).
		Set("channel_id = ?", in.ChannelID).
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating guild channel id: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync request: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) Leave(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	if guild.ChannelID == "" {
		return &emptypb.Empty{}, nil
	}

	_, err = guild.
		UpdateQuery(o.store).
		Set("channel_id = NULL").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating guild: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync request: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) Play(ctx context.Context, in *pb.PlayRequest) (*pb.PlayReply, error) {
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
	}

	return &pb.PlayReply{
		Tracks: tracksData,
		Total:  int64(total),
	}, nil
}

func (o *orcaServer) addTracks(
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

func (o *orcaServer) Skip(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	o.logger.Infof("Skipping track in guild %s", in.GuildID)

	var current models.RemoteTrack

	err = guild.CurrentTrackQuery(o.store).Scan(ctx, &current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		o.logger.Errorf("Error getting current track: %+v", err)

		return nil, ErrInternal
	}

	err = current.DeleteOrRequeue(ctx, o.store)
	if err != nil {
		o.logger.Errorf("Error deleting or requeuing current track: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
	if err != nil {
		o.logger.Errorf("Error sending resync message: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) Stop(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	o.logger.Infof("Stopping playback in guild %s", in.GuildID)

	_, err = o.store.
		NewDelete().
		Model((*models.RemoteTrack)(nil)).
		Where("bot_id = ?", bot.ID).
		Where("guild_id = ?", guild.ID).
		Exec(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		o.logger.Errorf("Error deleting all tracks: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
	if err != nil {
		o.logger.Errorf("Error sending resync message: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) Seek(ctx context.Context, in *pb.SeekRequest) (*pb.SeekReply, error) {
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

func (o *orcaServer) Loop(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	_, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	_, err = guild.UpdateQuery(o.store).
		Set("loop = NOT loop").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating loop state: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) ShuffleQueue(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
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

		o.logger.Errorf("Error updating ord keys: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) GetTracks(ctx context.Context, in *pb.GetTracksRequest) (*pb.GetTracksReply, error) {
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
		Looping:     guild.Loop,
		Remaining:   durationpb.New(remaining),
	}, nil
}

func (o *orcaServer) Pause(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	_, err = guild.
		UpdateQuery(o.store).
		Set("paused = true").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating pause state: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) Resume(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	_, err = guild.
		UpdateQuery(o.store).
		Set("paused = false").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating pause state: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) Remove(ctx context.Context, in *pb.RemoveRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
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

		o.logger.Errorf("Error deleting track from queue: %+v", err)

		return nil, ErrInternal
	}

	//goland:noinspection GoBoolExpressions // goland is crazy thinking this is always true
	if position == 0 {
		err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
		if err != nil {
			o.logger.Errorf("Error sending resync: %+v", err)

			return nil, ErrInternal
		}
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) SavePlaylist(ctx context.Context, in *pb.SavePlaylistRequest) (*emptypb.Empty, error) {
	_, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	qlen, err := guild.TracksQuery(o.store).Count(ctx)
	if err != nil {
		o.logger.Errorf("Error counting tracks from queue: %+v", err)

		return nil, ErrInternal
	}

	if qlen == 0 {
		return nil, status.Error(codes.InvalidArgument, models.ErrEmptyQueue.Error())
	} else if qlen > playlistSizeLimit {
		return nil, ErrPlaylistTooLarge
	}

	err = o.savePlaylist(ctx, guild, in.UserID, in.Name)
	if err != nil {
		o.logger.Errorf("Error saving playlist: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *orcaServer) savePlaylist(ctx context.Context, guild *models.RemoteGuild, userID, name string) error {
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

func (o *orcaServer) ListPlaylists(ctx context.Context, in *pb.ListPlaylistsRequest) (*pb.ListPlaylistsReply, error) {
	_, _, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
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
		o.logger.Errorf("Error selecting user playlists: %+v", err)

		return nil, ErrInternal
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

func (o *orcaServer) LoadPlaylist(ctx context.Context, in *pb.LoadPlaylistRequest) (*pb.PlayReply, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	qmax := 0.

	qlen, err := guild.TracksQuery(o.store).Count(ctx)
	if err != nil {
		o.logger.Errorf("Error counting queue tracks: %+v", err)

		return nil, ErrInternal
	}

	err = guild.TracksQuery(o.store).
		ColumnExpr("MAX(ord_key)").
		Scan(ctx, &qmax)
	if err != nil {
		o.logger.Errorf("Error getting max current ord key: %+v", err)

		return nil, ErrInternal
	}

	qempty := qlen == 0

	protoTracks, total, err := o.addPlaylistTracks(ctx, bot.ID, guild.ID, in.PlaylistID, qlen, qmax)
	if err != nil {
		if errors.Is(err, ErrQueueTooLarge) {
			return nil, ErrQueueTooLarge
		}

		o.logger.Errorf("Error adding playlist tracks: %+v", err)

		return nil, ErrInternal
	}

	if qempty {
		err = o.queueStartResync(ctx, guild, bot.ID, in.ChannelID)
		if err != nil {
			return nil, err
		}
	}

	return &pb.PlayReply{
		Tracks: protoTracks,
		Total:  int64(total),
	}, nil
}

func (o *orcaServer) addPlaylistTracks(
	ctx context.Context,
	botID, guildID, playlistID string,
	qlen int,
	qmax float64,
) ([]*pb.TrackData, int, error) {
	playlistTracks, playlistN, err := o.getPlaylistPreview(ctx, qlen, playlistID)
	if err != nil {
		return nil, 0, errorx.Decorate(err, "get playlist preview")
	}

	o.logger.Debugf("Loading playlist with length: %d", playlistN)

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

func (o *orcaServer) getPlaylistPreview(
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

func (o *orcaServer) Health(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (o *orcaServer) queueStartResync(ctx context.Context, guild *models.RemoteGuild, botID, channelID string) error {
	if guild.ChannelID != channelID {
		guild.ChannelID = channelID

		_, err := guild.UpdateQuery(o.store).Column("channel_id").Exec(ctx)
		if err != nil {
			o.logger.Errorf("Error updating guild channel id: %+v", err)

			return ErrInternal
		}
	}

	err := o.sendResync(ctx, botID, guild.ID, ResyncTargetCurrent, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync message: %+v", err)

		return ErrInternal
	}

	return nil
}

func main() {
	coreLogger, err := zap.NewDevelopment(zap.IncreaseLevel(zapcore.DebugLevel))
	if err != nil {
		panic(err)
	}

	logger := coreLogger.Sugar()

	ctx := context.Background()

	var vc *vault.Client

	var config *Config

	vc, err = gvault.ClientFromEnv(ctx)
	if err != nil {
		logger.Debugf("Failed to create vault client: %+v", err)

		config, err = loadConfig(ctx)
	} else {
		config, err = loadConfigVault(ctx, vc)
	}

	if err != nil {
		logger.Fatalf("Error loading config: %+v", err)
	}

	err = run(ctx, logger, vc, config)
	if err != nil {
		logger.Fatalf("Error running: %+v", err)
	}
}

func doMigrate(ctx context.Context, logger *zap.SugaredLogger, db *bun.DB) error {
	logger = logger.Named("migrate")

	migrateContext, migrateCancel := context.WithTimeout(ctx, MigrationsTimeout)
	defer migrateCancel()

	m, err := migrations.NewMigrations()
	if err != nil {
		return errorx.Decorate(err, "get migrations")
	}

	migrator := migrate.NewMigrator(db, m, migrate.WithMarkAppliedOnSuccess(true))

	err = migrator.Init(migrateContext)
	if err != nil {
		return errorx.Decorate(err, "migrate init")
	}

	if err = migrator.Lock(ctx); err != nil {
		return errorx.Decorate(err, "lock")
	}

	defer func(migrator *migrate.Migrator, ctx context.Context) {
		err := migrator.Unlock(ctx)
		if err != nil {
			logger.Errorf("Error unlocking migrator: %+v", err)
		}
	}(migrator, ctx)

	group, err := migrator.Migrate(ctx)
	if err != nil {
		return errorx.Decorate(err, "migrate")
	}

	if group.IsZero() {
		logger.Debug("No migrations to run")
	} else {
		logger.Infof("Ran %d migrations", len(group.Migrations))
	}

	return nil
}

func run(
	ctx context.Context, logger *zap.SugaredLogger, vc *vault.Client, config *Config,
) error {
	st := store.NewStore(ctx, logger, &store.Config{
		DB:     config.DB,
		Broker: config.Redis,
	}, vc)

	err := doMigrate(ctx, logger, st.DB)
	if err != nil {
		return errorx.Decorate(err, "do migrations")
	}

	port := 8590

	var lc net.ListenConfig

	lis, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", config.Port))
	if err != nil {
		return errorx.Decorate(err, "listen")
	}

	orca, err := newOrcaServer(ctx, logger, st, config)
	if err != nil {
		return errorx.Decorate(err, "create orca server")
	}

	grpcServer := grpc.NewServer()
	pb.RegisterOrcaServer(grpcServer, orca)

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- grpcServer.Serve(lis)
	}()

	defer grpcServer.GracefulStop()

	defer orca.gracefulShutdown(ctx)

	logger.Infof("Started gRPC server on port %d", port)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err = <-serverErrors:
		logger.Errorf("Error listening for gRPC calls: %+v", err)

		return errorx.Decorate(err, "listen")
	case sig := <-sc:
		logger.Infof("Exiting with signal %v", sig)

		return nil
	}
}

func (o *orcaServer) doResync(ctx context.Context, logger *zap.SugaredLogger, managed *models.Bot, r *ResyncMessage) {
	var wg sync.WaitGroup

	for _, target := range r.Targets {
		target := target

		wg.Add(1)

		go func() {
			defer wg.Done()

			switch target {
			case ResyncTargetGuild:
				err := managed.ResyncGuild(ctx, r.Guild)
				if err != nil {
					logger.Errorf("Error resyncing guild: %+v", err)
				}
			case ResyncTargetCurrent:
				managed.ResyncGuildTrack(ctx, r.Guild)
			}
		}()
	}

	wg.Wait()
}

func (o *orcaServer) handleResync(ctx context.Context, logger *zap.SugaredLogger, msg *redis.Message) error {
	r := new(ResyncMessage)

	err := json.Unmarshal([]byte(msg.Payload), r)
	if err != nil {
		return errorx.Decorate(err, "unmarshal resync")
	}

	managed := o.getStateByID(r.Bot)
	if managed == nil {
		logger.Debugf("Resync message for non-managed recipient %s, skipping", r.Bot)

		return nil
	}

	o.doResync(ctx, logger, managed, r)

	return nil
}

func (o *orcaServer) sendResync(ctx context.Context, botID, guildID string, targets ...ResyncTarget) error {
	b, err := json.Marshal(ResyncMessage{
		Bot:     botID,
		Guild:   guildID,
		Targets: targets,
	})
	if err != nil {
		return errorx.Decorate(err, "marshal resync message")
	}

	err = o.store.Publish(ctx, fmt.Sprintf("%s:%s", o.store.RedisPrefix, ResyncsChannel), b).Err()
	if err != nil {
		return errorx.Decorate(err, "publish resync message")
	}

	return nil
}

func (o *orcaServer) handleKeyDel(ctx context.Context, logger *zap.SugaredLogger, msg *redis.Message) error {
	val := msg.Payload
	pref := fmt.Sprintf("%s:", o.store.RedisPrefix)

	if !strings.HasPrefix(val, pref) {
		return nil
	}

	val = strings.TrimPrefix(val, pref)

	_, exists := o.states.Load(val)
	if exists {
		logger.Fatalf("Own managed state %s expired", val) // TODO: gracefully remove this state?

		return nil // never reached because logger.Fatal calls os.Exit, more of an IDE/linter hint
	}

	logger.Infof("Lock for %s expired, trying takeover", val)

	err := o.initFromStore(ctx)
	if err != nil {
		return errorx.Decorate(err, "init after takeover")
	}

	return nil
}

func (o *orcaServer) chooseOrdKeyRange(
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

func (o *orcaServer) chooseOrdKeyRangeFromRows(position int, rows *sql.Rows) (float64, float64, bool) {
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
