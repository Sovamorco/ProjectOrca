package models

import (
	"ProjectOrca/store"
	"context"
	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
	"slices"
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

func (q *Queue) add(ms *MusicTrack, position int) (*MusicTrack, error) {
	retms := ms
	// we have to very carefully sanitize position because slices.Insert can panic
	q.Lock()
	qlen := len(q.Tracks)
	// if there are other tracks in the queue
	if qlen > 0 {
		if position < 0 {
			// if position is negative, interpret that as index from end
			// e.g. -1 means put track as last, -2 means put track as before last, etc.
			position = qlen + position + 1
		}
		if position >= qlen {
			// if position is after last track - put as last
			position = qlen
			ms.OrdKey = q.Tracks[qlen-1].OrdKey + 1
		} else if position == 0 {
			// if position is at 0, put the track as the first track
			ms.OrdKey = q.Tracks[0].OrdKey - 1
		} else {
			// if position is somewhere between first and last, put in-between position and position - 1
			ms.OrdKey = (q.Tracks[position-1].OrdKey + q.Tracks[position].OrdKey) / 2
		}
	} else {
		position = 0
	}
	q.Tracks = slices.Insert(q.Tracks, position, ms)
	q.Unlock()
	if qlen == 0 { // a.k.a if there were no tracks in the queue before we added this one
		go q.start()
		return retms, nil
	} else if position == 0 {
		q.Tracks[0].Lock()
		err := q.Tracks[0].getStream()
		q.Tracks[0].Unlock()
		if err != nil {
			return nil, errorx.Decorate(err, "get stream")
		}
		// very very bad scary things
		q.Tracks[0].Lock()
		q.Tracks[1].Lock()
		//goland:noinspection GoVetCopyLock
		*q.Tracks[0], *q.Tracks[1] = *q.Tracks[1], *q.Tracks[0]
		q.Tracks[0], q.Tracks[1] = q.Tracks[1], q.Tracks[0]
		retms = q.Tracks[0]
		q.Tracks[0].Unlock()
		q.Tracks[1].Unlock()
	}
	// reaching here has the same condition as modifying ms.OrdKeys
	_, err := q.Store.NewUpdate().Model(retms).WherePK().Exec(context.TODO())
	if err != nil {
		return nil, errorx.Decorate(err, "store music track")
	}
	return retms, nil
}

func (q *Queue) start() {
	q.Logger.Info("Starting playback")
	vc, err := q.GuildState.BotState.Session.ChannelVoiceJoin(q.GuildState.GuildID, q.ChannelID, false, true)
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
