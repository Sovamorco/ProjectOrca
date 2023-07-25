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
	return CurrentTrackQuery(store, g.BotID, g.ID)
}

func (g *RemoteGuild) PositionTrackQuery(store *store.Store, position int) *bun.SelectQuery {
	return PositionTrackQuery(store, position, g.BotID, g.ID)
}

func (g *RemoteGuild) TracksQuery(store *store.Store) *bun.SelectQuery {
	return TracksQuery(store, g.BotID, g.ID)
}

func CurrentTrackQuery(store *store.Store, botID, guildID string) *bun.SelectQuery {
	return PositionTrackQuery(store, 0, botID, guildID)
}

func PositionTrackQuery(store *store.Store, position int, botID, guildID string) *bun.SelectQuery {
	return TracksQuery(store, botID, guildID).
		Order("ord_key").
		Offset(position).
		Limit(1)
}

func TracksQuery(store *store.Store, botID, guildID string) *bun.SelectQuery {
	return store.
		NewSelect().
		Model((*RemoteTrack)(nil)).
		Where("bot_id = ?", botID).
		Where("guild_id = ?", guildID)
}
