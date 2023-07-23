package models

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"sync"
	"time"

	"ProjectOrca/extractor"

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

	storeInterval = 1 * time.Second

	playLoopSleep = 50 * time.Millisecond
)

type Guild struct {
	// constant values
	id         string
	botID      string
	botSession *discordgo.Session
	logger     *zap.SugaredLogger
	store      *store.Store
	extractors *extractor.Extractors

	// signal channels
	storeLoopDone chan struct{}
	playLoopDone  chan struct{}
	resync        chan struct{}
	resyncPlaying chan struct{}
	playing       chan struct{}

	// potentially changeable lockable values
	vc      *discordgo.VoiceConnection
	vcMu    sync.RWMutex `exhaustruct:"optional"`
	track   *LocalTrack
	trackMu sync.RWMutex `exhaustruct:"optional"`
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

		storeLoopDone: make(chan struct{}, 1),
		playLoopDone:  make(chan struct{}, 1),
		resync:        make(chan struct{}, 1),
		resyncPlaying: make(chan struct{}, 1),
		playing:       make(chan struct{}, 1),

		vc:    nil,
		track: NewLocalTrack(logger, store, extractors),
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

func (g *Guild) getTrack() *LocalTrack {
	g.trackMu.RLock()
	defer g.trackMu.RUnlock()

	return g.track
}

func (g *Guild) setTrack(v *LocalTrack) {
	g.trackMu.Lock()
	defer g.trackMu.Unlock()

	g.track = v
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

	l := NewLocalTrack(g.logger, g.store, g.extractors)
	l.setRemote(track)

	if seekPos != utils.MinDuration {
		l.setPos(seekPos)
	}

	err = l.initialize(ctx)
	if err != nil {
		return errorx.Decorate(err, "initialize track")
	}

	old := g.getTrack()
	g.setTrack(l)

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

		track := g.getTrack()
		track.setRemote(nil)
		track.cleanup()
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

		track := g.getTrack()
		if remote, pos := track.getRemote(), track.getPos(); remote != nil {
			_, err := g.store.
				NewUpdate().
				Model(remote).
				Set("pos = ?", pos).
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

func (g *Guild) playLoop(ctx context.Context) { //nolint:cyclop // FIXME
	packet := make([]byte, packetSize)

	for {
		select {
		case <-g.playLoopDone:
			return
		default:
		}

		fulfilled, err := g.playLoopPreconditions(ctx)
		if err != nil {
			return // it should only ever return ErrShuttingDown, so just return
		}

		if !fulfilled {
			time.Sleep(playLoopSleep)

			continue
		}

		err = g.getTrack().getPacket(packet)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = g.stop(ctx)
				if err != nil {
					g.logger.Errorf("Error stopping current track: %+v", err)
				}

				continue
			}

			g.logger.Errorf("Error getting packet from stream: %+v", err)
			g.getTrack().cleanup()

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

		track := g.getTrack()
		if remote := track.getRemote(); remote != nil && !remote.Live {
			track.setPos(track.getPos() + frameSizeMs*time.Millisecond)
		}
	}
}

// playLoopPreconditions checks for all the preconditions for playing the track.
func (g *Guild) playLoopPreconditions(ctx context.Context) (bool, error) { //nolint:cyclop // FIXME
	if g.getTrack().getRemote() == nil {
		err := g.checkForNextTrack(ctx)

		if errors.Is(err, ErrShuttingDown) { //nolint:gocritic // I swear if else here is better
			return false, ErrShuttingDown
		} else if errors.Is(err, ErrNoTrack) {
			return false, nil
		} else if err != nil {
			g.logger.Errorf("Error checking for next track: %+v", err)

			return false, nil
		}
	}

	if g.getVC() == nil {
		err := g.checkForVC(ctx)

		if errors.Is(err, ErrShuttingDown) { //nolint:gocritic // I swear if else here is better
			return false, ErrShuttingDown
		} else if errors.Is(err, ErrNoVC) {
			return false, nil
		} else if err != nil {
			g.logger.Errorf("Error checking for voice connection: %+v", err)

			return false, nil
		}
	}

	if track := g.getTrack(); !track.initialized() {
		err := track.initialize(ctx)
		if err != nil {
			g.logger.Errorf("Error initializing track: %+v", err)

			track.setRemote(nil)

			return false, nil
		}
	}

	// try to consume playing from itself to verify that the track is, indeed, playing
	select {
	case <-g.playLoopDone:
		return false, ErrShuttingDown
	case g.playing <- <-g.playing:
	}

	return true, nil
}

func (g *Guild) checkForNextTrack(ctx context.Context) error {
	track, err := g.getNextTrack(ctx)
	if err == nil {
		g.getTrack().setRemote(track)

		return nil
	}

	if !errors.Is(err, ErrEmptyQueue) {
		return errorx.Decorate(err, "get track from store")
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

func (g *Guild) checkForVC(ctx context.Context) error {
	remote, err := g.getRemote(ctx)
	if err != nil {
		return errorx.Decorate(err, "get remote guild")
	}

	if remote.ChannelID != "" {
		err = g.connect(remote.ChannelID)
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

func (g *Guild) stop(ctx context.Context) error {
	track := g.getTrack()
	oldcurr := track.getRemote()

	track.setRemote(nil)
	track.cleanup()

	err := oldcurr.DeleteOrRequeue(ctx, g.store)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return errorx.Decorate(err, "delete or requeue track")
	}

	return nil
}

func (g *Guild) connect(channelID string) error {
	if channelID == "" || g.getTrack().getRemote() == nil {
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
