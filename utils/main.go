package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"

	"github.com/joomcode/errorx"
	"google.golang.org/protobuf/proto"
)

const (
	TokenLength = 128
)

var URLRx = regexp.MustCompile(`https?://(?:www\.)?.+`)

// codec is a Codec implementation with protobuf. It is the default codec for gRPC.
type codec struct{}

func (codec) Marshal(v interface{}) ([]byte, error) {
	vv, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("failed to marshal, message is %T, want proto.Message", v) //nolint:goerr113
	}

	return proto.Marshal(vv) //nolint:wrapcheck
}

func (codec) Unmarshal(data []byte, v interface{}) error {
	vv, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("failed to unmarshal, message is %T, want proto.Message", v) //nolint:goerr113
	}

	return proto.Unmarshal(data, vv) //nolint:wrapcheck
}

func (codec) Name() string {
	return "proto"
}

type BytesOrProtoMarshaller struct {
	Default codec `exhaustruct:"optional"`
}

func (b BytesOrProtoMarshaller) Marshal(v any) ([]byte, error) {
	vv, ok := v.([]byte)
	if ok {
		return vv, nil
	}

	return b.Default.Marshal(v)
}

func (b BytesOrProtoMarshaller) Unmarshal(data []byte, v any) error {
	vv, ok := v.(*[]byte) // pointer to slice is weird I know
	if ok {
		*vv = data

		return nil
	}

	return b.Default.Unmarshal(data, v)
}

func (b BytesOrProtoMarshaller) Name() string {
	return "BytesOrProto"
}

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
