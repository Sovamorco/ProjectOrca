package models

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"ProjectOrca/store"
	"ProjectOrca/utils"

	"github.com/joomcode/errorx"
	"go.uber.org/zap"
)

type LocalTrack struct {
	// constant values
	logger *zap.SugaredLogger
	store  *store.Store

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
}

func NewLocalTrack(logger *zap.SugaredLogger, store *store.Store) *LocalTrack {
	return &LocalTrack{
		logger: logger.Named("track"),
		store:  store,
		remote: nil,
		cmd:    nil,
		stream: nil,
		pos:    0,
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

	err := t.setStreamURL(ctx, remote)
	if err != nil {
		if remote.URL == "" {
			return errorx.Decorate(err, "get stream url")
		}

		t.logger.Errorf("Error getting new stream URL, hoping old one works: %+v", err)
	}

	err = t.startStream(remote)
	if err != nil {
		return errorx.Decorate(err, "get stream")
	}

	return nil
}

func (t *LocalTrack) setStreamURL(ctx context.Context, remote *RemoteTrack) error {
	urlB, err := utils.GetYTDLPOutput(t.logger, "--get-url", remote.OriginalURL)
	if err != nil {
		return errorx.Decorate(err, "get stream url")
	}

	remote.URL = strings.TrimSpace(string(urlB))

	_, err = t.store.NewUpdate().Model(remote).Column("url").WherePK().Exec(ctx)
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
		"-i", remote.URL,
		"-map", "0:a",
		// "-filter:a", "dynaudnorm=p=0.9:r=0.5", // makes metal sound dogshit :( TODO: revisit
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
		if err != nil {
			t.logger.Errorf("Error killing current process: %+v", err)
		}
	}

	if stream := t.getStream(); stream != nil {
		err := stream.Close()
		if err != nil {
			t.logger.Errorf("Error closing current stream: %+v", err)
		}
	}

	t.setCMD(nil)
	t.setStream(nil)
	t.setPos(0)
}

func (t *LocalTrack) getPacket(packet []byte) error {
	n, err := io.ReadFull(t.getStream(), packet)
	if err != nil {
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
