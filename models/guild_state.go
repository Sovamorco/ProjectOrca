package models

import (
	"context"
	"sync"
	"time"

	"ProjectOrca/store"
	"ProjectOrca/utils"

	pb "ProjectOrca/proto"

	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

type GuildState struct {
	bun.BaseModel `bun:"table:guilds" exhaustruct:"optional"`

	// -- private non-stored values
	botState *BotState
	logger   *zap.SugaredLogger
	store    *store.Store
	// mutexes for specific values
	queueMu sync.RWMutex `exhaustruct:"optional"`
	// -- end private non-stored values

	ID      string `bun:",pk"`
	GuildID string
	BotID   string
	Queue   *Queue `bun:"rel:has-one,join:id=guild_id"`
}

func (s *BotState) NewGuildState(ctx context.Context, guildID string) (*GuildState, error) {
	// basically check if bot is in guild
	_, err := s.session.State.Guild(guildID)
	if err != nil {
		return nil, errorx.Decorate(err, "get guild from state")
	}

	gs := &GuildState{
		botState: s,
		logger:   s.logger.Named("guild_state").With("guild_id", guildID),
		store:    s.store,

		ID:      uuid.New().String(),
		GuildID: guildID,
		BotID:   s.ID,
		Queue:   nil,
	}

	_, err = s.store.NewInsert().Model(gs).Exec(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "store guild state")
	}

	s.SetGuildState(guildID, gs)

	return gs, nil
}

func (g *GuildState) Restore(ctx context.Context, s *BotState) {
	logger := s.logger.Named("guild_state").With("guild_id", g.GuildID)

	logger.Info("Restoring guild state")

	g.botState = s
	g.logger = logger
	g.store = g.botState.store

	if g.Queue != nil {
		g.Queue.Restore(ctx, g)
	}
}

func (g *GuildState) gracefulShutdown() {
	if g.Queue != nil && g.Queue.vc != nil && g.Queue.vc.Ready {
		err := g.Queue.vc.Disconnect()
		if err != nil {
			g.logger.Errorf("Error disconnecting from voice: %s", err)
		}
	}
}

func (g *GuildState) PlayTracks(ctx context.Context, channelID, url string, position int) ([]*pb.TrackData, error) {
	if !utils.URLRx.MatchString(url) {
		url = "ytsearch:" + url
	}

	if g.Queue == nil {
		err := g.newQueue(ctx, channelID)
		if err != nil {
			return nil, errorx.Decorate(err, "create queue")
		}
	}

	mss, err := g.Queue.newMusicTracks(ctx, url)
	if err != nil {
		return nil, errorx.Decorate(err, "create music tracks")
	}

	res := make([]*pb.TrackData, len(mss))

	for i, ms := range mss {
		err = g.Queue.add(ctx, ms, position)
		if err != nil {
			return nil, errorx.Decorate(err, "add track to Queue")
		}

		res[i] = ms.ToProto()
	}

	return res, nil
}

func (g *GuildState) Skip() error {
	if g.Queue == nil || len(g.Queue.Tracks) < 1 {
		return ErrNotPlaying
	}

	g.Queue.Stop()

	return nil
}

func (g *GuildState) Stop(ctx context.Context) error {
	if g.Queue == nil {
		return ErrNotPlaying
	}

	g.Queue.tracksMu.Lock()

	if len(g.Queue.Tracks) < 1 {
		g.Queue.tracksMu.Unlock()

		return ErrNotPlaying
	}

	g.Queue.SetLoop(false)              // disable looping
	g.Queue.Tracks = g.Queue.Tracks[:1] // leave only current track
	g.Queue.Stop()                      // stop current track

	g.Queue.tracksMu.Unlock()

	_, err := g.store.NewDelete().Model((*MusicTrack)(nil)).Where("queue_id = ?", g.Queue.ID).Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "delete queue tracks")
	}

	g.Queue.loopMu.RLock()
	_, err = g.store.NewUpdate().Model(g.Queue).Column("loop").WherePK().Exec(ctx)
	g.Queue.loopMu.RUnlock()

	if err != nil {
		return errorx.Decorate(err, "update queue")
	}

	return nil
}

func (g *GuildState) Seek(ctx context.Context, pos time.Duration) error {
	if g.Queue == nil {
		return ErrNotPlaying
	}

	g.Queue.tracksMu.RLock()

	if len(g.Queue.Tracks) < 1 {
		g.Queue.tracksMu.RUnlock()

		return ErrNotPlaying
	}

	curr := g.Queue.Tracks[0]

	g.Queue.tracksMu.RUnlock()

	if curr.Live {
		return ErrSeekLive
	}

	curr.Seek(pos)

	curr.posMu.RLock()
	_, err := g.store.NewUpdate().Model(curr).Column("pos").WherePK().Exec(ctx)
	curr.posMu.RUnlock()

	if err != nil {
		return errorx.Decorate(err, "store new position")
	}

	return nil
}

func (g *GuildState) Pause(ctx context.Context) error {
	if g.Queue == nil || len(g.Queue.GetTracks()) < 1 {
		return ErrNotPlaying
	}

	if g.Queue.GetPaused() {
		return ErrAlreadyPaused
	}

	g.Queue.Pause()

	g.Queue.pausedMu.RLock()
	_, err := g.store.NewUpdate().Model(g.Queue).Column("paused").WherePK().Exec(ctx)
	g.Queue.pausedMu.RUnlock()

	if err != nil {
		return errorx.Decorate(err, "store paused state")
	}

	return nil
}

func (g *GuildState) Resume(ctx context.Context) error {
	if g.Queue == nil || len(g.Queue.GetTracks()) < 1 {
		return ErrNotPlaying
	}

	if !g.Queue.GetPaused() {
		return ErrNotPaused
	}

	g.Queue.Resume()

	g.Queue.pausedMu.RLock()
	_, err := g.store.NewUpdate().Model(g.Queue).Column("paused").WherePK().Exec(ctx)
	g.Queue.pausedMu.RUnlock()

	if err != nil {
		return errorx.Decorate(err, "store paused state")
	}

	return nil
}
