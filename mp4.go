package main

import (
	"github.com/bwmarrin/discordgo"
	"io"
	"time"
)

type mp4MusicStream struct {
	stream io.ReadSeeker
}

func newMp4MusicStream(stream io.ReadSeeker) (*mp4MusicStream, error) {
	ms := mp4MusicStream{
		stream: stream,
	}
	return &ms, nil
}

func (ms *mp4MusicStream) stop() {

}

func (ms *mp4MusicStream) seek(tc time.Duration) {

}

func (ms *mp4MusicStream) sendToVC(done chan struct{}, vc *discordgo.VoiceConnection) {

}
