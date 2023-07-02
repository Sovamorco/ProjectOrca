package main

import (
	"RaccoonBotMusic/opus"
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	sampleRate  = 48000
	channels    = 1
	frameSizeMs = 20
	frameSize   = channels * frameSizeMs * sampleRate / 1000
	bufferSize  = frameSize * 4
)

type VideoData struct {
	Id          string            `json:"id"`
	Title       string            `json:"title"`
	Channel     string            `json:"channel"`
	OriginalURL string            `json:"original_url"`
	URL         string            `json:"url"`
	HTTPHeaders map[string]string `json:"http_headers"`
}

type musicTrack struct {
	sync.Mutex
	logger    *zap.SugaredLogger
	cmd       *exec.Cmd
	stream    io.ReadCloser
	stop      chan struct{}
	volume    float32
	videoData *VideoData
}

var (
	currentms *musicTrack
)

func newMusicTrack(logger *zap.SugaredLogger, url string) (*musicTrack, error) {
	ms := musicTrack{
		logger: logger,
		stop:   make(chan struct{}),
		volume: 1,
	}
	err := ms.getStreamURL(url)
	return &ms, errors.Wrap(err, "get stream url")
}

func (ms *musicTrack) seek(pos time.Duration) error {
	oldcmd, oldstream := ms.cmd, ms.stream
	err := ms.getStream(pos)
	if err != nil {
		return errors.Wrap(err, "get stream")
	}
	err = oldstream.Close()
	if err != nil {
		ms.logger.Error("Error closing old stream: ", err)
	}
	err = oldcmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		ms.logger.Error("Error killing old ffmpeg: ", err)
	}
	return nil
}

func (ms *musicTrack) getStreamURL(url string) error {
	ytdlpArgs := []string{
		"--format-sort-force",
		"--format-sort", "+hasvid,proto,asr~48000,acodec:opus",
		"-f", "ba*",
		"-J",
		url,
	}
	ytdlp := exec.Command("yt-dlp", ytdlpArgs...)
	ms.logger.Debug(ytdlp)
	stdout, err := ytdlp.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "get ytdlp stdout pipe")
	}
	err = ytdlp.Start()
	if err != nil {
		return errors.Wrap(err, "start ytdlp")
	}
	jsonB, err := io.ReadAll(stdout)
	if err != nil {
		return errors.Wrap(err, "read stream url")
	}
	err = ytdlp.Wait()
	if err != nil {
		return errors.Wrap(err, "wait for ytdlp")
	}
	vd := VideoData{}
	err = json.Unmarshal(jsonB, &vd)
	if err != nil {
		return errors.Wrap(err, "unmarshal video data json")
	}
	ms.videoData = &vd
	return nil
}

func (ms *musicTrack) getFormattedHeaders() string {
	fmtd := make([]string, len(ms.videoData.HTTPHeaders))
	for k, v := range ms.videoData.HTTPHeaders {
		fmtd = append(fmtd, fmt.Sprintf("%s:%s", k, v))
	}
	return strings.Join(fmtd, "\r\n")
}

func (ms *musicTrack) getStream(pos time.Duration) error {
	ffmpegArgs := []string{
		"-headers", ms.getFormattedHeaders(),
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "2",
	}
	if !strings.Contains(ms.videoData.URL, ".m3u8") {
		ffmpegArgs = append(ffmpegArgs,
			"-reconnect_at_eof", "1",
			"-ss", fmt.Sprintf("%f", pos.Seconds()),
		)
	}
	ffmpegArgs = append(ffmpegArgs,
		"-i", ms.videoData.URL,
		"-f", "f32be",
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-ac", fmt.Sprintf("%d", channels),
		"pipe:1",
	)
	ffmpeg := exec.Command("ffmpeg", ffmpegArgs...)
	ms.logger.Debug(ffmpeg)
	stdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "get ffmpeg stdout pipe")
	}
	// make a.. BufferedReadCloser I guess?
	buf := struct {
		io.Reader
		io.Closer
	}{
		bufio.NewReaderSize(stdout, frameSize*5),
		stdout,
	}
	ms.Lock()
	ms.cmd = ffmpeg
	ms.stream = buf
	ms.Unlock()
	return nil
}

func (ms *musicTrack) streamToVC(vc *discordgo.VoiceConnection, done chan error) {
	defer close(done)
	err := ms.getStream(0)
	if err != nil {
		done <- errors.Wrap(err, "get stream")
		return
	}
	defer func(stream io.ReadCloser) {
		err := stream.Close()
		if err != nil && !errors.Is(err, os.ErrClosed) {
			ms.logger.Error("Error closing stream: ", err)
		}
	}(ms.stream)
	defer func(Process *os.Process) {
		err := Process.Signal(syscall.SIGTERM)
		if err != nil {
			ms.logger.Error("Error killing ffmpeg process: ", err)
		}
	}(ms.cmd.Process)
	pcmFrame := make([]float32, frameSize)
	enc, err := opus.NewEncoder(sampleRate, channels, opus.Audio)
	if err != nil {
		done <- errors.Wrap(err, "create opus encoder")
		return
	}
	for {
		ms.logger.Debug(pcmFrame)
		ms.Lock()
		err = binary.Read(ms.stream, binary.BigEndian, pcmFrame)
		ms.Unlock()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				done <- errors.Wrap(err, "read audio stream")
			}
			return
		}
		for i := range pcmFrame {
			pcmFrame[i] *= ms.volume
		}
		packet, err := enc.EncodeFloat32(pcmFrame, frameSize, bufferSize)
		if err != nil {
			done <- errors.Wrap(err, "encode pcm to opus")
			return
		}
		select {
		case <-ms.stop:
			empty(ms.stop)
			return
		case vc.OpusSend <- packet:
		}
	}
}
