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

const stopLen = 10 // most skip requests that can be buffered

type Queue struct {
	bun.BaseModel `bun:"table:queues" exhaustruct:"optional"`

	// -- private non-stored values
	guildState *GuildState
	logger     *zap.SugaredLogger
	vc         *discordgo.VoiceConnection
	store      *store.Store
	// when playback is paused - queue will wait on resume channel to resume
	resume chan struct{}
	// stop channel will stop current track playback upon receiving signal
	stop chan struct{}
	// mutexes for specific values
	vcMu     sync.RWMutex `exhaustruct:"optional"`
	pausedMu sync.RWMutex `exhaustruct:"optional"`
	loopMu   sync.RWMutex `exhaustruct:"optional"`
	tracksMu sync.RWMutex `exhaustruct:"optional"`
	// -- end private non-stored values

	ID        string `bun:",pk"`
	GuildID   string
	ChannelID string
	Paused    bool
	Loop      bool
	Tracks    []*MusicTrack `bun:"rel:has-many,join:id=queue_id"`
}

func (g *GuildState) newQueue(ctx context.Context, channelID string) error {
	g.Queue = &Queue{
		guildState: g,
		logger:     g.logger.Named("queue"),
		vc:         nil,
		store:      g.store,
		resume:     make(chan struct{}, 1),
		stop:       make(chan struct{}, stopLen),

		ID:        uuid.New().String(),
		GuildID:   g.ID,
		ChannelID: channelID,
		Paused:    false,
		Loop:      false,
		Tracks:    make([]*MusicTrack, 0),
	}

	_, err := g.store.NewInsert().Model(g.Queue).Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "store queue")
	}

	return nil
}

func (q *Queue) Restore(ctx context.Context, g *GuildState) {
	logger := g.logger.Named("queue")

	logger.Info("Restoring queue")

	q.guildState = g
	q.logger = logger
	q.store = q.guildState.store
	q.resume = make(chan struct{}, 1)
	q.stop = make(chan struct{}, stopLen)

	for _, track := range q.Tracks {
		track.Restore(q)
	}

	if q.ChannelID != "" && len(q.Tracks) > 0 {
		go q.start(context.WithoutCancel(ctx))
	}
}

func (q *Queue) Stop() {
	if len(q.stop) == cap(q.stop) { // too many stops already queued
		return
	}

	q.stop <- struct{}{}
}

func (q *Queue) Pause() {
	if q.GetPaused() {
		return
	}

	if len(q.resume) > 0 {
		// make sure this never blocks
		select {
		case <-q.resume:
		default:
		}
	}

	q.SetPaused(true)
}

func (q *Queue) Resume() {
	if !q.GetPaused() || len(q.resume) > 0 {
		return
	}

	q.resume <- struct{}{}

	q.SetPaused(false)
}

func (q *Queue) Shuffle(ctx context.Context) {
	if len(q.GetTracks()) < 3 { //nolint:gomnd // 1 playing track and at least 2 to actually shuffle
		return
	}

	q.tracksMu.Lock()
	// shuffle all tracks but the currently playing
	rand.Shuffle(len(q.Tracks)-1, func(i, j int) {
		q.Tracks[i+1], q.Tracks[j+1] = q.Tracks[j+1], q.Tracks[i+1]
	})
	q.tracksMu.Unlock()

	q.tracksMu.RLock()
	baseOrd := q.Tracks[0].GetOrdKey() + 1
	updTracks := q.Tracks[1:]
	q.tracksMu.RUnlock()

	for i, track := range updTracks {
		track.SetOrdKey(baseOrd + float64(i))
	}

	for _, track := range updTracks {
		track.ordKeyMu.RLock()
	}

	_, err := q.store.NewUpdate().Model(&updTracks).Column("ord_key").Bulk().Exec(ctx)

	for _, track := range updTracks {
		track.ordKeyMu.RUnlock()
	}

	if err != nil {
		q.logger.Errorf("Error updaing tracks ordkeys: %+v", err)
	}
}

func (q *Queue) FlipLoop(ctx context.Context) error {
	q.loopMu.Lock()
	q.Loop = !q.Loop
	q.loopMu.Unlock()

	q.loopMu.RLock()
	_, err := q.store.NewUpdate().Model(q).Column("loop").WherePK().Exec(ctx)
	q.loopMu.RUnlock()

	if err != nil {
		return errorx.Decorate(err, "store looping state")
	}

	return nil
}

func (q *Queue) add(ctx context.Context, ms *MusicTrack, position int) error {
	q.tracksMu.Lock()

	qlen := len(q.Tracks)
	position = q.choosePosition(position, qlen, ms)
	q.Tracks = slices.Insert(q.Tracks, position, ms)

	q.tracksMu.Unlock()

	if qlen == 0 { // a.k.a if there were no tracks in the queue before we added this one
		q.SetPaused(false) // unpause the queue in case it was paused

		go q.start(context.WithoutCancel(ctx))

		return nil
	}

	// reaching here has the same condition as modifying ms.OrdKey
	ms.ordKeyMu.RLock()
	_, err := q.store.NewUpdate().Model(ms).Column("ord_key").WherePK().Exec(ctx)
	ms.ordKeyMu.RUnlock()

	if err != nil {
		return errorx.Decorate(err, "store music track")
	}

	return nil
}

