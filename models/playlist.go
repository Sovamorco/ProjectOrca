package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/uptrace/bun"
)

type Playlist struct {
	bun.BaseModel `bun:"table:playlists" exhaustruct:"optional"`

	ID     string `bun:",pk"`
	UserID string
	Name   string
}

type PlaylistTrack struct {
	bun.BaseModel `bun:"table:playlist_tracks" exhaustruct:"optional"`

	ID            string `bun:",pk"`
	PlaylistID    string
	Duration      time.Duration
	OrdKey        float64
	Title         string
	ExtractionURL string
	DisplayURL    string
	StreamURL     string
	HTTPHeaders   map[string]string
	Live          bool
}

func (t *PlaylistTrack) ToRemote(botID, guildID string, baseOrdKey float64) *RemoteTrack {
	return &RemoteTrack{
		ID:            uuid.New().String(),
		BotID:         botID,
		GuildID:       guildID,
		Pos:           0,
		Duration:      t.Duration,
		OrdKey:        baseOrdKey + t.OrdKey,
		Title:         t.Title,
		ExtractionURL: t.ExtractionURL,
		DisplayURL:    t.DisplayURL,
		StreamURL:     t.StreamURL,
		HTTPHeaders:   t.HTTPHeaders,
		Live:          t.Live,
	}
}
