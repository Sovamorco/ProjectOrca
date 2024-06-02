package utils

import (
	"regexp"

	"github.com/joomcode/errorx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"
)

var (
	URLRx = regexp.MustCompile(`https?://(?:www\.)?`)

	SpotifyRx = regexp.MustCompile(
		`(?:spotify:|(?:https?://)?(?:www\.)?open\.spotify\.com/)(playlist|track|album)[:/]([a-zA-Z0-9]+)`,
	)

	YandexMusicTrackRx = regexp.MustCompile(
		`https?://(?:www\.)?music\.yandex\.(?:[^/]+)(?:/album/(?:[0-9]+))?/track/([0-9]+)`,
	)
	YandexMusicAlbumRx = regexp.MustCompile(
		`https?://(?:www\.)?music\.yandex\.(?:[^/]+)/album/([0-9]+)`,
	)
	YandexMusicPlaylistRx = regexp.MustCompile(
		`https?://(?:www\.)?music\.yandex\.(?:[^/]+)/users/([^/]+)/playlists/([0-9]+)`,
	)

	VKAlbumRX = regexp.MustCompile(
		`https?://(?:www\.)?vk\.com/.*?(?:audio_playlist|album/|playlist/)(-?[0-9]+)_([0-9]+)(?:(?:%2f|%2F|/|_)([a-z0-9]+))?`,
	)
	VKPersRX  = regexp.MustCompile(`https?://(?:www\.)?vk\.com/.*?audios(-?[0-9]+)`)
	VKTrackRx = regexp.MustCompile(`https?://(?:www\.)?vk\.com/.*?audio(-?[0-9]+)_([0-9]+)(?:(?:%2f|%2F|/|_)([a-z0-9]+))?`)
)

func Flatten[S ~[]E, E any](s []S) S {
	var res S

	for _, l := range s {
		res = append(res, l...)
	}

	return res
}

func MustCreateStatus(c codes.Code, msg string, details ...protoadapt.MessageV1) *status.Status {
	s := status.New(c, msg)

	s, err := s.WithDetails(details...)
	if err != nil {
		panic(errorx.Decorate(err, "with details"))
	}

	return s
}

//nolint:ireturn // that signature is required.
func MarshalErrorxStack(ierr error) any {
	err := errorx.Cast(ierr)
	if err == nil {
		return nil
	}

	return err.MarshalStackTrace()
}
