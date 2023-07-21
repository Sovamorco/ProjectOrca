package models

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"ProjectOrca/store"
	"ProjectOrca/utils"

	"github.com/joomcode/errorx"
	"go.uber.org/zap"
)

type LocalTrack struct {
	logger *zap.SugaredLogger
	store  *store.Store

	remote *RemoteTrack
	cmd    *exec.Cmd
	stream io.ReadCloser
}

func NewLocalTrack(logger *zap.SugaredLogger, store *store.Store) *LocalTrack {
	return &LocalTrack{
		logger: logger.Named("track"),
		store:  store,
		remote: nil,
		cmd:    nil,
		stream: nil,
	}
}

func (t *LocalTrack) initialized() bool {
	return t.remote != nil && t.cmd != nil && t.stream != nil
}

func (t *LocalTrack) initialize(ctx context.Context) error {
	if t.initialized() {
		return nil
	}

	err := t.setStreamURL(ctx)
	if err != nil {
		if t.remote.URL == "" {
			return errorx.Decorate(err, "get stream url")
		}

		t.logger.Errorf("Error getting new stream URL, hoping old one works: %+v", err)
	}

	err = t.startStream()
	if err != nil {
		return errorx.Decorate(err, "get stream")
	}

	return nil
}

func (t *LocalTrack) setStreamURL(ctx context.Context) error {
	urlB, err := utils.GetYTDLPOutput(t.logger, "--get-url", t.remote.OriginalURL)
	if err != nil {
		return errorx.Decorate(err, "get stream url")
	}

	t.remote.URL = strings.TrimSpace(string(urlB))

	_, err = t.store.NewUpdate().Model(t.remote).Column("url").WherePK().Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "store url")
	}

	return nil
}

func (t *LocalTrack) startStream() error {
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
		"-i", t.remote.URL,
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

	t.cmd = ffmpeg
	t.stream = buf

	return nil
}

func (t *LocalTrack) cleanup() {
	if t.cmd != nil {
		err := t.cmd.Process.Kill()
		if err != nil {
			t.logger.Errorf("Error killing current process: %+v", err)
		}

		t.cmd = nil
	}

	if t.stream != nil {
		err := t.stream.Close()
		if err != nil {
			t.logger.Errorf("Error closing current stream: %+v", err)
		}

		t.stream = nil
	}
}

func (t *LocalTrack) getPacket(packet []byte) error {
	n, err := io.ReadFull(t.stream, packet)
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
