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
	vc, err := q.Session.ChannelVoiceJoin(q.GuildState.Guild.ID, q.channelID, false, true)
	if err != nil {
		q.Logger.Error("Error joining voice channel: ", err)
	}
	q.vc = vc
	defer func(vc *discordgo.VoiceConnection) {
		err := vc.Disconnect()
		if err != nil {
			q.Logger.Error("Error leaving voice channel: ", err)
		}
	}(vc)

	for len(q.Tracks) > 0 {
		done := make(chan error)
		q.Tracks[0].streamToVC(q.vc, done)
		err := <-done
		if err != nil {
			q.Logger.Error("Error when streaming track: ", err)
		}
		q.Tracks = q.Tracks[1:]
	}
}
