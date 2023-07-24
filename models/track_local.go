package models

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"ProjectOrca/extractor"

	"ProjectOrca/store"
	"github.com/joomcode/errorx"
	"go.uber.org/zap"
)

const (
	cmdWaitTimeout = 5 * time.Second
)

var (
	ErrCMDStuck = errors.New("command stuck")
)

type LocalTrack struct {
	// constant values
	logger     *zap.SugaredLogger
	store      *store.Store
	extractors *extractor.Extractors

	// potentially changeable lockable values
	remote   *RemoteTrack
	remoteMu sync.RWMutex `exhaustruct:"optional"`
	cmd      *exec.Cmd
	cmdMu    sync.RWMutex `exhaustruct:"optional"`
	stream   io.ReadCloser
	streamMu sync.RWMutex `exhaustruct:"optional"`
	// pos is duplicated here to leave remote concurrency-safe
	pos   time.Duration
	posMu sync.RWMutex `exhaustruct:"optional"`

	// seek channel, weird, but I didn't find better ideas on how to make seeking consistent
	seek chan time.Duration
}

func NewLocalTrack(logger *zap.SugaredLogger, store *store.Store, extractors *extractor.Extractors) *LocalTrack {
	return &LocalTrack{
		logger:     logger.Named("track"),
		store:      store,
		extractors: extractors,
		remote:     nil,
		cmd:        nil,
		stream:     nil,
		pos:        0,

		seek: make(chan time.Duration, 1),
	}
}

func (t *LocalTrack) initialized() bool {
	return t.getRemote() != nil && t.getCMD() != nil && t.getStream() != nil
}

func (t *LocalTrack) initialize(ctx context.Context) error {
	if t.initialized() {
		return nil
	}

	remote := t.getRemote()

	if remote.StreamURL == "" {
		err := t.setStreamURL(ctx, remote)
		if err != nil {
			return errorx.Decorate(err, "get stream url")
		}
	}

	err := t.startStream(remote)
	if err != nil {
		return errorx.Decorate(err, "get stream")
	}

	return nil
}

func (t *LocalTrack) setStreamURL(ctx context.Context, remote *RemoteTrack) error {
	url, dur, err := t.extractors.ExtractStreamURL(ctx, remote.ExtractionURL)
	if err != nil {
		return errorx.Decorate(err, "extract stream url")
	}

	remote.StreamURL = url
	remote.Duration = dur

	_, err = t.store.NewUpdate().Model(remote).Column("stream_url", "duration").WherePK().Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "store url")
	}

	return nil
}

func (t *LocalTrack) startStream(remote *RemoteTrack) error {
	ffmpegArgs := []string{
		"-headers", remote.getFormattedHeaders(),
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "2",
	}

	if !remote.Live {
		ffmpegArgs = append(ffmpegArgs,
			"-reconnect_at_eof", "1",
			"-ss", fmt.Sprintf("%f", t.getPos().Seconds()),
		)
	}

	ffmpegArgs = append(ffmpegArgs,
		"-i", remote.StreamURL,
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

	t.logger.Debug(ffmpeg)

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

	t.setCMD(ffmpeg)
	t.setStream(buf)

	return nil
}

func (t *LocalTrack) cleanup() {
	if cmd := t.getCMD(); cmd != nil {
		err := cmd.Process.Kill()
		if err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.logger.Errorf("Error killing current process: %+v", err)
		}
	}

	if stream := t.getStream(); stream != nil {
		err := stream.Close()
		if err != nil && !errors.Is(err, os.ErrClosed) {
			t.logger.Errorf("Error closing current stream: %+v", err)
		}
	}

	t.setCMD(nil)
	t.setStream(nil)
	t.setPos(0) // resets local track position proxy, NOT remote track position
}

func (t *LocalTrack) getPacket(packet []byte) error {
	n, err := io.ReadFull(t.getStream(), packet)
	if err != nil {
		if errors.Is(err, io.EOF) {
			// additionally wait for ffmpeg to exit because we need exit code
			err := t.waitForCMDExit()
			if err != nil {
				return errorx.Decorate(err, "wait for ffmpeg")
			}
		}

		if !errors.Is(err, io.ErrUnexpectedEOF) {
			return errorx.Decorate(err, "read audio Stream")
		}

		// fill rest of the packet with zeros
		for i := n; i < len(packet); i++ {
			packet[i] = 0
		}
	}

	return nil
}

func (t *LocalTrack) waitForCMDExit() error {
	ec := make(chan error, 1)

	go func() {
		ec <- t.getCMD().Wait()
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
	err := t.getCMD().Process.Kill()
	if err != nil {
		return errorx.Decorate(err, "kill cmd")
	}

	return ErrCMDStuck
}
