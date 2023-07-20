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
	URL         string            `json:"url"`
	IsLive      bool              `json:"is_live"`
	Duration    float64           `json:"duration"`
	HTTPHeaders map[string]string `json:"http_headers"`
}

type YTDLSearchData struct {
	YTDLData
	Entries []YTDLData `json:"entries"`
}

type MusicTrack struct {
	bun.BaseModel `bun:"table:tracks" exhaustruct:"optional"`

	// -- non-stored values
	queue       *Queue
	logger      *zap.SugaredLogger
	cmd         *exec.Cmd
	stream      io.ReadCloser
	store       *store.Store
	initialized bool
	// mutexes for specific values
	cmdMu         sync.RWMutex `exhaustruct:"optional"`
	streamMu      sync.RWMutex `exhaustruct:"optional"`
	initializedMu sync.RWMutex `exhaustruct:"optional"`
	posMu         sync.RWMutex `exhaustruct:"optional"`
	ordKeyMu      sync.RWMutex `exhaustruct:"optional"`
	urlMu         sync.RWMutex `exhaustruct:"optional"`
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

func (q *Queue) newMusicTracks(ctx context.Context, url string) ([]*MusicTrack, error) {
	tracks, err := q.getTracksData(url)
	if err != nil {
		return nil, errorx.Decorate(err, "get tracks data")
	}

	res := make([]*MusicTrack, len(tracks))

	for i, trackData := range tracks {
		res[i] = q.newMusicTrackEmpty()
		res[i].fill(trackData)
	}

	_, err = q.store.NewInsert().Model(&res).Exec(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "store tracks")
	}

	return res, nil
}

func (q *Queue) newMusicTrackEmpty() *MusicTrack {
	return &MusicTrack{
		queue:       q,
		logger:      q.logger.Named("track"),
		cmd:         nil,
		stream:      nil,
		store:       q.store,
		initialized: false,
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

func (ms *MusicTrack) fill(data YTDLData) {
	ms.Title = data.Title
	ms.HTTPHeaders = data.HTTPHeaders
	ms.Live = data.IsLive
	ms.Duration = time.Duration(data.Duration * float64(time.Second))
	ms.logger = ms.logger.With("track", ms.Title)

	ms.OriginalURL = data.URL
	if data.OriginalURL != "" {
		ms.OriginalURL = data.OriginalURL
	}
}

func (q *Queue) getTracksData(url string) ([]YTDLData, error) {
	jsonB, err := getYTDLPOutput(q.logger, "--flat-playlist", "-J", url)
	if err != nil {
		return nil, errorx.Decorate(err, "get ytdlp output")
	}

	var vd YTDLSearchData
	err = json.Unmarshal(jsonB, &vd)

	if err != nil {
		return nil, errorx.Decorate(err, "unmarshal ytdl output")
	}

	var ad []YTDLData

	if vd.Entries == nil {
		ad = []YTDLData{vd.YTDLData}
	} else {
		if len(vd.Entries) < 1 {
			return nil, ErrNoResults
		}
		ad = vd.Entries
	}

	return ad, nil
}

func (ms *MusicTrack) Restore(q *Queue) {
	logger := q.logger.Named("track").With("track", ms.Title)

	logger.Info("Restoring track")

	ms.queue = q
	ms.logger = logger
	ms.store = ms.queue.store
}

func (ms *MusicTrack) ToProto() *pb.TrackData {
	return &pb.TrackData{
		Title:       ms.Title,
		OriginalURL: ms.OriginalURL,
		Url:         ms.GetURL(),
		Live:        ms.Live,
		Position:    durationpb.New(ms.GetPos()),
		Duration:    durationpb.New(ms.Duration),
	}
}

func (ms *MusicTrack) Seek(pos time.Duration) {
	ms.cleanup()

	ms.SetPos(pos)
}

func (ms *MusicTrack) Initialize(ctx context.Context) error {
	err := ms.getStreamURL(ctx)
	if err != nil {
		if ms.GetURL() == "" {
			return errorx.Decorate(err, "get stream url")
		}

		ms.logger.Errorf("Error getting new stream URL, hoping old one works: %+v", err)
	}

	err = ms.startStream()
	if err != nil {
		return errorx.Decorate(err, "get stream")
	}

	ms.setInitialized(true)

	return nil
}

func (ms *MusicTrack) getPacket(ctx context.Context, packet []byte) error {
	var err error

	if !ms.getInitialized() {
		err = ms.Initialize(ctx)
		if err != nil {
			return errorx.Decorate(err, "initialize")
		}
	}

	n, err := io.ReadFull(ms.getStream(), packet)
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

func (ms *MusicTrack) getStreamURL(ctx context.Context) error {
	urlB, err := getYTDLPOutput(ms.logger, "--get-url", ms.OriginalURL)
	if err != nil {
		return errorx.Decorate(err, "get stream url")
	}

	ms.SetURL(strings.TrimSpace(string(urlB)))

	ms.urlMu.RLock()
	_, err = ms.store.NewUpdate().Model(ms).Column("url").WherePK().Exec(ctx)
	ms.urlMu.RUnlock()

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

// startStream gets a stream for current song on specific position.
func (ms *MusicTrack) startStream() error {
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

	ms.logger.Debug(ffmpeg)

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

	ms.setCMD(ffmpeg)
	ms.setStream(buf)

	return nil
}

// cleanup cleans up resources used by track, namely stream and process.
func (ms *MusicTrack) cleanup() {
	if stream := ms.getStream(); stream != nil {
		err := stream.Close()
		if err != nil && !errors.Is(err, os.ErrClosed) {
			ms.logger.Errorf("Error closing Stream: %+v", err)
		}
	}

	ms.setStream(nil)

	if cmd := ms.getCMD(); cmd != nil {
		err := cmd.Process.Signal(syscall.SIGTERM)
		if err != nil {
			ms.logger.Errorf("Error killing ffmpeg process: %+v", err)
		}
	}

	ms.setCMD(nil)
	ms.SetPos(0)
	ms.setInitialized(false)
}
