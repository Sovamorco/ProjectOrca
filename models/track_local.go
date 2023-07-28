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
	"time"

	"github.com/joomcode/errorx"
)

// Track is basically a wrapper for values commonly passed together - remote track, command, stream, packet.
// It is not concurrency-safe, should be used within one goroutine.
type Track struct {
	g *Guild

	remote *RemoteTrack
	cmd    *exec.Cmd
	stream io.ReadCloser

	sendLoopDone chan struct{}
	packetChan   chan []byte
}

func NewTrack(guild *Guild) *Track {
	t := &Track{
		g: guild,

		remote: nil,
		cmd:    nil,
		stream: nil,

		sendLoopDone: make(chan struct{}, 1),
		packetChan:   make(chan []byte, packetBurstNum),
	}

	go t.sendLoop()

	return t
}

func (t *Track) shutdown() {
	// make sure this does not block
	select {
	case t.sendLoopDone <- struct{}{}:
	default:
	}
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

	err := t.startStream()
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

func (t *Track) startStream() error {
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
		"-filter:a", "dynaudnorm=p=0.9:r=0.9",
		"-acodec", "libopus",
		"-f", "data",
		"-ar", fmt.Sprint(sampleRate),
		"-ac", fmt.Sprint(channels),
		"-b:a", fmt.Sprint(bitrate),
		"-vbr", "off",
		"pipe:1",
	)

	ffmpeg := exec.Command("ffmpeg", ffmpegArgs...)

	t.g.logger.Debug(ffmpeg)

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

func (t *Track) clean() {
	if t.cmd != nil {
		err := t.cmd.Process.Kill()
		if err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.g.logger.Errorf("Error killing current process: %+v", err)
		}
	}

	if t.stream != nil {
		err := t.stream.Close()
		if err != nil && !errors.Is(err, os.ErrClosed) {
			t.g.logger.Errorf("Error closing current stream: %+v", err)
		}
	}

	t.cmd = nil
	t.remote = nil
	t.stream = nil
}

func (t *Track) sendPacketBurst() error {
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
		case <-t.g.playLoopDone:
			return ErrShuttingDown
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

	t.clean()

	err := old.DeleteOrRequeue(ctx, t.g.store)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return errorx.Decorate(err, "delete or requeue track")
	}

	return nil
}

func (t *Track) checkForNextTrack(ctx context.Context) error {
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

	err = t.g.connect(ctx, "")
	if err != nil {
		t.g.logger.Errorf("Error leaving voice channel: %+v", err)
	}

	// queue is empty right now
	// wait for signal on resyncPlaying channel instead of polling database
	select {
	case <-t.g.playLoopDone:
		return ErrShuttingDown
	case <-t.g.resyncPlaying:
	}

	return ErrNoTrack
}

func (t *Track) nextTrackPrecondition(ctx context.Context) error {
	if t.initialized() {
		return nil
	}

	err := t.checkForNextTrack(ctx)

	if errors.Is(err, ErrShuttingDown) || errors.Is(err, ErrNoTrack) {
		return err
	} else if err != nil {
		t.g.logger.Errorf("Error checking for next track: %+v", err)

		return ErrNoTrack
	}

	err = t.initialize(ctx)
	if err != nil {
		t.g.logger.Errorf("Error initializing track: %+v", err)

		_, err = t.g.store.NewDelete().Model(t.remote).WherePK().Exec(ctx)
		if err != nil {
			t.g.logger.Errorf("Error deleting broken track: %+v", err)
		}

		t.clean()

		return ErrNoTrack
	}

	return nil
}

func (t *Track) packetPrecondition(ctx context.Context) error {
	err := t.sendPacketBurst()
	if err == nil {
		return nil
	}

	if errors.Is(err, ErrShuttingDown) {
		return ErrShuttingDown
	}

	if errors.Is(err, io.EOF) {
		err = t.stop(ctx)
		if err != nil {
			t.g.logger.Errorf("Error stopping current track: %+v", err)
		}

		return io.EOF
	}

	t.g.logger.Errorf("Error getting packet from stream: %+v", err)

	_, err = t.g.store.
		NewUpdate().
		Model(t.remote).
		Set("stream_url = ?", "").
		WherePK().
		Exec(ctx)
	if err != nil {
		t.g.logger.Errorf("Error resetting stream url: %+v", err)
	}

	t.clean()

	return ErrBrokenStreamURL
}

func (t *Track) incrementPos() {
	if !t.initialized() {
		return
	}

	t.remote.Pos += frameSizeMs * time.Millisecond
	compensation := frameSizeMs * time.Millisecond * time.Duration(len(t.packetChan)) // compensate for packet buffer
	compensation = min(compensation, t.remote.Pos)                                    // compensation cannot be more than position itself

	// make sure this does not block
	select {
	case t.g.posChan <- posMsg{pos: t.remote.Pos - compensation, id: t.remote.ID}:
	default:
	}
}

func (t *Track) sendLoop() {
	var packet []byte

	for {
		select {
		case <-t.sendLoopDone:
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
