package main

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
)

const TokenLength = 128

var urlRx = regexp.MustCompile(`https?://(?:www\.)?.+`)

func atMostAbs[T int | float32 | float64](num, clamp T) T {
	if num < 0 {
		return max(num, -clamp)
	}
	return min(num, clamp)
}

func empty[T any](c chan T) {
	for len(c) != 0 {
		select {
		case <-c:
		default:
		}
	}
}

// https://stackoverflow.com/questions/45267125/how-to-generate-unique-random-alphanumeric-tokens-in-golang
func generateSecureToken() (string, error) {
	b := make([]byte, TokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
