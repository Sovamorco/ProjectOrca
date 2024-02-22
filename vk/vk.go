package vk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"

	"ProjectOrca/extractor"
	"ProjectOrca/utils"

	"github.com/joomcode/errorx"
	"go.uber.org/zap"
)

const (
	userAgent = "KateMobileAndroid/52.1 lite-445 (Android 4.4.2; SDK 19; x86; unknown Android SDK built for x86; en)"
)

var (
	ErrInvalidVKURL           = errors.New("invalid vk album/personal url")
	ErrCannotExtractStreamURL = errors.New("cannot extract stream url from vk")
)

type APIError struct {
	ErrorCode int    `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

type Track struct {
	ID       int    `json:"id"`
	OwnerID  int    `json:"owner_id"`
	Artist   string `json:"artist"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Duration int    `json:"duration"`
}

type AudioResponse struct {
	Items []Track `json:"items"`
}

type APIAudioResponse struct {
	Error    *APIError      `json:"error"`
	Response *AudioResponse `json:"response"`
}

type APIAudioGetByIDResponse struct {
	Error    *APIError `json:"error"`
	Response []Track   `json:"response"`
}

func (a APIError) Error() string {
	return fmt.Sprintf("api error: code %d, message: \"%s\"", a.ErrorCode, a.ErrorMsg)
}

type VK struct {
	logger *zap.SugaredLogger

	Token string
}

func New(_ context.Context, logger *zap.SugaredLogger, token string) (*VK, error) {
	return &VK{
		logger: logger.Named("vk"),

		Token: token,
	}, nil
}

func (v *VK) QueryMatches(_ context.Context, q string) bool {
	return utils.VKAlbumRX.MatchString(q) || utils.VKPersRX.MatchString(q) || utils.VKTrackRx.MatchString(q)
}

func (v *VK) ExtractTracksData(ctx context.Context, q string) ([]extractor.TrackData, error) {
	if matches := utils.VKAlbumRX.FindStringSubmatch(q); len(matches) >= 3 { //nolint:gomnd // no comment
		return v.extractAlbumTracksData(ctx, matches)
	} else if matches = utils.VKPersRX.FindStringSubmatch(q); len(matches) >= 2 { //nolint:gomnd // no comment
		return v.extractPersonalTracksData(ctx, matches)
	} else if matches = utils.VKTrackRx.FindStringSubmatch(q); len(matches) >= 3 { //nolint:gomnd // no comment
		return v.extractTrackData(ctx, matches)
	}

	return nil, ErrInvalidVKURL
}

func (v *VK) extractAlbumTracksData(ctx context.Context, matches []string) ([]extractor.TrackData, error) {
	body := map[string]string{
		"owner_id":    matches[1],
		"playlist_id": matches[2],
	}

	if len(matches) >= 4 { //nolint:gomnd // no comment
		body["access_key"] = matches[3]
	}

	var res APIAudioResponse

	err := v.doRequest(ctx, "https://api.vk.com/method/audio.get", body, &res)
	if err != nil {
		return nil, errorx.Decorate(err, "get audio tracks")
	} else if res.Error != nil {
		return nil, errorx.Decorate(res.Error, "get audio tracks")
	}

	return tracksToData(res.Response.Items), nil
}

func (v *VK) extractPersonalTracksData(ctx context.Context, matches []string) ([]extractor.TrackData, error) {
	body := map[string]string{
		"owner_id": matches[1],
	}

	var res APIAudioResponse

	err := v.doRequest(ctx, "https://api.vk.com/method/audio.get", body, &res)
	if err != nil {
		return nil, errorx.Decorate(err, "get audio tracks")
	} else if res.Error != nil {
		return nil, errorx.Decorate(res.Error, "get audio tracks")
	}

	return tracksToData(res.Response.Items), nil
}

func (v *VK) extractTrackData(ctx context.Context, matches []string) ([]extractor.TrackData, error) {
	body := map[string]string{
		"audios": fmt.Sprintf("%s_%s", matches[1], matches[2]),
	}

	if len(matches) >= 4 { //nolint:gomnd // no comment
		body["audios"] += "_" + matches[3]
	}

	var res APIAudioGetByIDResponse

	err := v.doRequest(ctx, "https://api.vk.com/method/audio.getById", body, &res)
	if err != nil {
		return nil, errorx.Decorate(err, "get audio tracks")
	} else if res.Error != nil {
		return nil, errorx.Decorate(res.Error, "get audio tracks")
	}

	return tracksToData(res.Response), nil
}

func (v *VK) doRequest(ctx context.Context, url string, body map[string]string, dest any) error {
	body["access_token"] = v.Token
	body["v"] = "5.999"

	var b bytes.Buffer

	w := multipart.NewWriter(&b)
	for k, val := range body {
		err := w.WriteField(k, val)
		if err != nil {
			return errorx.Decorate(err, "write multipart field")
		}
	}

	err := w.Close()
	if err != nil {
		return errorx.Decorate(err, "close multipart writer")
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, url, &b,
	)
	if err != nil {
		return errorx.Decorate(err, "create request")
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", w.FormDataContentType())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return errorx.Decorate(err, "do request")
	}

	defer func() {
		err = res.Body.Close()
		if err != nil {
			v.logger.Errorf("Error closing body: %+v", err)
		}
	}()

	err = json.NewDecoder(res.Body).Decode(dest)
	if err != nil {
		return errorx.Decorate(err, "decode response")
	}

	return nil
}

func (v *VK) ExtractionURLMatches(context.Context, string) bool {
	return false
}

func (v *VK) ExtractStreamURL(context.Context, string) (string, time.Duration, error) {
	return "", 0, ErrCannotExtractStreamURL
}

func tracksToData(tracks []Track) []extractor.TrackData {
	res := make([]extractor.TrackData, len(tracks))

	for i, item := range tracks {
		title := fmt.Sprintf("%s - %s", item.Artist, item.Title)

		res[i] = extractor.TrackData{
			Title:         title,
			ExtractionURL: "ytsearch:" + title, // in case stream url fails
			DisplayURL:    fmt.Sprintf("https://vk.com/audio%d_%d", item.OwnerID, item.ID),
			StreamURL:     item.URL,
			Live:          false,
			Duration:      time.Second * time.Duration(item.Duration),
			HTTPHeaders: map[string]string{
				"User-Agent": userAgent,
			},
		}
	}

	return res
}
