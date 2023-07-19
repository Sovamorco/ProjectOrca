package utils

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"

	"github.com/joomcode/errorx"
)

const TokenLength = 128

var URLRx = regexp.MustCompile(`https?://(?:www\.)?.+`)

type Number interface {
	int | int8 | int16 | int32 | int64 |
		uint | uint8 | uint16 | uint32 | uint64 |
		float32 | float64
}

// GenerateSecureToken
// yoinked from https://stackoverflow.com/questions/45267125/how-to-generate-unique-random-alphanumeric-tokens-in-golang
func GenerateSecureToken() (string, error) {
	b := make([]byte, TokenLength)

	if _, err := rand.Read(b); err != nil {
		return "", errorx.Decorate(err, "read from random source")
	}

	return hex.EncodeToString(b), nil
}

func Sum[T Number](nums ...T) T {
	if len(nums) == 0 {
		return 0
	}

	sum := nums[0]

	for _, num := range nums[1:] {
		sum += num
	}

	return sum
}

func Mean[T Number](nums ...T) float64 {
	if len(nums) == 0 {
		return 0
	}

	return float64(Sum(nums...)) / float64(len(nums))
}
