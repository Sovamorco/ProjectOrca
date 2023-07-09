package main

import (
	"github.com/bwmarrin/discordgo"
)

type Queue struct {
	*GuildState
	channelID string
	Tracks    []*MusicTrack
	vc        *discordgo.VoiceConnection
}

func (g *GuildState) newQueue(channelID string) {
	g.Queue = &Queue{
		GuildState: g,
		Tracks:     make([]*MusicTrack, 0),
		channelID:  channelID,
	}
}

func (q *Queue) add(ms *MusicTrack) {
	shouldStart := len(q.Tracks) == 0
	q.Tracks = append(q.Tracks, ms)
	if shouldStart {
		go q.start()
	}
}

func (q *Queue) start() {
	q.Logger.Info("Starting playback")
	vc, err := q.Session.ChannelVoiceJoin(q.GuildState.GuildID, q.channelID, false, true)
	if err != nil {
		q.Logger.Error("Error joining voice channel: ", err)
		return
	}
	q.vc = vc
	defer func(vc *discordgo.VoiceConnection) {
		err := vc.Disconnect()
		if err != nil {
			q.Logger.Error("Error leaving voice channel: ", err)
		}
	}(vc)

	for len(q.Tracks) > 0 {
		done := make(chan error, 1)
		q.Tracks[0].streamToVC(q.vc, done)
		err := <-done
		if err != nil {
			q.Logger.Error("Error when streaming track: ", err)
		}
		// check before indexing because of stop call
		if len(q.Tracks) < 1 {
			break
		}
		q.Tracks = q.Tracks[1:]
	}
}
