package models

import (
	"context"
	"errors"
	"io"
	"slices"
	"sync"
	"time"

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

	// -- non-stored values
	sync.Mutex `bun:"-" exhaustruct:"optional"`
	GuildState *GuildState                `bun:"-"`
	Logger     *zap.SugaredLogger         `bun:"-"`
	VC         *discordgo.VoiceConnection `bun:"-"`
	Store      *store.Store               `bun:"-"`
	// stop channel will stop current track playback upon receiving signal
	stop chan struct{} `bun:"-"`
	// Playing channel has to have a message for track to be playing
	// consume message from Playing to pause the track - add it back to resume it
	Playing chan struct{} `bun:"-"`
	// -- end non-stored values

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
		stop:       make(chan struct{}, 1),
		Playing:    make(chan struct{}, 1),

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
	q.stop = make(chan struct{}, 1)
	q.Playing = make(chan struct{}, 1)

	for _, track := range q.Tracks {
		track.Restore(q)
	}

	if q.ChannelID != "" && len(q.Tracks) > 0 {
		go q.start(context.WithoutCancel(ctx))
	}
}

func (q *Queue) Stop() {
	if len(q.stop) > 0 { // stop already queued
		return
	}
	q.stop <- struct{}{}
}

func (q *Queue) add(ctx context.Context, ms *MusicTrack, position int) error {
	q.Lock()

	qlen := len(q.Tracks)
	position = q.choosePosition(position, qlen, ms)
	q.Tracks = slices.Insert(q.Tracks, position, ms)

	q.Unlock()

	if qlen == 0 { // a.k.a if there were no tracks in the queue before we added this one
		go q.start(context.WithoutCancel(ctx))

		return nil
	}

	// reaching here has the same condition as modifying ms.OrdKeys
	_, err := q.Store.NewUpdate().Model(ms).WherePK().Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "store music track")
	}

	return nil
}

func (q *Queue) start(ctx context.Context) {
	q.Logger.Info("Starting playback")
	defer q.Logger.Info("Finished playback")

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

	storeLoopDone := make(chan struct{}, 1)
	go q.storeLoop(ctx, storeLoopDone)

	defer func() {
		storeLoopDone <- struct{}{}
	}()

	for {
		done := make(chan error, 1)

		if len(q.Tracks) < 1 {
			break
		}

		q.streamToVC(ctx, done)

		err := <-done
		if err != nil {
			q.Logger.Errorf("Error when streaming track: %+v", err)
		}

		q.Lock()

		// check before indexing because of stop call
		if len(q.Tracks) < 1 {
			q.Unlock()

			break
		}

		q.Tracks[0].cleanup()

		_, err = q.Store.NewDelete().Model(q.Tracks[0]).WherePK().Exec(ctx)
		if err != nil {
			q.Logger.Errorf("Error deleting track from store: %+v", err)
		}

		q.Tracks = q.Tracks[1:]

		q.Unlock()
	}
}

func (q *Queue) storeLoop(ctx context.Context, done chan struct{}) {
	ticker := time.NewTicker(storeInterval)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			q.Lock()

			_, err := q.Store.NewUpdate().Model(q.Tracks[0]).WherePK().Exec(ctx)
			if err != nil {
				q.Logger.Errorf("Error storing current track: %+v", err)
			}

			q.Unlock()
		}
	}
}

// streamToVC initializes first track in the queue and then starts stream loop.
func (q *Queue) streamToVC(ctx context.Context, done chan error) {
	q.Logger.Info("Streaming to VC")
	defer q.Logger.Info("Finished streaming to VC")

	defer close(done)

	if len(q.Tracks) < 1 {
		return
	}

	// start the track if not playing
	if len(q.Playing) < 1 {
		q.Playing <- struct{}{}
	}

	err := q.streamLoop(ctx)
	if err != nil {
		done <- errorx.Decorate(err, "stream")
	}
}

func (q *Queue) streamLoop(ctx context.Context) error {
	var err error

	var ms *MusicTrack

	packet := make([]byte, packetSize)

	for {
		q.Lock()

		if len(q.Tracks) < 1 {
			return nil
		}

		ms = q.Tracks[0]

		q.Unlock()

		err = ms.getPacket(ctx, packet)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return errorx.Decorate(err, "get packet")
		}

		// consume into itself to check that it has a value
		// if the value is missing - wait for it to be present
		q.Playing <- <-q.Playing

		select {
		case <-q.stop:
			return nil
		case q.VC.OpusSend <- packet:
		}

		if !ms.Live {
			ms.Pos += frameSizeMs * time.Millisecond
		}
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
