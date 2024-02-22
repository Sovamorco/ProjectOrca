package ytdl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"

	"ProjectOrca/extractor"
	"ProjectOrca/utils"

	"github.com/joomcode/errorx"
	"go.uber.org/zap"
)

var ErrInvalidOutput = errors.New("yt-dlp produced invalid output")

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

func (y *YTDL) QueryMatches(context.Context, string) bool {
	// matches everything, use as last extractor
	return true
}

func (y *YTDL) ExtractTracksData(_ context.Context, query string) ([]extractor.TrackData, error) {
	if !utils.URLRx.MatchString(query) {
		query = "ytsearch1:" + query
	}

	ytd, err := y.getTracksData(query)
	if err != nil {
		return nil, errorx.Decorate(err, "get ytdl tracks data")
	}

	res := make([]extractor.TrackData, 0, len(ytd))

	for _, datum := range ytd {
		if datum.Duration == 0 {
			continue
		}

		originalURL := datum.URL
		if datum.OriginalURL != "" {
			originalURL = datum.OriginalURL
		}

		res = append(res, extractor.TrackData{
			Title:         datum.Title,
			ExtractionURL: originalURL,
			DisplayURL:    originalURL,
			StreamURL:     "",
			Live:          datum.IsLive,
			Duration:      time.Duration(datum.Duration * float64(time.Second)),
			HTTPHeaders:   datum.HTTPHeaders,
		})
	}

	if len(res) == 0 {
		return nil, extractor.ErrNoResults
	}

	return res, nil
}

func (y *YTDL) ExtractionURLMatches(_ context.Context, extURL string) bool {
	return strings.HasPrefix(extURL, "ytsearch") || utils.URLRx.MatchString(extURL)
}

func (y *YTDL) ExtractStreamURL(_ context.Context, extURL string) (string, time.Duration, error) {
	urlB, err := y.getYTDLPOutput("-I", "1:1", "-O", "url,duration", extURL)
	if err != nil {
		return "", 0, errorx.Decorate(err, "get stream url")
	}

	spl := strings.Split(string(urlB), "\n")
	if len(spl) < 2 { //nolint:gomnd // 2 lines used later
		return "", 0, ErrInvalidOutput
	}

	url := spl[0]
	durationS := spl[1]

	duration, err := time.ParseDuration(durationS + "s")
	if err != nil {
		return "", 0, errorx.Decorate(err, "parse duration")
	}

	return url, duration, nil
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

	stderr, err := ytdlp.StderrPipe()
	if err != nil {
		return nil, errorx.Decorate(err, "get ytdlp stderr pipe")
	}

	err = ytdlp.Start()
	if err != nil {
		return nil, errorx.Decorate(err, "start ytdlp")
	}

	jsonB, err := io.ReadAll(stdout)
	if err != nil {
		return nil, errorx.Decorate(err, "read ytdlp output")
	}

	errlog, err := io.ReadAll(stderr)
	if err != nil {
		return nil, errorx.Decorate(err, "read ytdlp error output")
	}

	err = ytdlp.Wait()
	if err != nil {
		y.logger.Errorf("YTDLP stderr:\n%s", string(errlog))

		return nil, errorx.Decorate(err, "wait for ytdlp")
	}

	return jsonB, nil
}
