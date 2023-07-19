package models

import (
	"context"
	"errors"
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

	BotState *BotState          `bun:"-"` // do not store parent bot state
	Logger   *zap.SugaredLogger `bun:"-"` // do not store logger
	Store    *store.Store       `bun:"-"` // do not store the store

	ID      string `bun:",pk"`
	GuildID string
	BotID   string
	Queue   *Queue `bun:"rel:has-one,join:id=guild_id"`
}

var (
	ErrNotPlaying    = errors.New("nothing playing")
	ErrSeekLive      = errors.New("cannot seek live track")
	ErrNotPaused     = errors.New("not paused")
	ErrAlreadyPaused = errors.New("already paused")
)

func (s *BotState) NewGuildState(ctx context.Context, guildID string) (*GuildState, error) {
	// basically check if bot is in guild
	_, err := s.Session.State.Guild(guildID)
	if err != nil {
		return nil, errorx.Decorate(err, "get guild from state")
	}

	gs := &GuildState{
		BotState: s,
		Logger:   s.Logger.Named("guild_state").With("guild_id", guildID),
		Store:    s.Store,

		ID:      uuid.New().String(),
		GuildID: guildID,
		BotID:   s.ID,
		Queue:   nil,
	}

	_, err = s.Store.NewInsert().Model(gs).Exec(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "store guild state")
	}

	s.SetGuildState(guildID, gs)

	return gs, nil
}

func (g *GuildState) Restore(ctx context.Context, s *BotState) {
	logger := s.Logger.Named("guild_state").With("guild_id", g.GuildID)

	logger.Info("Restoring guild state")

	g.BotState = s
	g.Logger = logger
	g.Store = g.BotState.Store

	if g.Queue != nil {
		g.Queue.Restore(ctx, g)
	}
}

func (g *GuildState) gracefulShutdown() {
	if g.Queue != nil && g.Queue.VC != nil && g.Queue.VC.Ready {
		err := g.Queue.VC.Disconnect()
		if err != nil {
			g.Logger.Errorf("Error disconnecting from voice: %s", err)
		}
	}
}

func (g *GuildState) PlayTrack(ctx context.Context, channelID, url string, position int) (*pb.TrackData, error) {
	var ms *MusicTrack

	if !utils.URLRx.MatchString(url) {
		url = "ytsearch:" + url
	}

	if g.Queue == nil {
		err := g.newQueue(ctx, channelID)
		if err != nil {
			return nil, errorx.Decorate(err, "create queue")
		}
	}

	ms, err := g.Queue.newMusicTrack(ctx, url)
	if err != nil {
		return nil, errorx.Decorate(err, "create music track")
	}

	err = g.Queue.add(ctx, ms, position)
	if err != nil {
		return nil, errorx.Decorate(err, "add track to Queue")
	}

	return ms.ToProto(), nil
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

	g.Queue.Lock()

	if len(g.Queue.Tracks) < 1 {
		g.Queue.Unlock()

		return ErrNotPlaying
	}

	g.Queue.Tracks = g.Queue.Tracks[:1] // leave only current track
	g.Queue.Stop()                      // stop current track

	g.Queue.Unlock()

	_, err := g.Store.NewDelete().Model((*MusicTrack)(nil)).Where("queue_id = ?", g.Queue.ID).Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "delete queue tracks")
	}

	return nil
}

func (g *GuildState) Seek(pos time.Duration) error {
	if g.Queue == nil {
		return ErrNotPlaying
	}

	g.Queue.Lock()

	if len(g.Queue.Tracks) < 1 {
		g.Queue.Unlock()

		return ErrNotPlaying
	}

	curr := g.Queue.Tracks[0]

	g.Queue.Unlock()

	if curr.Live {
		return ErrSeekLive
	}

	err := curr.Seek(pos)
	if err != nil {
		return errorx.Decorate(err, "seek")
	}

	return nil
}

func (g *GuildState) Pause() error {
	if g.Queue == nil || len(g.Queue.Tracks) < 1 {
		return ErrNotPlaying
	}

	if len(g.Queue.Playing) < 1 {
		return ErrAlreadyPaused
	}

	<-g.Queue.Playing

	return nil
}

func (g *GuildState) Resume() error {
	if g.Queue == nil || len(g.Queue.Tracks) < 1 {
		return ErrNotPlaying
	}

	if len(g.Queue.Playing) > 0 {
		return ErrNotPaused
	}

	g.Queue.Playing <- struct{}{}

	return nil
}
