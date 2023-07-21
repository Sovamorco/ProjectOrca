package utils

import (
	"regexp"
	"time"
)

var URLRx = regexp.MustCompile(`https?://(?:www\.)?.+`)

const (
	// MinDuration - copied from time.minDuration.
	MinDuration time.Duration = -1 << 63
)
