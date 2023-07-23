package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"ProjectOrca/extractor"
	"ProjectOrca/utils"

	"github.com/joomcode/errorx"
	"github.com/zmb3/spotify/v2"
	"github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	tracksItemsLimit = 50
)

var (
	ErrInvalidSpotifyURL      = errors.New("invalid spotify url")
	ErrCannotExtractStreamURL = errors.New("cannot use spotify extractor to extract stream url")
)

type YTDLSearchData struct {
	Entries []YTDLSearchDatum `json:"entries"`
}

type YTDLSearchDatum struct {
	OriginalURL string `json:"original_url"`
	URL         string `json:"url"`
}

type Spotify struct {
	client     *spotify.Client
	httpClient *http.Client
	config     *clientcredentials.Config
}

func New(ctx context.Context, clientID, clientSecret string) (*Spotify, error) {
	config := &clientcredentials.Config{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		TokenURL:       spotifyauth.TokenURL,
		Scopes:         nil,
		EndpointParams: nil,
		AuthStyle:      0,
	}

	token, err := config.Token(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "get token")
	}

	httpClient := spotifyauth.New().Client(ctx, token)
	client := spotify.New(httpClient)

	return &Spotify{
		client:     client,
		httpClient: httpClient,
		config:     config,
	}, nil
}

func (s *Spotify) QueryMatches(_ context.Context, q string) bool {
	return utils.SpotifyRx.MatchString(q)
}

func (s *Spotify) ExtractTracksData(ctx context.Context, url string) ([]extractor.TrackData, error) {
	matches := utils.SpotifyRx.FindStringSubmatch(url)
	if len(matches) < 3 { //nolint:gomnd // see regex
		return nil, ErrInvalidSpotifyURL
	}

	urlType := matches[1]
	identifier := matches[2]

	err := s.refreshToken(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "refresh spotify token")
	}

	spotifyID := spotify.ID(identifier)

	var data []extractor.TrackData

	switch urlType {
	case "track":
		data, err = s.getTrackData(ctx, spotifyID)
	case "album":
		data, err = s.getAlbumTracksData(ctx, spotifyID)
	case "playlist":
		data, err = s.getPlaylistTracksData(ctx, spotifyID)
	default:
		return nil, ErrInvalidSpotifyURL
	}

	if err != nil {
		return nil, errorx.Decorate(err, "get tracks data")
	}

	return data, nil
}

func (s *Spotify) ExtractionURLMatches(_ context.Context, _ string) bool {
	// cannot use spotify for stream url extraction
	return false
}

func (s *Spotify) ExtractStreamURL(_ context.Context, _ string) (string, error) {
	return "", ErrCannotExtractStreamURL
}

func (s *Spotify) refreshToken(ctx context.Context) error {
	curr, err := s.client.Token()
	if err != nil {
		return errorx.Decorate(err, "get current token")
	}

	if curr.Valid() {
		return nil
	}

	newToken, err := s.config.Token(ctx)
	if err != nil {
		return errorx.Decorate(err, "generate new token")
	}

	s.config.Client(ctx)

	newClient := spotifyauth.New().Client(ctx, newToken)
	s.httpClient.Transport = newClient.Transport

	return nil
}

func (s *Spotify) getTrackData(ctx context.Context, id spotify.ID) ([]extractor.TrackData, error) {
	tracks, err := s.client.GetTracks(ctx, []spotify.ID{id})
	if err != nil {
		return nil, errorx.Decorate(err, "get spotify tracks")
	}

	if len(tracks) != 1 || tracks[0] == nil {
		return nil, extractor.ErrNoResults
	}

	track := tracks[0]

	title := getTrackTitle(track.SimpleTrack)

	return []extractor.TrackData{
		{
			Title:         title,
			ExtractionURL: fmt.Sprintf("ytsearch1:%s", title),
			DisplayURL:    getTrackDisplayURL(track.SimpleTrack),
			StreamURL:     "",
			Live:          false,
			Duration:      track.TimeDuration(),
			HTTPHeaders:   nil,
		},
	}, nil
}

func (s *Spotify) getAlbumTracksIter(ctx context.Context, id spotify.ID) ([]spotify.SimpleTrack, error) {
	res := make([]spotify.SimpleTrack, 0)

	for {
		page, err := s.client.GetAlbumTracks(ctx, id, spotify.Limit(tracksItemsLimit), spotify.Offset(len(res)))
		if err != nil {
			return nil, errorx.Decorate(err, "get album tracks")
		}

		res = append(res, page.Tracks...)

		if len(res) == page.Total {
			break
		}
	}

	return res, nil
}

func (s *Spotify) getAlbumTracksData(ctx context.Context, id spotify.ID) ([]extractor.TrackData, error) {
	tracks, err := s.getAlbumTracksIter(ctx, id)
	if err != nil {
		return nil, errorx.Decorate(err, "get album")
	}

	res := make([]extractor.TrackData, len(tracks))

	for i, track := range tracks {
		title := getTrackTitle(track)

		res[i] = extractor.TrackData{
			Title:         title,
			ExtractionURL: fmt.Sprintf("ytsearch1:%s", title),
			DisplayURL:    getTrackDisplayURL(track),
			StreamURL:     "",
			Live:          false,
			Duration:      track.TimeDuration(),
			HTTPHeaders:   nil,
		}
	}

	return res, nil
}

func (s *Spotify) getPlaylistItemsIter(ctx context.Context, id spotify.ID) ([]spotify.PlaylistItem, error) {
	res := make([]spotify.PlaylistItem, 0)

	for {
		page, err := s.client.GetPlaylistItems(ctx, id, spotify.Limit(tracksItemsLimit), spotify.Offset(len(res)))
		if err != nil {
			return nil, errorx.Decorate(err, "get playlist items")
		}

		res = append(res, page.Items...)

		if len(res) == page.Total {
			break
		}
	}

	return res, nil
}

func (s *Spotify) getPlaylistTracksData(ctx context.Context, id spotify.ID) ([]extractor.TrackData, error) {
	items, err := s.getPlaylistItemsIter(ctx, id)
	if err != nil {
		return nil, errorx.Decorate(err, "get playlist")
	}

	res := make([]extractor.TrackData, len(items))

	for i, track := range items {
		title := getTrackTitle(track.Track.Track.SimpleTrack)

		res[i] = extractor.TrackData{
			Title:         title,
			ExtractionURL: fmt.Sprintf("ytsearch1:%s", title),
			DisplayURL:    getTrackDisplayURL(track.Track.Track.SimpleTrack),
			StreamURL:     "",
			Live:          false,
			Duration:      track.Track.Track.TimeDuration(),
			HTTPHeaders:   nil,
		}
	}

	return res, nil
}

func getTrackTitle(track spotify.SimpleTrack) string {
	if len(track.Artists) == 0 {
		return track.Name
	}

	artists := make([]string, len(track.Artists))
	for i, artist := range track.Artists {
		artists[i] = artist.Name
	}

	return fmt.Sprintf("%s - %s", strings.Join(artists, ", "), track.Name)
}

func getTrackDisplayURL(track spotify.SimpleTrack) string {
	s, ok := track.ExternalURLs["spotify"]
	if ok {
		return s
	}

	return track.Endpoint
}
