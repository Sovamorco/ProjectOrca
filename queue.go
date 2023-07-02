package main

import (
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
)

var currentQueue *queue

type queue struct {
	logger    *zap.SugaredLogger
	session   *discordgo.Session
	guildID   string
	channelID string
	tracks    []*musicTrack
	vc        *discordgo.VoiceConnection
}

func newQueue(logger *zap.SugaredLogger, s *discordgo.Session, guildID, channelID string) *queue {
	return &queue{
		logger:    logger,
		tracks:    make([]*musicTrack, 0),
		session:   s,
		guildID:   guildID,
		channelID: channelID,
	}
}

func (q *queue) add(ms *musicTrack) {
	shouldStart := len(q.tracks) == 0
	q.tracks = append(q.tracks, ms)
	if shouldStart {
		go q.start()
	}
}

func (q *queue) start() {
	vc, err := q.session.ChannelVoiceJoin(q.guildID, q.channelID, false, true)
	if err != nil {
		q.logger.Error("Error joining voice channel: ", err)
	}
	q.vc = vc
	defer func(vc *discordgo.VoiceConnection) {
		err := vc.Disconnect()
		if err != nil {
			q.logger.Error("Error leaving voice channel: ", err)
		}
	}(vc)

	for len(q.tracks) > 0 {
		done := make(chan error)
		q.tracks[0].streamToVC(q.vc, done)
		err := <-done
		if err != nil {
			q.logger.Error("Error when streaming track: ", err)
		}
		q.tracks = q.tracks[1:]
	}
}
