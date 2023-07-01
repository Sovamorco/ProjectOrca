package main

import (
	"fmt"
	"github.com/kkdai/youtube/v2"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"strings"
)

var AudioQuality = map[string]int{
	"AUDIO_QUALITY_LOW":    1,
	"AUDIO_QUALITY_MEDIUM": 2,
	"AUDIO_QUALITY_HIGH":   3,
}

var ytClient = youtube.Client{}

func ytKey(url string) bool {
	return strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be")
}

func ytExtractor(logger *zap.SugaredLogger, url string) (*musicStreamData, error) {
	video, err := ytClient.GetVideo(url)
	if err != nil {
		return nil, errors.Wrap(err, "get video")
	}
	format, err := selectAudioFormat(logger, video)
	if err != nil {
		return nil, errors.Wrap(err, "select audio format")
	}
	streamURL, err := ytClient.GetStreamURL(video, format)
	if err != nil {
		return nil, errors.Wrap(err, "get stream url")
	}
	var mf mediaFormat
	switch true {
	case strings.Contains(format.MimeType, "audio/webm") && strings.Contains(format.MimeType, "opus"):
		mf = mediaFormatWebmOpus
	case strings.Contains(format.MimeType, "audio/mp4"):
		mf = mediaFormatMp4
	default:
		return nil, fmt.Errorf("could not find media format for mime type \"%s\"", format.MimeType)
	}
	msd := musicStreamData{
		streamURL: streamURL,
		format:    mf,
		title:     fmt.Sprintf("%s by %s", video.Title, video.Author),
	}
	return &msd, nil
}

func selectAudioFormat(logger *zap.SugaredLogger, video *youtube.Video) (*youtube.Format, error) {
	var selected *youtube.Format
	selectedPriority := 0
	for _, format := range video.Formats.WithAudioChannels() {
		format := format
		priority := 0
		// we would prefer not to convert sample rate
		if format.AudioSampleRate != "48000" {
			priority--
		}
		// we would prefer not to convert codec
		if !strings.Contains(format.MimeType, "opus") {
			priority--
		}
		// we would really prefer not to have to download video
		if strings.Contains(format.MimeType, "video/") {
			priority -= 5
		}
		if selected == nil {
			selected = &format
			selectedPriority = priority
			continue
		}
		formatAudioQuality, ok := AudioQuality[format.AudioQuality]
		if !ok {
			formatAudioQuality = 0
		}
		selectedAudioQuality, ok := AudioQuality[selected.AudioQuality]
		if !ok {
			selectedAudioQuality = 0
		}
		// we value audio quality a lot
		if formatAudioQuality > selectedAudioQuality {
			priority += 2
		}
		if priority > selectedPriority {
			selected = &format
			selectedPriority = priority
		}
	}
	if selected == nil {
		return nil, errors.New("failed to find selected format")
	}

	logger.Infof("selected: %s, %s", selected.MimeType, selected.AudioSampleRate)

	return selected, nil
}
