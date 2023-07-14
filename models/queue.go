package models

import (
	"ProjectOrca/store"
	"context"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
	"sync"
)

type Queue struct {
	bun.BaseModel `bun:"table:queues"`

	sync.Mutex `bun:"-"`                  // do not store mutex state
	GuildState *GuildState                `bun:"-"` // do not store parent guild state
	Logger     *zap.SugaredLogger         `bun:"-"` // do not store logger
	VC         *discordgo.VoiceConnection `bun:"-"` // do not store voice connection
	Store      *store.Store               `bun:"-"` // do not store the store

	ID        string `bun:",pk"`
	GuildID   string
	ChannelID string
	Tracks    []*MusicTrack `bun:"rel:has-many,join:id=queue_id"`
}

func (g *GuildState) newQueue(channelID string) error {
	g.Queue = &Queue{
		GuildState: g,
		Logger:     g.Logger.Named("queue"),
		Store:      g.Store,

		ID:        uuid.New().String(),
		GuildID:   g.ID,
		ChannelID: channelID,
		Tracks:    make([]*MusicTrack, 0),
	}
	_, err := g.Store.NewInsert().Model(g.Queue).Exec(context.TODO())
	if err != nil {
		return errorx.Decorate(err, "store queue")
	}
	return nil
}

func (q *Queue) Restore(g *GuildState) {
	logger := g.Logger.Named("queue")
	logger.Info("Restoring queue")
	q.GuildState = g
	q.Logger = logger
	q.Store = q.GuildState.Store
	for _, track := range q.Tracks {
		track.Restore(q)
	}
	if q.ChannelID != "" && len(q.Tracks) > 0 {
		go q.start()
	}
}

func (q *Queue) add(ms *MusicTrack) error {
	q.Lock()
	qlen := len(q.Tracks)
	if qlen > 0 {
		ms.OrdKey = q.Tracks[qlen-1].OrdKey + 1
	}
	q.Tracks = append(q.Tracks, ms)
	q.Unlock()
	if qlen == 0 { // a.k.a if there were no tracks in the queue before we added this one
		go q.start()
		return nil
	}
	// reaching here has the same condition as modifying ms.OrdKeys
	_, err := q.Store.NewUpdate().Model(ms).WherePK().Exec(context.TODO())
	return err
}

func (q *Queue) start() {
	q.Logger.Info("Starting playback")
	vc, err := q.GuildState.BotState.Session.ChannelVoiceJoin(q.GuildState.ID, q.ChannelID, false, true)
	if err != nil {
		q.Logger.Errorf("Error joining voice channel: %+v", err)
		return
	}
	q.VC = vc
	defer func(vc *discordgo.VoiceConnection) {
		err := vc.Disconnect()
		if err != nil {
			q.Logger.Errorf("Error leaving voice channel: %+v", err)
		}
	}(vc)

	for len(q.Tracks) > 0 {
		done := make(chan error, 1)
		curr := q.Tracks[0]
		curr.streamToVC(q.VC, done)
		err := <-done
		if err != nil {
			q.Logger.Errorf("Error when streaming track: %+v", err)
		}
		_, err = q.Store.NewDelete().Model(curr).WherePK().Exec(context.TODO())
		if err != nil {
			q.Logger.Errorf("Error deleting track from store: %+v", err)
		}
		// check before indexing because of stop call
		if len(q.Tracks) < 1 {
			break
		}
		q.Lock()
		q.Tracks = q.Tracks[1:]
		q.Unlock()
	}
}
