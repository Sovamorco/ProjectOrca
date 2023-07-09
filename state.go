package main

import (
	pb "ProjectOrca/proto"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

type State struct {
	Guilds  map[string]*GuildState
	Logger  *zap.SugaredLogger
	Session *discordgo.Session
}

func newState(logger *zap.SugaredLogger, session *discordgo.Session) *State {
	s := State{
		Guilds:  make(map[string]*GuildState),
		Logger:  logger,
		Session: session,
	}
	return &s
}

type GuildState struct {
	*State
	GuildID      string
	Queue        *Queue
	Volume       float32
	TargetVolume float32
}

func (s *State) newGuildState(guildID string) *GuildState {
	gs := &GuildState{
		State:        s,
		GuildID:      guildID,
		Volume:       1,
		TargetVolume: 1,
	}
	s.Guilds[guildID] = gs
	return gs
}

func (g *GuildState) playSound(channelID, url string) (*pb.TrackData, error) {
	var ms *MusicTrack

	if !urlRx.MatchString(url) {
		url = "ytsearch:" + url
	}

	if g.Queue == nil {
		g.newQueue(channelID)
	}

	ms, err := g.Queue.newMusicTrack(url)
	if err != nil {
		return nil, errors.Wrap(err, "create music track")
	}

	g.Queue.add(ms)
	if err != nil {
		return nil, errors.Wrap(err, "add track to Queue")
	}
	return ms.TrackData, nil
}

func (g *GuildState) stop() error {
	if len(g.Queue.Tracks) < 1 {
		return errors.New("Nothing playing")
	}
	g.Queue.Tracks[0].Stop <- struct{}{}
	return nil
}

func (g *GuildState) seek(pos time.Duration) error {
	if len(g.Queue.Tracks) < 1 {
		return errors.New("Nothing playing")
	}
	err := g.Queue.Tracks[0].seek(pos)
	return errors.Wrap(err, "seek")
}
