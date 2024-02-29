package extractor

import (
	"context"
	"time"

	pb "ProjectOrca/proto"
	"ProjectOrca/utils"

	"github.com/joomcode/errorx"
	"google.golang.org/grpc/codes"
)

var (
	ErrNoExtractor = utils.MustCreateStatus(codes.InvalidArgument, "no extractor for given url", &pb.ErrorCodeWrapper{
		Code: pb.ErrorCode_ErrNoExtractor,
	}).Err()
	ErrNoResults = utils.MustCreateStatus(codes.InvalidArgument, "no results for given url", &pb.ErrorCodeWrapper{
		Code: pb.ErrorCode_ErrNoResults,
	}).Err()
)

type TrackData struct {
	Title         string
	ExtractionURL string
	DisplayURL    string
	StreamURL     string
	Live          bool
	Duration      time.Duration
	HTTPHeaders   map[string]string
}

type Extractor interface {
	QueryMatches(ctx context.Context, query string) bool
	ExtractTracksData(ctx context.Context, url string) ([]TrackData, error)

	ExtractionURLMatches(ctx context.Context, url string) bool
	ExtractStreamURL(ctx context.Context, url string) (string, time.Duration, error)
}

type Extractors struct {
	extractors []Extractor
}

func NewExtractors(extractors ...Extractor) *Extractors {
	return &Extractors{
		extractors: extractors,
	}
}

func (e *Extractors) AddExtractor(ex Extractor) {
	e.extractors = append(e.extractors, ex)
}

func (e *Extractors) ExtractTracksData(ctx context.Context, url string) ([]TrackData, error) {
	for _, extractor := range e.extractors {
		if extractor.QueryMatches(ctx, url) {
			d, err := extractor.ExtractTracksData(ctx, url)
			if err != nil {
				return nil, errorx.Decorate(err, "extract tracks data")
			}

			return d, nil
		}
	}

	return nil, ErrNoExtractor
}

func (e *Extractors) ExtractStreamURL(ctx context.Context, extURL string) (string, time.Duration, error) {
	for _, extractor := range e.extractors {
		if extractor.ExtractionURLMatches(ctx, extURL) {
			s, dur, err := extractor.ExtractStreamURL(ctx, extURL)
			if err != nil {
				return "", 0, errorx.Decorate(err, "extract stream url")
			}

			return s, dur, nil
		}
	}

	return "", 0, ErrNoExtractor
}
