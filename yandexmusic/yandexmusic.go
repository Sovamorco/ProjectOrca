package yandexmusic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"ProjectOrca/extractor"
	"ProjectOrca/utils"

	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
)

var (
	ErrInvalidYMURL           = errors.New("invalid yandex music url")
	ErrCannotExtractStreamURL = errors.New("cannot use yandex music extractor to extract stream url")
)

type Track struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	DurationMS int    `json:"durationMs"`

	Artists []struct {
		Name string
	} `json:"artists"`
}

func (t Track) getTitle() string {
	artist := ""
	if len(t.Artists) > 0 {
		artist = t.Artists[0].Name + " - "
	}

	return artist + t.Title
}

func (t Track) toTrackData() extractor.TrackData {
	title := t.getTitle()

	return extractor.TrackData{
		Title:         title,
		ExtractionURL: getExtractionURL(title),
		DisplayURL:    getDisplayURL(t.ID),
		StreamURL:     "",
		Live:          false,
		Duration:      time.Duration(t.DurationMS) * time.Millisecond,
		HTTPHeaders:   nil,
	}
}

type YMResponse struct {
	Type string `json:"type"`
}

type SingleTrackData struct {
	Track Track `json:"track"`
}

type AlbumData struct {
	Volumes [][]Track `json:"volumes"`
}

type PlaylistData struct {
	Playlist struct {
		Tracks []Track `json:"tracks"`
	} `json:"playlist"`
}

type YandexMusic struct {
	baseURL url.URL
	client  *http.Client
}

func New() *YandexMusic {
	var client http.Client

	//nolint:exhaustruct
	baseURL := url.URL{
		Scheme: "https",
		Host:   "music.yandex.ru",
	}

	return &YandexMusic{
		baseURL: baseURL,
		client:  &client,
	}
}

func (s *YandexMusic) QueryMatches(_ context.Context, q string) bool {
	return utils.YandexMusicTrackRx.MatchString(q) ||
		utils.YandexMusicAlbumRx.MatchString(q) ||
		utils.YandexMusicPlaylistRx.MatchString(q)
}

func (s *YandexMusic) ExtractTracksData(ctx context.Context, url string) ([]extractor.TrackData, error) {
	if matches := utils.YandexMusicTrackRx.FindStringSubmatch(url); len(matches) >= 2 { //nolint:mnd // see regex
		return s.getTrackData(ctx, url, matches[1])
	} else if matches := utils.YandexMusicAlbumRx.FindStringSubmatch(url); len(matches) >= 2 { //nolint:mnd // see regex
		return s.getAlbumTracksData(ctx, url, matches[1])
	} else if matches := utils.YandexMusicPlaylistRx.FindStringSubmatch(url); len(matches) >= 3 { //nolint:mnd // see regex
		return s.getPlaylistTracksData(ctx, url, matches[1], matches[2])
	}

	return nil, ErrInvalidYMURL
}

func (s *YandexMusic) ExtractionURLMatches(_ context.Context, _ string) bool {
	// cannot use spotify for stream url extraction
	return false
}

func (s *YandexMusic) ExtractStreamURL(_ context.Context, _ string) (string, time.Duration, error) {
	return "", 0, ErrCannotExtractStreamURL
}

func (s *YandexMusic) getTrackData(ctx context.Context, url, id string) ([]extractor.TrackData, error) {
	track, err := doRequest[SingleTrackData](ctx, s.client, s.baseURL,
		"track",
		map[string]string{
			"track": id,
		}, url)
	if err != nil {
		return nil, errorx.Decorate(err, "do yandex music request")
	}

	return []extractor.TrackData{track.Track.toTrackData()}, nil
}

func (s *YandexMusic) getAlbumTracksData(ctx context.Context, url, id string) ([]extractor.TrackData, error) {
	data, err := doRequest[AlbumData](ctx, s.client, s.baseURL,
		"album",
		map[string]string{
			"album": id,
		}, url)
	if err != nil {
		return nil, errorx.Decorate(err, "do yandex music request")
	}

	tracksNum := 0
	for _, volume := range data.Volumes {
		tracksNum += len(volume)
	}

	tracksData := make([]extractor.TrackData, 0, tracksNum)

	for _, volume := range data.Volumes {
		for _, track := range volume {
			tracksData = append(tracksData, track.toTrackData())
		}
	}

	return tracksData, nil
}

func (s *YandexMusic) getPlaylistTracksData(ctx context.Context, url, owner, id string) ([]extractor.TrackData, error) {
	data, err := doRequest[PlaylistData](ctx,
		s.client, s.baseURL, "playlist",
		map[string]string{
			"owner": owner,
			"kinds": id,
		}, url)
	if err != nil {
		return nil, errorx.Decorate(err, "do yandex music request")
	}

	tracksData := make([]extractor.TrackData, 0, len(data.Playlist.Tracks))

	for _, track := range data.Playlist.Tracks {
		tracksData = append(tracksData, track.toTrackData())
	}

	return tracksData, nil
}

func doRequest[T any](
	ctx context.Context,
	client *http.Client, baseURL url.URL,
	handler string, params map[string]string, referrer string,
) (T, error) {
	logger := zerolog.Ctx(ctx)

	var data T

	req, err := createRequest(ctx, baseURL, handler, params, referrer)
	if err != nil {
		return data, errorx.Decorate(err, "create request")
	}

	res, err := client.Do(req)
	if err != nil {
		return data, errorx.Decorate(err, "send request")
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			logger.Error().Err(err).Msg("Error closing body")
		}
	}()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return data, errorx.Decorate(err, "read body")
	}

	var ymresponse YMResponse

	err = json.Unmarshal(body, &ymresponse)
	if err != nil {
		return data, errorx.Decorate(err, "unmarshal response")
	}

	if ymresponse.Type == "captcha" {
		return data, extractor.ErrCaptcha
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, errorx.Decorate(err, "decode response")
	}

	return data, nil
}

func createRequest(
	ctx context.Context, baseURL url.URL, handler string, params map[string]string, referrer string,
) (*http.Request, error) {
	baseURL.Path = "/handlers/" + handler + ".jsx"

	query := baseURL.Query()

	query.Add("light", "true")
	query.Add("lang", "ru")
	query.Add("external-domain", "music.yandex.ru")
	query.Add("overembed", "false")

	for k, v := range params {
		query.Add(k, v)
	}

	baseURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL.String(), http.NoBody)
	if err != nil {
		return nil, errorx.Decorate(err, "create request")
	}

	req.Header.Add("Referer", referrer)
	req.Header.Add("X-Requested-With", "XMLHttpRequest")
	req.Header.Add("X-Retpath-Y", referrer)

	return req, nil
}

func getExtractionURL(title string) string {
	return "https://www.youtube.com/results?search_query=" + title
}

func getDisplayURL(id string) string {
	return "https://music.yandex.ru/track/" + id
}
