package models

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"ProjectOrca/extractor"

	"ProjectOrca/store"

	"github.com/bwmarrin/discordgo"
	"github.com/joomcode/errorx"
	"go.uber.org/zap"
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

	storeInterval = 1000 * time.Millisecond

	playLoopSleep = 50 * time.Millisecond

	opusSendTimeout = 1 * time.Second

	cmdWaitTimeout = 5 * time.Second
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
	logger     *zap.SugaredLogger
	store      *store.Store
	extractors *extractor.Extractors

	// concurrency-safe
	posChan chan posMsg

	// signal channels
	storeLoopDone chan struct{}
	playLoopDone  chan struct{}
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
	logger *zap.SugaredLogger,
	store *store.Store,
	extractors *extractor.Extractors,
) *Guild {
	logger = logger.Named("guild").With("guild_id", id)
	g := &Guild{
		id:         id,
		botID:      botID,
		botSession: botSession,
		logger:     logger,
		store:      store,
		extractors: extractors,

		posChan: make(chan posMsg, 1),

		storeLoopDone: make(chan struct{}, 1),
		playLoopDone:  make(chan struct{}, 1),
		resync:        make(chan struct{}, 1),
		resyncPlaying: make(chan struct{}, 1),
		playing:       make(chan struct{}, 1),

		vc: nil,
	}

	go g.storeLoop(context.WithoutCancel(ctx))
	go g.playLoop(context.WithoutCancel(ctx))

	return g
}

func (g *Guild) getVC() *discordgo.VoiceConnection {
	g.vcMu.RLock()
	defer g.vcMu.RUnlock()

	return g.vc
}

func (g *Guild) gracefulShutdown() {
	if g.getVC() != nil {
		g.vcMu.Lock()
		err := g.vc.Disconnect()
		g.vcMu.Unlock()

		if err != nil {
			g.logger.Errorf("Error disconnecting from voice channel: %+v", err)
		}
	}

	// just in case - make sure these do not block
	select {
	case g.storeLoopDone <- struct{}{}:
	default:
	}

	select {
	case g.playLoopDone <- struct{}{}:
	default:
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
	ticker := time.NewTicker(storeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.storeLoopDone:
			return
		case pos := <-g.posChan:
			// check if ticker passed
			select {
			case <-ticker.C:
				_, err := g.store.
					NewUpdate().
					Model((*RemoteTrack)(nil)).
					Set("pos = ?", pos.pos).
					Where("id = ?", pos.id).
					Exec(ctx)
				if err != nil {
					g.logger.Errorf("Error storing track position: %+v", err)
				}
			default:
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
	track := NewTrack(g)

	var err error

	for {
		err = g.playLoopPreconditions(ctx, track)

		switch {
		case errors.Is(err, ErrShuttingDown):
			track.shutdown()

			return
		case err != nil:
			time.Sleep(playLoopSleep)

			continue
		}
	}
}

// playLoopPreconditions checks for all the preconditions for playing the track.
func (g *Guild) playLoopPreconditions(ctx context.Context, track *Track) error {
	// if we need to resync playing - reset current playing track
	select {
	case <-g.playLoopDone:
		track.clean()

		return ErrShuttingDown
	case <-g.resyncPlaying:
		track.clean()
	default:
	}

	err := track.nextTrackPrecondition(ctx)
	if err != nil {
		g.logger.Debugf("Failed next track precondition: %+v", err)

		return err
	}

	err = g.vcPrecondition(ctx)
	if err != nil {
		g.logger.Debugf("Failed vc precondition: %+v", err)

		return err
	}

	err = track.packetPrecondition(ctx)
	if err != nil {
		g.logger.Debugf("Failed packet precondition: %+v", err)

		return err
	}

	// try to consume playing from itself to verify that the track is, indeed, playing
	select {
	case <-g.playLoopDone:
		return ErrShuttingDown
	case g.playing <- <-g.playing:
	}

	return nil
}

func (g *Guild) vcPrecondition(ctx context.Context) error {
	if vc := g.getVC(); vc != nil {
		if !vc.Ready {
			err := vc.Disconnect()
			if err != nil {
				g.logger.Errorf("Failed to disconnect from broken channel")
			}

			return ErrNoVC
		}

		return nil
	}

	err := g.checkForVC(ctx)

	switch {
	case errors.Is(err, ErrShuttingDown):
		return err
	case errors.Is(err, ErrNoVC):
		return ErrNoVC
	case err != nil:
		g.logger.Errorf("Error checking for voice connection: %+v", err)

		return ErrNoVC
	}

	return nil
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
	case <-g.playLoopDone:
		return ErrShuttingDown
	case <-g.resync:
	}

	return ErrNoVC
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

		g.vcMu.Lock()
		g.vc = vc
		g.vcMu.Unlock()
	}

	return nil
}
