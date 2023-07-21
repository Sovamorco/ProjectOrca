package models

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"sync"
	"time"

	"ProjectOrca/utils"

	"ProjectOrca/store"

	"github.com/bwmarrin/discordgo"
	"github.com/joomcode/errorx"
	"go.uber.org/zap"
)

const (
	sampleRate  = 48000
	channels    = 2
	frameSizeMs = 20
	bitrate     = 64000 // bits/s
	packetSize  = bitrate * frameSizeMs / 1000 / 8

	bufferMilliseconds = 500
	bufferPackets      = bufferMilliseconds / frameSizeMs

	storeInterval = 5 * time.Second

	playLoopSleep = 50 * time.Millisecond
)

type Guild struct {
	id            string
	botID         string
	botSession    *discordgo.Session
	logger        *zap.SugaredLogger
	store         *store.Store
	vc            *discordgo.VoiceConnection
	vcMu          sync.RWMutex `exhaustruct:"optional"`
	paused        bool
	channelID     string
	storeLoopDone chan struct{}
	playLoopDone  chan struct{}
	resyncPlaying chan struct{}

	track *LocalTrack
}

func NewGuild(
	ctx context.Context,
	id, botID string,
	botSession *discordgo.Session,
	logger *zap.SugaredLogger,
	store *store.Store,
) *Guild {
	logger = logger.Named("guild").With("guild_id", id)
	g := &Guild{
		id:            id,
		botID:         botID,
		botSession:    botSession,
		logger:        logger,
		store:         store,
		vc:            nil,
		paused:        false,
		channelID:     "",
		storeLoopDone: make(chan struct{}, 1),
		playLoopDone:  make(chan struct{}, 1),
		resyncPlaying: make(chan struct{}, 1),

		track: NewLocalTrack(logger, store),
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

func (g *Guild) subTrack(ctx context.Context, seekPos time.Duration) error {
	track, err := g.getNextTrack(ctx)
	if err != nil {
		return errorx.Decorate(err, "get next track")
	}

	l := NewLocalTrack(g.logger, g.store)
	l.remote = track

	if seekPos != utils.MinDuration {
		l.remote.Pos = seekPos
	}

	err = l.initialize(ctx)
	if err != nil {
		return errorx.Decorate(err, "initialize track")
	}

	old := g.track
	g.track = l

	old.cleanup()

	return nil
}

func (g *Guild) ResyncPlaying(ctx context.Context, seekPos time.Duration) {
	// try to change the track ourselves
	err := g.subTrack(ctx, seekPos)
	if err != nil {
		if !errors.Is(err, ErrEmptyQueue) {
			g.logger.Errorf("Error substituting track: %+v", err)
		}

		g.track.remote = nil
		g.track.cleanup()
	}

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
		case <-ticker.C:
		}

		if g.track.remote != nil {
			_, err := g.store.
				NewUpdate().
				Model(g.track.remote).
				Column("pos").
				WherePK().
				Exec(ctx)
			if err != nil {
				g.logger.Errorf("Error storing track position: %+v", err)
			}
		}
	}
}

func (g *Guild) getNextTrack(ctx context.Context) (*RemoteTrack, error) {
	var track RemoteTrack

	err := g.store.
		NewSelect().
		Model(&track).
		Where("bot_id = ? AND guild_id = ?", g.botID, g.id).
		Order("ord_key").
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEmptyQueue
		}

		return nil, errorx.Decorate(err, "get track from store")
	}

	return &track, nil
}

func (g *Guild) playLoop(ctx context.Context) { //nolint:funlen,gocognit,cyclop // fixme
	packet := make([]byte, packetSize)

	for {
		select {
		case <-g.playLoopDone:
			return
		default:
		}

		if g.track.remote == nil {
			err := g.checkForNextTrack(ctx)

			if errors.Is(err, ErrShuttingDown) {
				return
			}

			if errors.Is(err, ErrNoTrack) {
				continue
			}
		}

		if g.getVC() == nil {
			if g.channelID == "" {
				time.Sleep(playLoopSleep)

				continue
			}

			err := g.connect(g.channelID)
			if err != nil {
				g.logger.Errorf("Error connecting to voice channel: %+v", err)
				time.Sleep(playLoopSleep)

				continue
			}
		}

		if g.paused {
			time.Sleep(playLoopSleep)

			continue
		}

		if !g.track.initialized() {
			err := g.track.initialize(ctx)
			if err != nil {
				g.logger.Errorf("Error initializing track: %+v", err)

				g.track.remote = nil

				time.Sleep(playLoopSleep)

				continue
			}
		}

		err := g.track.getPacket(packet)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = g.stop(ctx)
				if err != nil {
					g.logger.Errorf("Error stopping current track: %+v", err)
				}

				continue
			}

			g.logger.Errorf("Error getting packet from stream: %+v", err)
			g.track.cleanup()

			continue
		}

		g.vcMu.RLock()

		select {
		case <-g.playLoopDone:
			g.vcMu.RUnlock()

			return
		case g.vc.OpusSend <- packet:
		}

		g.vcMu.RUnlock()

		if g.track.remote != nil && !g.track.remote.Live {
			g.track.remote.Pos += frameSizeMs * time.Millisecond
		}
	}
}

func (g *Guild) checkForNextTrack(ctx context.Context) error {
	track, err := g.getNextTrack(ctx)
	if err == nil {
		g.track.remote = track

		return nil
	}

	if !errors.Is(err, ErrEmptyQueue) {
		g.logger.Errorf("Error getting track from store: %+v", err)
		time.Sleep(playLoopSleep)

		return ErrNoTrack
	}

	err = g.connect("")
	if err != nil {
		g.logger.Errorf("Error leaving voice channel: %+v", err)
	}

	// queue is empty right now
	// wait for signal on resyncPlaying channel instead of polling database
	select {
	case <-g.playLoopDone:
		return ErrShuttingDown
	case <-g.resyncPlaying:
	}

	return ErrNoTrack
}

func (g *Guild) stop(ctx context.Context) error {
	oldcurr := g.track.remote

	g.track.remote = nil
	g.track.cleanup()

	err := oldcurr.DeleteOrRequeue(ctx, g.store)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return errorx.Decorate(err, "delete or requeue track")
	}

	return nil
}

func (g *Guild) connect(channelID string) error {
	if channelID == "" || g.track.remote == nil {
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
