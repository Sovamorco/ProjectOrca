package models

import (
	"context"
	"slices"
	"sync"

	"ProjectOrca/store"
	"ProjectOrca/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

type Queue struct {
	bun.BaseModel `bun:"table:queues" exhaustruct:"optional"`

	sync.Mutex `bun:"-" exhaustruct:"optional"` // do not store mutex state
	GuildState *GuildState                      `bun:"-"` // do not store parent guild state
	Logger     *zap.SugaredLogger               `bun:"-"` // do not store logger
	VC         *discordgo.VoiceConnection       `bun:"-"` // do not store voice connection
	Store      *store.Store                     `bun:"-"` // do not store the store

	ID        string `bun:",pk"`
	GuildID   string
	ChannelID string
	Tracks    []*MusicTrack `bun:"rel:has-many,join:id=queue_id"`
}

func (g *GuildState) newQueue(ctx context.Context, channelID string) error {
	g.Queue = &Queue{
		GuildState: g,
		Logger:     g.Logger.Named("queue"),
		VC:         nil,
		Store:      g.Store,

		ID:        uuid.New().String(),
		GuildID:   g.ID,
		ChannelID: channelID,
		Tracks:    make([]*MusicTrack, 0),
	}

	_, err := g.Store.NewInsert().Model(g.Queue).Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "store queue")
	}

	return nil
}

func (q *Queue) Restore(ctx context.Context, g *GuildState) {
	logger := g.Logger.Named("queue")

	logger.Info("Restoring queue")

	q.GuildState = g
	q.Logger = logger
	q.Store = q.GuildState.Store

	for _, track := range q.Tracks {
		track.Restore(q)
	}

	if q.ChannelID != "" && len(q.Tracks) > 0 {
		go q.start(context.WithoutCancel(ctx))
	}
}

func (q *Queue) add(ctx context.Context, ms *MusicTrack, position int) (*MusicTrack, error) {
	retms := ms

	q.Lock()

	qlen := len(q.Tracks)
	position = q.choosePosition(position, qlen, ms)
	q.Tracks = slices.Insert(q.Tracks, position, ms)

	q.Unlock()

	if qlen == 0 { // a.k.a if there were no tracks in the queue before we added this one
		go q.start(context.WithoutCancel(ctx))

		return retms, nil
	} else if position == 0 {
		q.Tracks[0].Lock()
		err := q.Tracks[0].getStream()
		q.Tracks[0].Unlock()

		if err != nil {
			return nil, errorx.Decorate(err, "get stream")
		}

		// very, very bad scary things
		q.Tracks[0].Lock()
		q.Tracks[1].Lock()

		//goland:noinspection GoVetCopyLock
		*q.Tracks[0], *q.Tracks[1] = *q.Tracks[1], *q.Tracks[0] //nolint:govet
		q.Tracks[0], q.Tracks[1] = q.Tracks[1], q.Tracks[0]

		retms = q.Tracks[0]

		q.Tracks[0].Unlock()
		q.Tracks[1].Unlock()
	}

	// reaching here has the same condition as modifying ms.OrdKeys
	_, err := q.Store.NewUpdate().Model(retms).WherePK().Exec(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "store music track")
	}

	return retms, nil
}

func (q *Queue) start(ctx context.Context) {
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

	for {
		done := make(chan error, 1)

		q.Lock()

		if len(q.Tracks) > 0 {
			q.Unlock()

			break
		}

		curr := q.Tracks[0]

		q.Unlock()

		curr.streamToVC(ctx, done)

		err := <-done
		if err != nil {
			q.Logger.Errorf("Error when streaming track: %+v", err)
		}

		_, err = q.Store.NewDelete().Model(curr).WherePK().Exec(ctx)
		if err != nil {
			q.Logger.Errorf("Error deleting track from store: %+v", err)
		}

		// check before indexing because of stop call
		q.Lock()

		if len(q.Tracks) < 1 {
			q.Unlock()

			break
		}

		q.Tracks = q.Tracks[1:]

		q.Unlock()
	}
}

// choosePosition chooses position and sets track ordkey based on queue length and desired position.
// we have to very carefully sanitize position because slices.Insert can panic.
func (q *Queue) choosePosition(position, qlen int, ms *MusicTrack) int {
	// if there are no other tracks in the queue - insert on position 0
	if qlen == 0 {
		return 0
	}

	if position < 0 {
		// if position is negative, interpret that as index from end (wrap around)
		// e.g. -1 means put track as last, -2 means put track as before last, etc.
		position = qlen + position + 1
	}

	// if position is after last track - put as last
	if position >= qlen {
		ms.OrdKey = q.Tracks[qlen-1].OrdKey + 1

		return qlen
	}

	// if position is at or below 0, put the track as the first track
	if position <= 0 {
		ms.OrdKey = q.Tracks[0].OrdKey - 1

		return 0
	}

	// if position is somewhere between first and last, put in-between position and position - 1
	ms.OrdKey = utils.Mean(q.Tracks[position-1].OrdKey, q.Tracks[position].OrdKey)

	return position
}
