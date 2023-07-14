package models

import (
	pb "ProjectOrca/proto"
	"ProjectOrca/store"
	"ProjectOrca/utils"
	"context"
	"errors"
	"github.com/joomcode/errorx"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
	"time"
)

type GuildState struct {
	bun.BaseModel `bun:"table:guild_states"`

	BotState *BotState          `bun:"-"` // do not store parent bot state
	Logger   *zap.SugaredLogger `bun:"-"` // do not store logger
	Store    *store.Store       `bun:"-"` // do not store the store

	ID           string `bun:",pk"`
	BotID        string
	Queue        *Queue `bun:"rel:has-one,join:id=guild_id"`
	Volume       float32
	TargetVolume float32
}

func (s *BotState) NewGuildState(guildID string) (*GuildState, error) {
	gs := &GuildState{
		BotState: s,
		Logger:   s.Logger.Named("guild_state").With("guild_id", guildID),
		Store:    s.Store,

		ID:           guildID,
		BotID:        s.ID,
		Volume:       1,
		TargetVolume: 1,
	}
	_, err := s.Store.NewInsert().Model(gs).Exec(context.TODO())
	if err != nil {
		return nil, errorx.Decorate(err, "store guild state")
	}
	s.SetGuildState(guildID, gs)
	return gs, nil
}

func (g *GuildState) Restore(s *BotState) {
	logger := s.Logger.Named("guild_state").With("guild_id", g.ID)
	logger.Info("Restoring guild state")
	g.BotState = s
	g.Logger = logger
	g.Store = g.BotState.Store
	if g.Queue != nil {
		g.Queue.Restore(g)
	}
}

func (g *GuildState) gracefulShutdown() {

}

func (g *GuildState) PlaySound(channelID, url string) (*pb.TrackData, error) {
	var ms *MusicTrack

	if !utils.UrlRx.MatchString(url) {
		url = "ytsearch:" + url
	}

	if g.Queue == nil {
		err := g.newQueue(channelID)
		if err != nil {
			return nil, errorx.Decorate(err, "create queue")
		}
	}

	ms, err := g.Queue.newMusicTrack(url)
	if err != nil {
		return nil, errorx.Decorate(err, "create music track")
	}

	g.Queue.add(ms)
	if err != nil {
		return nil, errorx.Decorate(err, "add track to Queue")
	}
	return ms.TrackData, nil
}

func (g *GuildState) Skip() error {
	if len(g.Queue.Tracks) < 1 {
		return errors.New("nothing playing")
	}
	g.Queue.Tracks[0].Stop <- struct{}{}
	return nil
}

func (g *GuildState) Stop() error {
	if len(g.Queue.Tracks) < 1 {
		return errors.New("Nothing playing")
	}
	current := g.Queue.Tracks[0]
	g.Queue.Tracks = nil
	current.Stop <- struct{}{}
	_, err := g.Store.NewDelete().Model((*MusicTrack)(nil)).Where("queue_id = ?", g.Queue.ID).Exec(context.TODO())
	return err
}

func (g *GuildState) Seek(pos time.Duration) error {
	if len(g.Queue.Tracks) < 1 {
		return errors.New("Nothing playing")
	}
	err := g.Queue.Tracks[0].Seek(pos)
	if err != nil {
		return errorx.Decorate(err, "seek")
	}
	return nil
}
