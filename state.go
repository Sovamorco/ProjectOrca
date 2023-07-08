package main

import (
	pb "ProjectOrca/proto"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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

//func (g *GuildState) seekHandler(i *discordgo.InteractionCreate) error {
//	options := i.ApplicationCommandData().Options
//	if len(options) < 1 {
//		return errors.New("missing seek point")
//	}
//	tcF := options[0].Value.(float64)
//	tc := time.Duration(tcF)
//	g.Logger.Infof("seek to %ds", tc)
//	err := g.Queue.Tracks[0].seek(time.Second * tc)
//	if err != nil {
//		return errors.Wrap(err, "seek")
//	}
//	return g.respondSimpleMessage(i, fmt.Sprintf("seek to %ds", tc))
//}
//
//func (g *GuildState) volumeHandler(i *discordgo.InteractionCreate) error {
//	options := i.ApplicationCommandData().Options
//	if len(options) < 1 {
//		return errors.New("missing volume")
//	}
//	volume := options[0].Value.(float64)
//	g.Logger.Infof("set volume to %f%%", volume)
//	g.TargetVolume = float32(volume / 100)
//	return g.respondSimpleMessage(i, fmt.Sprintf("set volume to %.2f%%", volume))
//}
//
//func (g *GuildState) stopHandler(i *discordgo.InteractionCreate) error {
//	g.Logger.Info("Stop current track")
//	g.Queue.Tracks[0].Stop <- struct{}{}
//	return g.respondSimpleMessage(i, "stopped")
//}

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
