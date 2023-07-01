package main

import (
	"github.com/bwmarrin/discordgo"
	"github.com/ebml-go/webm"
	"github.com/pkg/errors"
	"io"
	"sync"
	"time"
)

type webmMusicStream struct {
	sync.Mutex
	stream *webm.Reader
}

func newWebmMusicStream(stream io.ReadSeeker) (*webmMusicStream, error) {
	var m webm.WebM
	webmStream, err := webm.Parse(stream, &m)
	if err != nil {
		return nil, errors.Wrap(err, "parse webm")
	}
	ws := webmMusicStream{
		stream: webmStream,
	}
	return &ws, nil
}

func (ws *webmMusicStream) seek(tc time.Duration) {
	foundSeekPacket := false
	// 10 seconds is usually the time between cues in YouTube webm, we
	// subtract it because ebml-go/webm stream implementation searches
	// for greater than or equal cue, but we need the closest less than
	// or equal cue
	// and if the time between cues is more than 10 seconds then whatever
	seekDuration := tc - 9990*time.Millisecond
	ws.Lock()
	ws.stream.Seek(seekDuration)
	for packet := range ws.stream.Chan {
		if foundSeekPacket && packet.Timecode+20*time.Millisecond >= tc {
			break
		} else if packet.Data == nil && packet.Timecode == seekDuration {
			foundSeekPacket = true
		}
	}
	ws.Unlock()
}

func (ws *webmMusicStream) stop() {
	ws.stream.Shutdown()
}

func (ws *webmMusicStream) sendToVC(done chan struct{}, vc *discordgo.VoiceConnection) {
	var packet webm.Packet
	var ok bool
	for {
		ws.Lock()
		packet, ok = <-ws.stream.Chan
		ws.Unlock()
		if !ok || packet.Timecode < 0 {
			done <- struct{}{}
			return
		}

		vc.OpusSend <- packet.Data
	}
}
