package models

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"ProjectOrca/extractor"
	"ProjectOrca/models/notifications"

	"ProjectOrca/store"

	"github.com/bwmarrin/discordgo"
	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
)

const (
	sampleRate     = 48000
	channels       = 1
	frameSizeMs    = 20
	bitrate        = 128000 // bits/s
	packetSize     = bitrate * frameSizeMs / 1000 / 8
	packetBurstNum = 10

	bufferMilliseconds = 500 // also dynaudnorm (possibly) has its own buffer
	bufferPackets      = bufferMilliseconds / frameSizeMs

	storeInterval = 1 * time.Second

	playLoopSleep = 50 * time.Millisecond

	opusSendTimeout = 1 * time.Second

	cmdWaitTimeout = 5 * time.Second

	checkVCPeriod  = 200 * time.Millisecond
	checkVCTimeout = 10 * time.Second
)

type posMsg struct {
	pos time.Duration
	id  string
}

type Guild struct {
	// constant values
	id         string
	botID      string
	botSession *discordgo.Session
	store      *store.Store
	extractors *extractor.Extractors

	// concurrency-safe
	posChan chan posMsg

	// signal channels
	resync        chan struct{}
	resyncPlaying chan struct{}
	playing       chan struct{}

	// potentially changeable lockable values
	vc   *discordgo.VoiceConnection
	vcMu sync.RWMutex `exhaustruct:"optional"`
}

func NewGuild(
	ctx context.Context,
	id, botID string,
	botSession *discordgo.Session,
	store *store.Store,
	extractors *extractor.Extractors,
) *Guild {
	g := &Guild{
		id:         id,
		botID:      botID,
		botSession: botSession,
		store:      store,
		extractors: extractors,

		posChan: make(chan posMsg, 1),

		resync:        make(chan struct{}, 1),
		resyncPlaying: make(chan struct{}, 1),
		playing:       make(chan struct{}, 1),

		vc: nil,
	}

	go g.storeLoop(ctx)
	go g.playLoop(ctx)

	return g
}

func (g *Guild) getVC() *discordgo.VoiceConnection {
	g.vcMu.RLock()
	defer g.vcMu.RUnlock()

	return g.vc
}

func (g *Guild) gracefulShutdown(ctx context.Context) {
	logger := zerolog.Ctx(ctx)

	if g.getVC() != nil {
		g.vcMu.Lock()
		err := g.vc.Disconnect()
		g.vcMu.Unlock()

		if err != nil {
			logger.Error().Err(err).Msg("Error disconnecting from voice channel")
		}
	}
}

func (g *Guild) ResyncPlaying() {
	// make sure this does not block
	select {
	case g.resyncPlaying <- struct{}{}:
	default:
	}
}

func (g *Guild) storeLoop(ctx context.Context) {
	logger := zerolog.Ctx(ctx)

	ticker := time.NewTicker(storeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case pos := <-g.posChan:
			// check if ticker passed
			select {
			case <-ticker.C:
			default:
				continue
			}

			_, err := g.store.
				NewUpdate().
				Model((*RemoteTrack)(nil)).
				Set("pos = ?", pos.pos).
				Where("id = ?", pos.id).
				Exec(ctx)
			if err != nil {
				logger.Error().
					Str("trackId", pos.id).
					Float64("pos", pos.pos.Seconds()).
					Err(err).Msg("Error storing track position")
			}
		}
	}
}

func (g *Guild) getNextTrack(ctx context.Context) (*RemoteTrack, error) {
	var track RemoteTrack

	err := CurrentTrackQuery(g.store, g.botID, g.id).Scan(ctx, &track)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEmptyQueue
		}

		return nil, errorx.Decorate(err, "get track from store")
	}

	return &track, nil
}

func (g *Guild) playLoop(ctx context.Context) {
	logger := zerolog.Ctx(ctx)

	track := NewTrack(ctx, g)

	var err error

	for {
		err = g.playLoopPreconditions(ctx, track)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}

			logger.Error().Err(err).Msg("Failed play loop preconditions")

			time.Sleep(playLoopSleep)

			continue
		}
	}
}

// playLoopPreconditions checks for all the preconditions for playing the track.
func (g *Guild) playLoopPreconditions(ctx context.Context, track *Track) error {
	// if we need to resync playing - reset current playing track
	select {
	case <-ctx.Done():
		track.clean(ctx)

		return context.Canceled
	case <-g.resyncPlaying:
		track.clean(ctx)

		go notifications.SendQueueNotificationLog(ctx, g.store, g.botID, g.id)
	default:
	}

	err := track.nextTrackPrecondition(ctx)
	if err != nil {
		return errorx.Decorate(err, "next track precondition")
	}

	err = g.vcPrecondition(ctx)
	if err != nil {
		return errorx.Decorate(err, "voice connection precondition")
	}

	err = track.packetPrecondition(ctx)
	if err != nil {
		return errorx.Decorate(err, "packet precondition")
	}

	// try to consume playing from itself to verify that the track is, indeed, playing
	select {
	case <-ctx.Done():
		return context.Canceled
	case g.playing <- <-g.playing:
	}

	return nil
}

func (g *Guild) vcPrecondition(ctx context.Context) error {
	if vc := g.getVC(); vc != nil {
		if !vc.Ready {
			g.waitForReady(ctx)
		}

		return nil
	}

	err := g.checkForVC(ctx)
	if err != nil {
		return errorx.Decorate(err, "check for voice connection")
	}

	return nil
}

func (g *Guild) waitForReady(ctx context.Context) {
	readyTimeout := time.After(checkVCTimeout)

	for {
		select {
		case <-time.After(checkVCPeriod):
			vc := g.getVC()
			if vc == nil || vc.Ready {
				return
			}
		case <-readyTimeout:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (g *Guild) checkForVC(ctx context.Context) error {
	remote, err := g.getRemote(ctx)
	if err != nil {
		return errorx.Decorate(err, "get remote guild")
	}

	if remote.ChannelID != "" {
		err = g.connect(ctx, remote.ChannelID)
		if err != nil {
			return errorx.Decorate(err, "connect to voice channel")
		}

		return nil
	}

	// wait for resync
	select {
	case <-ctx.Done():
		return context.Canceled
	case <-g.resync:
	}

	return nil
}

func (g *Guild) getRemote(ctx context.Context) (*RemoteGuild, error) {
	var r RemoteGuild

	r.BotID, r.ID = g.botID, g.id

	err := g.store.
		NewSelect().
		Model(&r).
		WherePK().
		Scan(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "get remote guild")
	}

	return &r, nil
}

func (g *Guild) connect(ctx context.Context, channelID string) error {
	_, err := g.store.
		NewUpdate().
		Model((*RemoteGuild)(nil)).
		Set("channel_id = ?", channelID).
		Where("bot_id = ?", g.botID).
		Where("id = ?", g.id).
		Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "store channel id")
	}

	if channelID == "" {
		if g.getVC() != nil {
			g.vcMu.Lock()

			err := g.vc.Disconnect()
			if err != nil {
				g.vcMu.Unlock()

				return errorx.Decorate(err, "disconnect from voice channel")
			}

			g.vc = nil

			g.vcMu.Unlock()
		}

		return nil
	}

	if vc := g.getVC(); vc == nil || vc.ChannelID != channelID {
		vc, err := g.botSession.ChannelVoiceJoin(g.id, channelID, false, true)
		if err != nil {
			return errorx.Decorate(err, "join voice channel")
		}

		vc.LogLevel = discordgo.LogInformational

		g.vcMu.Lock()
		g.vc = vc
		g.vcMu.Unlock()
	}

	return nil
}
