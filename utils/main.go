package utils

import (
	"regexp"
)

var (
	URLRx     = regexp.MustCompile(`https?://(?:www\.)?`)
	SpotifyRx = regexp.MustCompile(
		`(?:spotify:|(?:https?://)?(?:www\.)?open\.spotify\.com/)(playlist|track|album)[:/]([a-zA-Z0-9]+)`,
	)
	VKAlbumRX = regexp.MustCompile(
		`https?://(?:www\.)?vk\.com/.*?(?:audio_playlist|album/|playlist/)(-?[0-9]+)_([0-9]+)(?:(?:%2f|%2F|/|_)([a-z0-9]+))?`,
	)
	VKPersRX  = regexp.MustCompile(`https?://(?:www\.)?vk\.com/.*?audios(-?[0-9]+)`)
	VKTrackRx = regexp.MustCompile(`https?://(?:www\.)?vk\.com/.*?audio(-?[0-9]+)_([0-9]+)(?:(?:%2f|%2F|/|_)([a-z0-9]+))?`)
)
