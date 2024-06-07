package yandexmusic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ProjectOrca/extractor"
	"ProjectOrca/utils"
	"ProjectOrca/ytdl"

	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
)

var (
	ErrInvalidYMURL           = errors.New("invalid yandex music url")
	ErrCannotExtractStreamURL = errors.New("cannot use yandex music extractor to extract stream url")
	ErrNoAlbum                = errorx.IllegalState.New("missing album in track results")
)

type Track struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	DurationMS int    `json:"durationMs"`

	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`

	Albums []struct {
		ID int `json:"id"`
	} `json:"albums"`
}

func (t Track) getTitle() string {
	artists := ""

	if len(t.Artists) > 0 {
		artistsNames := make([]string, len(t.Artists))
		for i, artist := range t.Artists {
			artistsNames[i] = artist.Name
		}

		artists = strings.Join(artistsNames, ", ") + " - "
	}

	return artists + t.Title
}

func (t Track) toTrackData() (extractor.TrackData, error) {
	var res extractor.TrackData

	if len(t.Albums) == 0 {
		return res, ErrNoAlbum
	}

	albumID := t.Albums[0].ID
	title := t.getTitle()

	return extractor.TrackData{
		Title:         title,
		ExtractionURL: getDisplayURL(albumID, t.ID),
		DisplayURL:    getDisplayURL(albumID, t.ID),
		StreamURL:     "",
		Live:          false,
		Duration:      time.Duration(t.DurationMS) * time.Millisecond,
		HTTPHeaders:   nil,
	}, nil
}

type SingleTrackData struct {
	Track Track `json:"track"`
}

type AlbumData struct {
	Volumes [][]Track `json:"volumes"`
}

type PlaylistData struct {
	Tracks []SingleTrackData `json:"tracks"`
}

type Response[T any] struct {
	Result T `json:"result"`
}

type YandexMusic struct {
	baseURL url.URL
	client  *http.Client
	token   string

	ytdl *ytdl.YTDL
}

func New(ytdl *ytdl.YTDL, token string) *YandexMusic {
	var client http.Client

	//nolint:exhaustruct
	baseURL := url.URL{
		Scheme: "https",
		Host:   "api.music.yandex.net",
	}

	return &YandexMusic{
		baseURL: baseURL,
		client:  &client,
		token:   token,

		ytdl: ytdl,
	}
}

func (y *YandexMusic) QueryMatches(_ context.Context, q string) bool {
	return utils.YandexMusicTrackRx.MatchString(q) ||
		utils.YandexMusicAlbumRx.MatchString(q) ||
		utils.YandexMusicPlaylistRx.MatchString(q)
}

func (y *YandexMusic) ExtractTracksData(ctx context.Context, url string) ([]extractor.TrackData, error) {
	var res []extractor.TrackData

	var err error

	if utils.YandexMusicTrackRx.MatchString(url) {
		res, err = y.ytdl.ExtractTracksDataHeaders(ctx, url, y.authHeader())
	} else if matches := utils.YandexMusicAlbumRx.FindStringSubmatch(url); len(matches) >= 2 { //nolint:mnd // see regex.
		res, err = y.getAlbumTracksData(ctx, matches[1])
	} else if matches := utils.YandexMusicPlaylistRx.FindStringSubmatch(url); len(matches) >= 3 { //nolint:mnd
		res, err = y.getPlaylistTracksData(ctx, matches[1], matches[2])
	} else {
		err = ErrInvalidYMURL
	}

	if err != nil {
		return nil, errorx.Decorate(err, "extract tracks")
	}

	return res, nil
}

func (y *YandexMusic) ExtractionURLMatches(_ context.Context, q string) bool {
	return utils.YandexMusicTrackRx.MatchString(q)
}

func (y *YandexMusic) ExtractStreamURL(ctx context.Context, url string) (string, time.Duration, error) {
	stream, dur, err := y.ytdl.ExtractStreamURLHeaders(ctx, url, y.authHeader())
	if err != nil {
		return "", 0, errorx.Decorate(err, "extract stream url with ytdl")
	}

	return stream, dur, nil
}

func (y *YandexMusic) getAlbumTracksData(ctx context.Context, albumID string) ([]extractor.TrackData, error) {
	url := y.baseURL

	url.Path = "/albums/" + albumID + "/with-tracks"

	var data Response[AlbumData]

	err := y.doRequest(ctx, url, &data)
	if err != nil {
		return nil, errorx.Decorate(err, "do request")
	}

	res := make([]extractor.TrackData, 0)

	for _, volume := range data.Result.Volumes {
		for _, track := range volume {
			trackData, err := track.toTrackData()
			if err != nil {
				if errors.Is(err, ErrNoAlbum) {
					continue
				}

				return nil, errorx.Decorate(err, "convert track to data")
			}

			res = append(res, trackData)
		}
	}

	return res, nil
}

func (y *YandexMusic) getPlaylistTracksData(
	ctx context.Context, owner, playlistID string,
) ([]extractor.TrackData, error) {
	url := y.baseURL

	url.Path = "/users/" + owner + "/playlists/" + playlistID

	var data Response[PlaylistData]

	err := y.doRequest(ctx, url, &data)
	if err != nil {
		return nil, errorx.Decorate(err, "do request")
	}

	res := make([]extractor.TrackData, 0)

	for _, track := range data.Result.Tracks {
		trackData, err := track.Track.toTrackData()
		if err != nil {
			if errors.Is(err, ErrNoAlbum) {
				continue
			}

			return nil, errorx.Decorate(err, "convert track to data")
		}

		res = append(res, trackData)
	}

	return res, nil
}

func (y *YandexMusic) doRequest(ctx context.Context, url url.URL, dest any) error {
	logger := zerolog.Ctx(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), http.NoBody)
	if err != nil {
		return errorx.Decorate(err, "create request")
	}

	req.Header.Add("Authorization", "OAuth "+y.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errorx.Decorate(err, "send request")
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			logger.Error().Err(err).Msg("Error closing body")
		}
	}()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return errorx.Decorate(err, "read body")
	}

	logger.Debug().Str("body", string(b)).Send()

	err = json.Unmarshal(b, dest)
	if err != nil {
		return errorx.Decorate(err, "decode response")
	}

	return nil
}

func getDisplayURL(albumID int, trackID string) string {
	return fmt.Sprintf("https://music.yandex.ru/album/%d/track/%s", albumID, trackID)
}

func (y *YandexMusic) authHeader() map[string]string {
	return map[string]string{
		"Authorization": "OAuth " + y.token,
	}
}
