package ytdl

import (
	"bytes"
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
	"github.com/rs/zerolog"
)

//nolint:gochecknoglobals // this is really a constant, but it's not possible to declare it as such.
var ytdlpUnsupportedWarning = []byte("The program functionality for this site has been marked as broken, " +
	"and will probably not work")

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
	cookiesFile string
}

func New(cookiesFile string) *YTDL {
	return &YTDL{
		cookiesFile: cookiesFile,
	}
}

func (y *YTDL) QueryMatches(context.Context, string) bool {
	// matches everything, use as last extractor
	return true
}

func (y *YTDL) ExtractTracksData(ctx context.Context, query string) ([]extractor.TrackData, error) {
	return y.ExtractTracksDataHeaders(ctx, query, nil)
}

func (y *YTDL) ExtractTracksDataHeaders(
	ctx context.Context, query string, headers map[string]string,
) ([]extractor.TrackData, error) {
	if !utils.URLRx.MatchString(query) {
		query = "ytsearch1:" + query
	}

	ytd, err := y.getTracksData(ctx, query, headers)
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

func (y *YTDL) ExtractStreamURL(ctx context.Context, extURL string) (string, time.Duration, error) {
	return y.ExtractStreamURLHeaders(ctx, extURL, nil)
}

func (y *YTDL) ExtractStreamURLHeaders(
	ctx context.Context, extURL string, headers map[string]string,
) (string, time.Duration, error) {
	//nolint:mnd // flag and value for flag.
	args := make([]string, 0, len(headers)*2)
	for k, v := range headers {
		args = append(args, "--add-headers", k+":"+v)
	}

	args = append(args, "-I", "1:1", "-O", "url,duration", extURL)

	urlB, err := y.getYTDLPOutput(ctx, args...)
	if err != nil {
		return "", 0, errorx.Decorate(err, "get stream url")
	}

	spl := strings.Split(string(urlB), "\n")
	if len(spl) < 2 { //nolint:mnd // 2 lines used later
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

func (y *YTDL) getTracksData(ctx context.Context, query string, headers map[string]string) ([]TrackData, error) {
	//nolint:mnd // flag and value for flag.
	args := make([]string, 0, len(headers)*2)
	for k, v := range headers {
		args = append(args, "--add-headers", k+":"+v)
	}

	args = append(args, "--flat-playlist", "-J", query)

	jsonB, err := y.getYTDLPOutput(ctx, args...)
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

func (y *YTDL) getYTDLPOutput(ctx context.Context, args ...string) ([]byte, error) {
	logger := zerolog.Ctx(ctx)

	ytdlpArgs := []string{
		"--format-sort-force",
		"--format-sort", "+hasvid,proto,asr~48000,acodec:opus",
		"-f", "ba*",
	}

	if y.cookiesFile != "" {
		ytdlpArgs = append(ytdlpArgs, "--cookies", y.cookiesFile)
	}

	ytdlpArgs = append(ytdlpArgs, args...)

	ytdlp := exec.CommandContext(ctx, "yt-dlp", ytdlpArgs...)

	logger.Debug().Msg(ytdlp.String())

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
		if bytes.Contains(errlog, ytdlpUnsupportedWarning) {
			return nil, extractor.ErrNoExtractor
		}

		logger.Error().Bytes("errlog", errlog).Msg("YTDLP stderr")

		return nil, errorx.Decorate(err, "wait for ytdlp")
	}

	return jsonB, nil
}
