package models

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"ProjectOrca/models/notifications"

	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
)

const (
	// after end of queue and before disconnecting wait for packetburst to hopefully be empty.
	endQueueDisconnectDelay = packetBurstNum * frameSizeMs * time.Millisecond

	// https://trac.ffmpeg.org/wiki/AudioVolume
	// global volume factor for all streams.
	volume = "0.5"
)

// Track is basically a wrapper for values commonly passed together - remote track, command, stream, packet.
// It is not concurrency-safe, should be used within one goroutine.
type Track struct {
	g *Guild

	remote *RemoteTrack
	cmd    *exec.Cmd
	stream io.ReadCloser

	packetChan chan []byte
}

func NewTrack(ctx context.Context, guild *Guild) *Track {
	t := &Track{
		g: guild,

		remote: nil,
		cmd:    nil,
		stream: nil,

		packetChan: make(chan []byte, packetBurstNum),
	}

	go t.sendLoop(ctx)

	return t
}

func (t *Track) initialized() bool {
	return t.remote != nil && t.cmd != nil && t.stream != nil
}

func (t *Track) initialize(ctx context.Context) error {
	if t.initialized() {
		return nil
	}

	if t.remote.StreamURL == "" {
		err := t.setStreamURL(ctx)
		if err != nil {
			return errorx.Decorate(err, "get stream url")
		}
	}

	err := t.startStream(ctx)
	if err != nil {
		return errorx.Decorate(err, "get stream")
	}

	return nil
}

func (t *Track) setStreamURL(ctx context.Context) error {
	url, dur, err := t.g.extractors.ExtractStreamURL(ctx, t.remote.ExtractionURL)
	if err != nil {
		return errorx.Decorate(err, "extract stream url")
	}

	t.remote.StreamURL = url
	t.remote.Duration = dur

	_, err = t.g.store.NewUpdate().Model(t.remote).Column("stream_url", "duration").WherePK().Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "store url")
	}

	return nil
}

func (t *Track) startStream(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)

	ffmpegArgs := []string{
		"-headers", t.remote.getFormattedHeaders(),
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "2",
	}

	if !t.remote.Live {
		ffmpegArgs = append(ffmpegArgs,
			"-reconnect_at_eof", "1",
			"-ss", fmt.Sprintf("%f", t.remote.Pos.Seconds()),
		)
	}

	ffmpegArgs = append(ffmpegArgs,
		"-i", t.remote.StreamURL,
		"-map", "0:a",
		"-filter:a", "dynaudnorm=p=0.9:r=0.9,volume="+volume,
		"-acodec", "libopus",
		"-f", "data",
		"-ar", strconv.Itoa(sampleRate),
		"-ac", strconv.Itoa(channels),
		"-b:a", strconv.Itoa(bitrate),
		"-vbr", "off",
		"pipe:1",
	)

	ffmpeg := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)

	logger.Debug().Msg(ffmpeg.String())

	stdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		return errorx.Decorate(err, "get ffmpeg stdout pipe")
	}

	// make a.. BufferedReadCloser I guess?
	buf := struct {
		io.Reader
		io.Closer
	}{
		bufio.NewReaderSize(stdout, packetSize*bufferPackets),
		stdout,
	}

	err = ffmpeg.Start()
	if err != nil {
		return errorx.Decorate(err, "start ffmpeg process")
	}

	t.cmd = ffmpeg
	t.stream = buf

	return nil
}

func (t *Track) clean(ctx context.Context) {
	logger := zerolog.Ctx(ctx)

	if t.cmd != nil {
		err := t.cmd.Process.Kill()
		if err != nil && !errors.Is(err, os.ErrProcessDone) {
			logger.Error().Err(err).Msg("Error killing current process")
		}
	}

	if t.stream != nil {
		err := t.stream.Close()
		if err != nil && !errors.Is(err, os.ErrClosed) {
			logger.Error().Err(err).Msg("Error closing current stream")
		}
	}

	t.cmd = nil
	t.remote = nil
	t.stream = nil
}

func (t *Track) sendPacketBurst(ctx context.Context) error {
	for i := 0; i < packetBurstNum; i++ {
		packet := make([]byte, packetSize)

		n, err := io.ReadFull(t.stream, packet)
		if err != nil {
			if errors.Is(err, io.EOF) {
				// additionally wait for ffmpeg to exit because we need exit code
				err := t.waitForCMDExit()
				if err != nil {
					return errorx.Decorate(err, "wait for ffmpeg")
				}
			}

			if !errors.Is(err, io.ErrUnexpectedEOF) {
				return errorx.Decorate(err, "read audio stream")
			}

			// fill rest of the packet with zeros
			for i := n; i < len(packet); i++ {
				packet[i] = 0
			}
		}

		select {
		case <-ctx.Done():
			return context.Canceled
		case t.packetChan <- packet:
		}

		t.incrementPos()
	}

	return nil
}

