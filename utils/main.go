package utils

import (
	"regexp"
	"time"
)

var (
	URLRx     = regexp.MustCompile(`https?://(?:www\.)?.+`)
	SpotifyRx = regexp.MustCompile(
		`(?:spotify:|(?:https?://)?(?:www\.)?open\.spotify\.com/)(playlist|track|album)[:/]([a-zA-Z0-9]+)(.*)$`,
	)
)

const (
	// MinDuration - copied from time.minDuration.
	MinDuration time.Duration = -1 << 63
)
