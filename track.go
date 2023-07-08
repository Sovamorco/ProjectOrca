package main

import (
	"ProjectOrca/opus"
	pb "ProjectOrca/proto"
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

	smoothVolumeStepPercentPerSecond = 50.

	smoothVolumeStep = smoothVolumeStepPercentPerSecond / 100 / (1000 / frameSizeMs)
)

type YTDLData struct {
	Id          string            `json:"id"`
	Title       string            `json:"title"`
	Channel     string            `json:"channel"`
	OriginalURL string            `json:"original_url"`
	URL         string            `json:"url"`
	HTTPHeaders map[string]string `json:"http_headers"`
}

type YTDLSearchData struct {
	YTDLData
	Entries []YTDLData `json:"entries"`
}

func (td *YTDLData) toProto() *pb.TrackData {
	return &pb.TrackData{
		Title:       td.Title,
		OriginalURL: td.OriginalURL,
		Url:         td.URL,
		HttpHeaders: td.HTTPHeaders,
	}
}

type MusicTrack struct {
	sync.Mutex
	*Queue
	CMD       *exec.Cmd
	Stream    io.ReadCloser
	Stop      chan struct{}
	TrackData *pb.TrackData
}

func (q *Queue) newMusicTrack(url string) (*MusicTrack, error) {
	ms := q.newMusicTrackEmpty()
	err := ms.getStreamURL(url)
	return ms, errors.Wrap(err, "get Stream url")
}

func (q *Queue) newMusicTrackEmpty() *MusicTrack {
	return &MusicTrack{
		Queue: q,
		Stop:  make(chan struct{}),
	}
}

func (ms *MusicTrack) seek(pos time.Duration) error {
	oldCmd, oldStream := ms.CMD, ms.Stream
	err := ms.getStream(pos)
	if err != nil {
		return errors.Wrap(err, "get Stream")
	}
	err = oldStream.Close()
	if err != nil {
		ms.Logger.Error("Error closing old Stream: ", err)
	}
	err = oldCmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		ms.Logger.Error("Error killing old ffmpeg: ", err)
	}
	return nil
}

func getYTDLPOutput(logger *zap.SugaredLogger, url string) ([]byte, error) {
	ytdlpArgs := []string{
		"--format-sort-force",
		"--format-sort", "+hasvid,proto,asr~48000,acodec:opus",
		"-f", "ba*",
		"-J",
		url,
	}
	ytdlp := exec.Command("yt-dlp", ytdlpArgs...)
	logger.Debug(ytdlp)
	stdout, err := ytdlp.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "get ytdlp stdout pipe")
	}
	err = ytdlp.Start()
	if err != nil {
		return nil, errors.Wrap(err, "start ytdlp")
	}
	jsonB, err := io.ReadAll(stdout)
	if err != nil {
		return nil, errors.Wrap(err, "read Stream url")
	}
	err = ytdlp.Wait()
	if err != nil {
		return nil, errors.Wrap(err, "wait for ytdlp")
	}
	return jsonB, nil
}

func (ms *MusicTrack) getStreamURL(url string) error {
	jsonB, err := getYTDLPOutput(ms.Logger, url)
	if err != nil {
		return errors.Wrap(err, "get ytdlp output")
	}
	var ad YTDLData
	vd := YTDLSearchData{}
	err = json.Unmarshal(jsonB, &vd)
	if err != nil {
		return errors.Wrap(err, "unmarshal ytdl output")
	}
	if vd.Entries == nil {
		ad = vd.YTDLData
	} else {
		if len(vd.Entries) < 1 {
			return errors.New("no search results")
		}
		ad = vd.Entries[0]
	}
	ms.TrackData = ad.toProto()
	return nil
}

func (ms *MusicTrack) getFormattedHeaders() string {
	fmtd := make([]string, len(ms.TrackData.HttpHeaders))
	for k, v := range ms.TrackData.HttpHeaders {
		fmtd = append(fmtd, fmt.Sprintf("%s:%s", k, v))
	}
	return strings.Join(fmtd, "\r\n")
}

func (ms *MusicTrack) getStream(pos time.Duration) error {
	ffmpegArgs := []string{
		"-headers", ms.getFormattedHeaders(),
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "2",
	}
	if !strings.Contains(ms.TrackData.Url, ".m3u8") {
		ffmpegArgs = append(ffmpegArgs,
			"-reconnect_at_eof", "1",
			"-ss", fmt.Sprintf("%f", pos.Seconds()),
		)
	}
	ffmpegArgs = append(ffmpegArgs,
		"-i", ms.TrackData.Url,
		"-filter:a", "loudnorm",
		"-f", "f32be",
		"-ar", fmt.Sprintf("%d", sampleRate),
		"-ac", fmt.Sprintf("%d", channels),
		"pipe:1",
	)
	ffmpeg := exec.Command("ffmpeg", ffmpegArgs...)
	ms.Logger.Debug(ffmpeg)
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
	err = ffmpeg.Start()
	if err != nil {
		return errors.Wrap(err, "start ffmpeg process")
	}
	ms.Lock()
	ms.CMD = ffmpeg
	ms.Stream = buf
	ms.Unlock()
	return nil
}

func (ms *MusicTrack) streamToVC(vc *discordgo.VoiceConnection, done chan error) {
	defer close(done)
	err := ms.getStream(0)
	if err != nil {
		done <- errors.Wrap(err, "get Stream")
		return
	}
	defer func(stream io.ReadCloser) {
		err := stream.Close()
		if err != nil && !errors.Is(err, os.ErrClosed) {
			ms.Logger.Error("Error closing Stream: ", err)
		}
	}(ms.Stream)
	defer func(Process *os.Process) {
		err := Process.Signal(syscall.SIGTERM)
		if err != nil {
			ms.Logger.Error("Error killing ffmpeg process: ", err)
		}
	}(ms.CMD.Process)
	pcmFrame := make([]float32, frameSize)
	enc, err := opus.NewEncoder(sampleRate, channels, opus.Audio)
	if err != nil {
		done <- errors.Wrap(err, "create opus encoder")
		return
	}
	for {
		ms.Lock()
		err = binary.Read(ms.Stream, binary.BigEndian, pcmFrame)
		ms.Unlock()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				done <- errors.Wrap(err, "read audio Stream")
			}
			return
		}
		// change Volume by at most smoothVolumeStep towards the TargetVolume every frame
		if ms.Volume != ms.TargetVolume {
			ms.Volume += atMostAbs(ms.TargetVolume-ms.Volume, smoothVolumeStep)
		}
		for i := range pcmFrame {
			pcmFrame[i] *= ms.Volume
		}
		packet, err := enc.EncodeFloat32(pcmFrame, frameSize, bufferSize)
		if err != nil {
			done <- errors.Wrap(err, "encode pcm to opus")
			return
		}
		select {
		case <-ms.Stop:
			empty(ms.Stop)
			return
		case vc.OpusSend <- packet:
		}
	}
}
