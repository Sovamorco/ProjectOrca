package models

import (
	"ProjectOrca/store"

	"github.com/uptrace/bun"
)

type RemoteGuild struct {
	bun.BaseModel `bun:"table:guilds" exhaustruct:"optional"`

	BotID     string `bun:",pk"`
	ID        string `bun:",pk"`
	ChannelID string
	Paused    bool
	Loop      bool
}

func NewRemoteGuild(botID, id string) *RemoteGuild {
	return &RemoteGuild{
		BotID:     botID,
		ID:        id,
		ChannelID: "",
		Paused:    false,
		Loop:      false,
	}
}

func (g *RemoteGuild) UpdateQuery(store *store.Store) *bun.UpdateQuery {
	return store.
		NewUpdate().
		Model(g).
		WherePK()
}

func (g *RemoteGuild) CurrentTrackQuery(store *store.Store) *bun.SelectQuery {
	return store.
		NewSelect().
		Model((*RemoteTrack)(nil)).
		Where("bot_id = ? AND guild_id = ?", g.BotID, g.ID).
		Order("ord_key").
		Limit(1)
}
