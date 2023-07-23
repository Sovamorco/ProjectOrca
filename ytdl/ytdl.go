package ytdl

import (
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"time"

	"ProjectOrca/extractor"
	"ProjectOrca/utils"

	"github.com/joomcode/errorx"
	"go.uber.org/zap"
)

type TrackData struct {
	Title       string            `json:"title"`
	OriginalURL string            `json:"original_url"`
	URL         string            `json:"url"`
	IsLive      bool              `json:"is_live"`
	Duration    float64           `json:"duration"`
	HTTPHeaders map[string]string `json:"http_headers"`
}

type SearchData struct {
	TrackData
	Entries []TrackData `json:"entries"`
}

type YTDL struct {
	logger *zap.SugaredLogger
}

func New(logger *zap.SugaredLogger) *YTDL {
	return &YTDL{
		logger: logger.Named("ytdl"),
	}
}

func (y *YTDL) QueryMatches(_ context.Context, q string) bool {
	// matches everything except for spotify
	// maybe should add more exceptions but whatever
	return !utils.SpotifyRx.MatchString(q)
}

func (y *YTDL) ExtractTracksData(_ context.Context, query string) ([]extractor.TrackData, error) {
	if !utils.URLRx.MatchString(query) {
		query = "ytsearch1:" + query
	}

	ytd, err := y.getTracksData(query)
	if err != nil {
		return nil, errorx.Decorate(err, "get ytdl tracks data")
	}

	res := make([]extractor.TrackData, len(ytd))

	for i, datum := range ytd {
		originalURL := datum.URL
		if datum.OriginalURL != "" {
			originalURL = datum.OriginalURL
		}

		res[i] = extractor.TrackData{
			Title:         datum.Title,
			ExtractionURL: originalURL,
			DisplayURL:    originalURL,
			StreamURL:     "",
			Live:          datum.IsLive,
			Duration:      time.Duration(datum.Duration * float64(time.Second)),
			HTTPHeaders:   datum.HTTPHeaders,
		}
	}

	return res, nil
}

func (y *YTDL) ExtractionURLMatches(_ context.Context, extURL string) bool {
	return strings.HasPrefix(extURL, "ytsearch") || utils.URLRx.MatchString(extURL)
}

func (y *YTDL) ExtractStreamURL(_ context.Context, extURL string) (string, error) {
	urlB, err := y.getYTDLPOutput("--get-url", extURL)
	if err != nil {
		return "", errorx.Decorate(err, "get stream url")
	}

	return strings.TrimSpace(string(urlB)), nil
}

func (y *YTDL) getTracksData(query string) ([]TrackData, error) {
	jsonB, err := y.getYTDLPOutput("--flat-playlist", "-J", query)
	if err != nil {
		return nil, errorx.Decorate(err, "get ytdlp output")
	}

	var vd SearchData
	err = json.Unmarshal(jsonB, &vd)

	if err != nil {
		return nil, errorx.Decorate(err, "unmarshal ytdl output")
	}

	var ad []TrackData

	if vd.Entries == nil {
		ad = []TrackData{vd.TrackData}
	} else {
		if len(vd.Entries) < 1 {
			return nil, extractor.ErrNoResults
		}
		ad = vd.Entries
	}

	return ad, nil
}

func (y *YTDL) getYTDLPOutput(args ...string) ([]byte, error) {
	ytdlpArgs := []string{
		"--format-sort-force",
		"--format-sort", "+hasvid,proto,asr~48000,acodec:opus",
		"-f", "ba*",
	}
	ytdlpArgs = append(ytdlpArgs, args...)

	ytdlp := exec.Command("yt-dlp", ytdlpArgs...)

	y.logger.Debug(ytdlp)

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