func (q *Queue) start(ctx context.Context) {
	q.logger.Info("Starting playback")
	defer q.logger.Info("Finished playback")

	vc, err := q.guildState.botState.session.ChannelVoiceJoin(q.guildState.GuildID, q.ChannelID, false, true)
	if err != nil {
		q.logger.Errorf("Error joining voice channel: %+v", err)

		return
	}

	q.setVc(vc)

	defer func(vc *discordgo.VoiceConnection) {
		err := vc.Disconnect()
		if err != nil {
			q.logger.Errorf("Error leaving voice channel: %+v", err)
		}
	}(vc)

	storeLoopDone := make(chan struct{}, 1)
	go q.storeLoop(ctx, storeLoopDone)

	defer func() {
		storeLoopDone <- struct{}{}
	}()

	for {
		if len(q.GetTracks()) < 1 {
			break
		}

		err = q.streamToVC(ctx)
		if err != nil {
			q.logger.Errorf("Error when streaming track: %+v", err)
		}

		q.afterStream(ctx)
	}
}

func (q *Queue) afterStream(ctx context.Context) {
	q.tracksMu.Lock()
	defer q.tracksMu.Unlock()

	if len(q.Tracks) < 1 {
		return
	}

	// cleanup track resources
	q.Tracks[0].cleanup()

	if q.GetLoop() {
		// choosePosition updates track's ordKey
		_ = q.choosePosition(-1, len(q.Tracks), q.Tracks[0])

		q.Tracks[0].posMu.RLock()
		q.Tracks[0].ordKeyMu.RLock()
		_, err := q.store.NewUpdate().Model(q.Tracks[0]).Column("pos", "ord_key").WherePK().Exec(ctx)
		q.Tracks[0].posMu.RUnlock()
		q.Tracks[0].ordKeyMu.RUnlock()

		if err != nil {
			q.logger.Errorf("Error saving position of track in loop: %+v", err)
		}

		// move track to last
		q.Tracks = append(q.Tracks[1:], q.Tracks[0])
	} else {
		_, err := q.store.NewDelete().Model(q.Tracks[0]).WherePK().Exec(ctx)
		if err != nil {
			q.logger.Errorf("Error deleting track from store: %+v", err)
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
			q.tracksMu.RLock()

			if len(q.Tracks) < 1 {
				q.tracksMu.RUnlock()

				continue
			}

			track := q.Tracks[0]
			q.tracksMu.RUnlock()

			track.posMu.RLock()
			_, err := q.store.NewUpdate().Model(track).Column("pos").WherePK().Exec(ctx)
			track.posMu.RUnlock()

			if err != nil {
				q.logger.Errorf("Error storing current track: %+v", err)
			}
		}
	}
}

func (q *Queue) getPacket(ctx context.Context, packet []byte) (*MusicTrack, error) {
	q.tracksMu.RLock()

	if len(q.Tracks) < 1 {
		q.tracksMu.RUnlock()

		return nil, io.EOF
	}

	ms := q.Tracks[0]

	q.tracksMu.RUnlock()

	return ms, ms.getPacket(ctx, packet)
}

func (q *Queue) streamToVC(ctx context.Context) error {
	q.logger.Info("Streaming to VC")
	defer q.logger.Info("Finished streaming to VC")

	packet := make([]byte, packetSize)

	for {
		ms, err := q.getPacket(ctx, packet)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return errorx.Decorate(err, "get packet")
		}

		if q.GetPaused() {
			select {
			case <-q.stop:
				return nil
			case <-q.resume:
			}
		}

		select {
		case <-q.stop:
			return nil
		case q.getVc().OpusSend <- packet:
		}

		if !ms.Live {
			ms.SetPos(ms.Pos + frameSizeMs*time.Millisecond)
		}
	}
}

// choosePosition chooses position and sets track ordkey based on queue length and desired position.
// we have to very carefully sanitize position because slices.Insert can panic.
// !!! lock q.Tracks from caller function.
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
		ms.SetOrdKey(q.Tracks[qlen-1].GetOrdKey() + 1)

		return qlen
	}

	// if position is at or below 0, put the track as the first track
	if position <= 0 {
		ms.SetOrdKey(q.Tracks[0].GetOrdKey() - 1)

		return 0
	}

	// if position is somewhere between first and last, put in-between position and position - 1
	ms.SetOrdKey(utils.Mean(q.Tracks[position-1].GetOrdKey(), q.Tracks[position].GetOrdKey()))

	return position
}
