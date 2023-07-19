package models

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	pb "ProjectOrca/proto"
	"ProjectOrca/store"

	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

const (
	sampleRate  = 48000
	channels    = 2
	frameSizeMs = 20
	bitrate     = 64000 // bits/s
	packetSize  = bitrate * frameSizeMs / 1000 / 8

	bufferMilliseconds = 500
	bufferPackets      = bufferMilliseconds / frameSizeMs

	storeInterval = 5 * time.Second
)

var ErrNoResults = errors.New("no search results")

type YTDLData struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Channel     string            `json:"channel"`
	OriginalURL string            `json:"original_url"`
	IsLive      bool              `json:"is_live"`
	Duration    float64           `json:"duration"`
	URL         string            `json:"url"`
	HTTPHeaders map[string]string `json:"http_headers"`
}

type YTDLSearchData struct {
	YTDLData
	Entries []YTDLData `json:"entries"`
}

type MusicTrack struct {
	bun.BaseModel `bun:"table:tracks" exhaustruct:"optional"`

	// -- non-stored values
	sync.RWMutex `bun:"-" exhaustruct:"optional"`
	Queue        *Queue             `bun:"-"`
	Logger       *zap.SugaredLogger `bun:"-"`
	CMD          *exec.Cmd          `bun:"-"`
	Stream       io.ReadCloser      `bun:"-"`
	Store        *store.Store       `bun:"-"`
	Initialized  bool               `bun:"-"`
	// -- end non-stored values

	ID          string `bun:",pk"`
	QueueID     string
	Pos         time.Duration
	Duration    time.Duration
	OrdKey      float64
	Title       string
	OriginalURL string
	URL         string
	HTTPHeaders map[string]string
	Live        bool
}

func (q *Queue) newMusicTrack(ctx context.Context, url string) (*MusicTrack, error) {
	ms := q.newMusicTrackEmpty()

	err := ms.getTrackData(url)
	if err != nil {
		return nil, errorx.Decorate(err, "get stream url")
	}

	_, err = q.Store.NewInsert().Model(ms).Exec(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "store music track")
	}

	return ms, nil
}

func (q *Queue) newMusicTrackEmpty() *MusicTrack {
	return &MusicTrack{
		Queue:       q,
		Logger:      q.Logger.Named("track"),
		CMD:         nil,
		Stream:      nil,
		Store:       q.Store,
		Initialized: false,
		ID:          uuid.New().String(),
		QueueID:     q.ID,
		Pos:         0,
		Duration:    0,
		OrdKey:      0,
		Title:       "",
		OriginalURL: "",
		URL:         "",
		HTTPHeaders: nil,
		Live:        false,
	}
}

func (ms *MusicTrack) Restore(q *Queue) {
	logger := q.Logger.Named("track").With("track", ms.Title)

	logger.Info("Restoring track")

	ms.Queue = q
	ms.Logger = logger
	ms.Store = ms.Queue.Store
}

func (ms *MusicTrack) ToProto() *pb.TrackData {
	return &pb.TrackData{
		Title:       ms.Title,
		OriginalURL: ms.OriginalURL,
		Url:         ms.URL,
		Live:        ms.Live,
		Position:    durationpb.New(ms.Pos),
		Duration:    durationpb.New(ms.Duration),
	}
}

func (ms *MusicTrack) Seek(pos time.Duration) error {
	ms.Lock()

	oldCmd, oldStream := ms.CMD, ms.Stream
	ms.Pos = pos
	err := ms.getStream()

	ms.Unlock()

	if err != nil {
		return errorx.Decorate(err, "get Stream")
	}

	if oldStream != nil {
		err = oldStream.Close()
		if err != nil {
			ms.Logger.Errorf("Error closing old Stream: %+v", err)
		}
	}

	if oldCmd != nil {
		err = oldCmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			ms.Logger.Errorf("Error killing old ffmpeg: %+v", err)
		}
	}

	return nil
}

func (ms *MusicTrack) Initialize(ctx context.Context) error {
	ms.Lock()
	defer ms.Unlock()

	err := ms.getStreamURL(ctx)
	if err != nil {
		if ms.URL == "" {
			return errorx.Decorate(err, "get stream url")
		}

		ms.Logger.Errorf("Error getting new stream URL, hoping old one works: %+v", err)
	}

	err = ms.getStream()
	if err != nil {
		return errorx.Decorate(err, "get stream")
	}

	ms.Initialized = true

	return nil
}