func (t *Track) waitForCMDExit() error {
	ec := make(chan error, 1)

	go func() {
		ec <- t.cmd.Wait()
	}()

	select {
	case err := <-ec:
		if err != nil {
			return errorx.Decorate(err, "wait for cmd exit")
		}

		return nil
	case <-time.After(cmdWaitTimeout):
	}

	// waiting timed out
	err := t.cmd.Process.Kill()
	if err != nil {
		return errorx.Decorate(err, "kill cmd")
	}

	return ErrCMDStuck
}

func (t *Track) stop(ctx context.Context) error {
	old := t.remote

	t.clean(ctx)

	err := old.DeleteOrRequeue(ctx, t.g.store)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return errorx.Decorate(err, "delete or requeue track")
	}

	go notifications.SendQueueNotificationLog(ctx, t.g.store, t.g.botID, t.g.id)

	return nil
}

func (t *Track) checkForNextTrack(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)

	if t.remote != nil {
		return nil
	}

	next, err := t.g.getNextTrack(ctx)
	if err == nil {
		t.remote = next

		return nil
	}

	if !errors.Is(err, ErrEmptyQueue) {
		return errorx.Decorate(err, "get track from store")
	}

	time.Sleep(endQueueDisconnectDelay)

	err = t.g.connect(ctx, "")
	if err != nil {
		logger.Error().Err(err).Msg("Error leaving voice channel")
	}

	// queue is empty right now
	// wait for signal on resyncPlaying channel instead of polling database
	select {
	case <-ctx.Done():
		return context.Canceled
	case <-t.g.resyncPlaying:
		go notifications.SendQueueNotificationLog(ctx, t.g.store, t.g.botID, t.g.id)
	}

	return ErrNoTrack
}

func (t *Track) nextTrackPrecondition(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)

	if t.initialized() {
		return nil
	}

	err := t.checkForNextTrack(ctx)

	if errors.Is(err, context.Canceled) || errors.Is(err, ErrNoTrack) {
		return err
	} else if err != nil {
		logger.Error().Err(err).Msg("Error checking for next track")

		return ErrNoTrack
	}

	err = t.initialize(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Error initializing track")

		_, err = t.g.store.NewDelete().Model(t.remote).WherePK().Exec(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("Error deleting broken track")
		}

		t.clean(ctx)

		return ErrNoTrack
	}

	return nil
}

func (t *Track) packetPrecondition(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)

	err := t.sendPacketBurst(ctx)
	if err == nil {
		return nil
	}

	if errors.Is(err, context.Canceled) {
		return err
	}

	if errors.Is(err, io.EOF) {
		ierr := t.stop(ctx)
		if ierr != nil {
			return errorx.Decorate(ierr, "stop track")
		}

		return nil
	}

	_, rerr := t.g.store.
		NewUpdate().
		Model(t.remote).
		Set("stream_url = ?", "").
		WherePK().
		Exec(ctx)
	if rerr != nil {
		logger.Error().Err(rerr).Msg("Error resetting stream url")
	}

	t.clean(ctx)

	return errorx.Decorate(err, "send packet burst")
}

func (t *Track) incrementPos() {
	if !t.initialized() {
		return
	}

	t.remote.Pos += frameSizeMs * time.Millisecond
	compensation := frameSizeMs * time.Millisecond * time.Duration(len(t.packetChan)) // compensate for packet buffer

	compensation = min(compensation, t.remote.Pos) // compensation cannot be more than position itself

	// make sure this does not block
	select {
	case t.g.posChan <- posMsg{pos: t.remote.Pos - compensation, id: t.remote.ID}:
	default:
	}
}

func (t *Track) sendLoop(ctx context.Context) {
	var packet []byte

	for {
		select {
		case <-ctx.Done():
			return
		case packet = <-t.packetChan:
		}

		t.g.vcMu.RLock()

		if t.g.vc == nil || !t.g.vc.Ready {
			t.g.vcMu.RUnlock()

			continue
		}

		select {
		case t.g.vc.OpusSend <- packet:
		case <-time.After(opusSendTimeout):
		}

		t.g.vcMu.RUnlock()
	}
}
