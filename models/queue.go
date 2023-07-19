package models

import (
	"context"
	"errors"
	"io"
	"math/rand"
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
	sync.RWMutex `bun:"-" exhaustruct:"optional"`
	GuildState   *GuildState                `bun:"-"`
	Logger       *zap.SugaredLogger         `bun:"-"`
	VC           *discordgo.VoiceConnection `bun:"-"`
	Store        *store.Store               `bun:"-"`
	// when playback is paused - queue will wait on resume channel to resume
	resume chan struct{} `bun:"-"`
	// stop channel will stop current track playback upon receiving signal
	stop chan struct{} `bun:"-"`
	// -- end non-stored values

	ID        string `bun:",pk"`
	GuildID   string
	ChannelID string
	Paused    bool
	Loop      bool
	Tracks    []*MusicTrack `bun:"rel:has-many,join:id=queue_id"`
}

func (g *GuildState) newQueue(ctx context.Context, channelID string) error {
	g.Queue = &Queue{
		GuildState: g,
		Logger:     g.Logger.Named("queue"),
		VC:         nil,
		Store:      g.Store,
		resume:     make(chan struct{}, 1),
		stop:       make(chan struct{}, 1),

		ID:        uuid.New().String(),
		GuildID:   g.ID,
		ChannelID: channelID,
		Paused:    false,
		Loop:      false,
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
	q.resume = make(chan struct{}, 1)
	q.stop = make(chan struct{}, 1)

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

func (q *Queue) Pause() {
	if q.Paused {
		return
	}

	if len(q.resume) > 0 {
		select {
		case <-q.resume:
		default:
		}
	}

	q.Paused = true
}

func (q *Queue) Resume() {
	if !q.Paused || len(q.resume) > 0 {
		return
	}

	q.resume <- struct{}{}
	q.Paused = false
}

func (q *Queue) Shuffle(ctx context.Context) {
	if len(q.Tracks) < 3 { //nolint:gomnd // 1 playing track and at least 2 to actually shuffle
		return
	}

	q.Lock()
	// shuffle all tracks but the currently playing
	rand.Shuffle(len(q.Tracks)-1, func(i, j int) {
		q.Tracks[i+1], q.Tracks[j+1] = q.Tracks[j+1], q.Tracks[i+1]
	})

	baseOrd := q.Tracks[0].OrdKey + 1

	for i, track := range q.Tracks[1:] {
		track.Lock()
		track.OrdKey = baseOrd + float64(i)
		track.Unlock()

		_, err := q.Store.NewUpdate().Model(track).WherePK().Exec(ctx)
		if err != nil {
			q.Logger.Errorf("Error updaing track ordkey: %+v", err)
		}
	}

	q.Unlock()
}

func (q *Queue) add(ctx context.Context, ms *MusicTrack, position int) error {
	q.Lock()

	qlen := len(q.Tracks)
	position = q.choosePosition(position, qlen, ms)
	q.Tracks = slices.Insert(q.Tracks, position, ms)

	q.Unlock()

	if qlen == 0 { // a.k.a if there were no tracks in the queue before we added this one
		q.Paused = false // unpause the queue in case it was paused

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
	// if position is somewhere between first and last, put in-between position and position - 1

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
		if len(q.Tracks) < 1 {
			break
		}

		err = q.streamToVC(ctx)
		if err != nil {
			q.Logger.Errorf("Error when streaming track: %+v", err)
		}

		q.afterStream(ctx)
	}
}

func (q *Queue) afterStream(ctx context.Context) {
	q.Lock()
	defer q.Unlock()

	if len(q.Tracks) < 1 {
		return
	}

	// cleanup track resources
	q.Tracks[0].Lock()
	q.Tracks[0].cleanup()
	q.Tracks[0].Unlock()

	if q.Loop {
		// choosePosition updates track's ordKey
		_ = q.choosePosition(-1, len(q.Tracks), q.Tracks[0])

		_, err := q.Store.NewUpdate().Model(q.Tracks[0]).WherePK().Exec(ctx)
		if err != nil {
			q.Logger.Errorf("Error saving position of track in loop: %+v", err)
		}

		// move track to last
		q.Tracks = append(q.Tracks[1:], q.Tracks[0])
	} else {
		_, err := q.Store.NewDelete().Model(q.Tracks[0]).WherePK().Exec(ctx)
		if err != nil {
			q.Logger.Errorf("Error deleting track from store: %+v", err)
		}

		q.Tracks = q.Tracks[1:]
	}
}

func (q *Queue) storeLoop(ctx context.Context, done chan struct{}) {
	ticker := time.NewTicker(storeInterval)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			q.RLock()

			_, err := q.Store.NewUpdate().Model(q.Tracks[0]).WherePK().Exec(ctx)
			if err != nil {
				q.Logger.Errorf("Error storing current track: %+v", err)
			}

			q.RUnlock()
		}
	}
}

func (q *Queue) getPacket(ctx context.Context, packet []byte) (*MusicTrack, error) {
	q.RLock()
	defer q.RUnlock()

	if len(q.Tracks) < 1 {
		return nil, io.EOF
	}

	ms := q.Tracks[0]

	return ms, ms.getPacket(ctx, packet)
}

func (q *Queue) streamToVC(ctx context.Context) error {
	q.Logger.Info("Streaming to VC")
	defer q.Logger.Info("Finished streaming to VC")

	packet := make([]byte, packetSize)

	for {
		ms, err := q.getPacket(ctx, packet)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return errorx.Decorate(err, "get packet")
		}

		if q.Paused {
			select {
			case <-q.stop:
				return nil
			case <-q.resume:
			}
		}

		select {
		case <-q.stop:
			return nil
		case q.VC.OpusSend <- packet:
		}

		if !ms.Live {
			ms.Lock()
			ms.Pos += frameSizeMs * time.Millisecond
			ms.Unlock()
		}
	}
}

// choosePosition chooses position and sets track ordkey based on queue length and desired position.
// we have to very carefully sanitize position because slices.Insert can panic.
func (q *Queue) choosePosition(position, qlen int, ms *MusicTrack) int {
	ms.Lock()
	defer ms.Unlock()

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
