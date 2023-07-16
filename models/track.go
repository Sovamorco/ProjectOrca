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

	"ProjectOrca/store"
	"ProjectOrca/utils"

	pb "ProjectOrca/proto"

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

	bufferPackets = 10

	storeInterval = 5 * time.Second
)

var ErrNoResults = errors.New("no search results")

type YTDLData struct {
	ID          string            `json:"id"`
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
	bun.BaseModel `bun:"table:tracks"`

	sync.Mutex `bun:"-"`          // do not store mutex state
	Queue      *Queue             `bun:"-"` // do not store parent queue
	Logger     *zap.SugaredLogger `bun:"-"` // do not store logger
	CMD        *exec.Cmd          `bun:"-"` // do not store cmd
	Stream     io.ReadCloser      `bun:"-"` // do not store stream
	Stop       chan struct{}      `bun:"-"` // do not store channel
	Store      *store.Store       `bun:"-"` // do not store the store

	ID        string `bun:",pk"`
	QueueID   string
	Pos       time.Duration
	TrackData *pb.TrackData `bun:"type:json"`
	OrdKey    float64
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
	return &MusicTrack{ //nolint:exhaustruct
		Logger: q.Logger.Named("track"),
		Stop:   make(chan struct{}),
		Store:  q.Store,

		ID:      uuid.New().String(),
		QueueID: q.ID,
		Queue:   q,
		Pos:     0,
	}
}

func (ms *MusicTrack) Restore(q *Queue) {
	logger := q.Logger.Named("track").With("track", ms.TrackData.Title)

	logger.Info("Restoring track")

	ms.Queue = q
	ms.Logger = logger
	ms.Stop = make(chan struct{})
	ms.Store = ms.Queue.Store
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

	ms.TrackData = ad.toProto()
	ms.Logger = ms.Logger.With("track", ms.TrackData.Title)

	return nil
}

func (ms *MusicTrack) getStreamURL(ctx context.Context) error {
	urlB, err := getYTDLPOutput(ms.Logger, "--get-url", ms.TrackData.OriginalURL)
	if err != nil {
		return errorx.Decorate(err, "get stream url")
	}

	ms.TrackData.Url = strings.TrimSpace(string(urlB))

	_, err = ms.Store.NewUpdate().Model(ms).WherePK().Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "store track")
	}

	return nil
}

func (ms *MusicTrack) getFormattedHeaders() string {
	fmtd := make([]string, 0, len(ms.TrackData.HttpHeaders))

	for k, v := range ms.TrackData.HttpHeaders {
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

	if !strings.Contains(ms.TrackData.Url, ".m3u8") {
		ffmpegArgs = append(ffmpegArgs,
			"-reconnect_at_eof", "1",
			"-ss", fmt.Sprintf("%f", ms.Pos.Seconds()),
		)
	}

	ffmpegArgs = append(ffmpegArgs,
		"-i", ms.TrackData.Url,
		"-map", "0:a",
		"-filter:a", "loudnorm",
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

func (ms *MusicTrack) storeLoop(ctx context.Context, done chan struct{}) {
	ticker := time.NewTicker(storeInterval)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			_, err := ms.Store.NewUpdate().Model(ms).WherePK().Exec(ctx)
			if err != nil {
				ms.Logger.Errorf("Error storing current track: %+v", err)
			}
		}
	}
}

func (ms *MusicTrack) streamToVC(ctx context.Context, done chan error) {
	ms.Logger.Info("Streaming to VC")
	defer ms.Logger.Info("Finished streaming to VC")

	defer close(done)

	ms.Lock()

	err := ms.getStreamURL(ctx)
	if err != nil {
		ms.Logger.Errorf("Error getting new stream URL, hoping old one works: %+v", err)
	}

	err = ms.getStream()

	ms.Unlock()

	if err != nil {
		done <- errorx.Decorate(err, "get Stream")

		return
	}

	defer ms.cleanup()

	storeLoopDone := make(chan struct{}, 1)
	go ms.storeLoop(ctx, storeLoopDone)

	defer func() {
		storeLoopDone <- struct{}{}
	}()

	err = ms.streamLoop()
	if err != nil {
		done <- errorx.Decorate(err, "stream")
	}
}

func (ms *MusicTrack) streamLoop() error {
	var n int

	var err error

	packet := make([]byte, packetSize)

	for {
		ms.Lock()
		n, err = io.ReadFull(ms.Stream, packet)
		ms.Unlock()

		if err != nil {
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				{
					if !errors.Is(err, io.EOF) {
						return errorx.Decorate(err, "read audio Stream")
					}

					return nil
				}
			}

			// fill rest of the packet with zeros
			for i := n; i < len(packet); i++ {
				packet[i] = 0
			}
		}

		select {
		case <-ms.Stop:
			utils.Empty(ms.Stop)

			return nil
		case ms.Queue.VC.OpusSend <- packet:
		}

		ms.Pos += frameSizeMs * time.Millisecond
	}
}

func (ms *MusicTrack) cleanup() {
	err := ms.Stream.Close()
	if err != nil && !errors.Is(err, os.ErrClosed) {
		ms.Logger.Errorf("Error closing Stream: %+v", err)
	}

	err = ms.CMD.Process.Signal(syscall.SIGTERM)
	if err != nil {
		ms.Logger.Errorf("Error killing ffmpeg process: %+v", err)
	}
}
