package utils

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
)

const TokenLength = 128

var UrlRx = regexp.MustCompile(`https?://(?:www\.)?.+`)

// GenerateSecureToken
// yoinked from https://stackoverflow.com/questions/45267125/how-to-generate-unique-random-alphanumeric-tokens-in-golang
func GenerateSecureToken() (string, error) {
	b := make([]byte, TokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func AtMostAbs[T int | float32 | float64](num, clamp T) T {
	if num < 0 {
		return max(num, -clamp)
	}
	return min(num, clamp)
}

func Empty[T any](c chan T) {
	for len(c) != 0 {
		select {
		case <-c:
		default:
		}
	}
}