func (ms *MusicTrack) getPacket(ctx context.Context, packet []byte) error {
	var err error

	if !ms.Initialized {
		err = ms.Initialize(ctx)
		if err != nil {
			return errorx.Decorate(err, "initialize")
		}
	}

	ms.RLock()

	n, err := io.ReadFull(ms.Stream, packet)

	ms.RUnlock()

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

func getYTDLPOutput(logger *zap.SugaredLogger, args ...string) ([]byte, error) {
	ytdlpArgs := []string{
		"--format-sort-force",
		"--format-sort", "+hasvid,proto,asr~48000,acodec:opus",
		"-f", "ba*",
	}
	ytdlpArgs = append(ytdlpArgs, args...)

	ytdlp := exec.Command("yt-dlp", ytdlpArgs...)

	logger.Debug(ytdlp)

	stdout, err := ytdlp.StdoutPipe()
	if err != nil {
		return nil, errorx.Decorate(err, "get ytdlp stdout pipe")
	}

	err = ytdlp.Start()
	if err != nil {
		return nil, errorx.Decorate(err, "start ytdlp")
	}

	jsonB, err := io.ReadAll(stdout)
	if err != nil {
		return nil, errorx.Decorate(err, "read Stream url")
	}

	err = ytdlp.Wait()
	if err != nil {
		return nil, errorx.Decorate(err, "wait for ytdlp")
	}

	return jsonB, nil
}

func (ms *MusicTrack) getTrackData(url string) error {
	jsonB, err := getYTDLPOutput(ms.Logger, "-J", url)
	if err != nil {
		return errorx.Decorate(err, "get ytdlp output")
	}

	var ad YTDLData

	var vd YTDLSearchData
	err = json.Unmarshal(jsonB, &vd)

	if err != nil {
		return errorx.Decorate(err, "unmarshal ytdl output")
	}

	if vd.Entries == nil {
		ad = vd.YTDLData
	} else {
		if len(vd.Entries) < 1 {
			return ErrNoResults
		}
		ad = vd.Entries[0]
	}

	ms.Title = ad.Title
	ms.OriginalURL = ad.OriginalURL
	ms.URL = ad.URL
	ms.HTTPHeaders = ad.HTTPHeaders
	ms.Live = ad.IsLive
	ms.Duration = time.Duration(ad.Duration * float64(time.Second))
	ms.Logger = ms.Logger.With("track", ms.Title)

	return nil
}

func (ms *MusicTrack) getStreamURL(ctx context.Context) error {
	urlB, err := getYTDLPOutput(ms.Logger, "--get-url", ms.OriginalURL)
	if err != nil {
		return errorx.Decorate(err, "get stream url")
	}

	ms.URL = strings.TrimSpace(string(urlB))

	_, err = ms.Store.NewUpdate().Model(ms).WherePK().Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "store track")
	}

	return nil
}

func (ms *MusicTrack) getFormattedHeaders() string {
	fmtd := make([]string, 0, len(ms.HTTPHeaders))

	for k, v := range ms.HTTPHeaders {
		fmtd = append(fmtd, fmt.Sprintf("%s:%s", k, v))
	}

	return strings.Join(fmtd, "\r\n")
}

// getStream gets a stream for current song on specific position
// it DOES NOT lock the track, please lock in the caller function.
func (ms *MusicTrack) getStream() error {
	ffmpegArgs := []string{
		"-headers", ms.getFormattedHeaders(),
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "2",
	}

	if !ms.Live {
		ffmpegArgs = append(ffmpegArgs,
			"-reconnect_at_eof", "1",
			"-ss", fmt.Sprintf("%f", ms.Pos.Seconds()),
		)
	}

	ffmpegArgs = append(ffmpegArgs,
		"-i", ms.URL,
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

	ms.Logger.Debug(ffmpeg)

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

	ms.CMD = ffmpeg
	ms.Stream = buf

	return nil
}

func (ms *MusicTrack) cleanup() {
	ms.Lock()

	err := ms.Stream.Close()
	if err != nil && !errors.Is(err, os.ErrClosed) {
		ms.Logger.Errorf("Error closing Stream: %+v", err)
	}

	err = ms.CMD.Process.Signal(syscall.SIGTERM)
	if err != nil {
		ms.Logger.Errorf("Error killing ffmpeg process: %+v", err)
	}

	ms.Unlock()
}
