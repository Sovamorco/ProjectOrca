package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"reflect"
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
	Guild        *discordgo.Guild
	Queue        *Queue
	Volume       float32
	TargetVolume float32
}

func (s *State) newGuildState(guildID string) (*GuildState, error) {
	guild, err := s.Session.State.Guild(guildID)
	if err != nil {
		return nil, err
	}
	g := &GuildState{
		State:        s,
		Guild:        guild,
		Volume:       1,
		TargetVolume: 1,
	}
	return g, nil
}

func (s *State) readyHandlerWrapper() func(s *discordgo.Session, _ *discordgo.Ready) {
	return func(session *discordgo.Session, _ *discordgo.Ready) {
		s.Logger.Info("Ready handler called")
		err := session.UpdateGameStatus(0, "testing")
		if err != nil {
			s.Logger.Error("Error updating status: ", err)
		}
	}
}

func (s *State) interactionApplicationCommandHandler(_ *discordgo.Session, i *discordgo.InteractionCreate) {
	var err error
	g, ok := s.Guilds[i.GuildID]
	if !ok {
		g, err = s.newGuildState(i.GuildID)
		if err != nil {
			s.Logger.Errorf("Error creating guild state for guild %s: %v", i.GuildID, err)
			return
		}
		s.Guilds[g.Guild.ID] = g
	}
	name := i.ApplicationCommandData().Name
	err = g.handle(name, i)
	if err != nil {
		s.Logger.Errorf("Error handling \"%s\" interaction: %v", name, err)
		return
	}
}

func (s *State) interactionCreateHandlerWrapper() func(session *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(session *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			s.interactionApplicationCommandHandler(session, i)
		default:
			s.Logger.Errorf("Unknown interaction type: %s (%s)", i.Type.String(), reflect.TypeOf(i.Data).Name())
		}
	}
}

func (g *GuildState) handle(name string, i *discordgo.InteractionCreate) error {
	switch name {
	case "play":
		return g.playHandler(i)
	case "seek":
		return g.seekHandler(i)
	case "volume":
		return g.volumeHandler(i)
	case "stop":
		return g.stopHandler(i)
	}
	return fmt.Errorf("command not found: %s", name)
}

func (g *GuildState) playHandler(i *discordgo.InteractionCreate) error {
	options := i.ApplicationCommandData().Options
	if len(options) < 1 {
		return errors.New("missing url")
	}
	url := options[0].Value.(string)

	// Look for the message sender in that guild's current voice states.
	for _, vs := range g.Guild.VoiceStates {
		if vs.UserID == i.Member.User.ID {
			err := g.playSound(i, vs.ChannelID, url)
			return errors.Wrap(err, "play sound")
		}
	}
	return g.respondSimpleMessage(i, "user not in voice channel")
}

func (g *GuildState) seekHandler(i *discordgo.InteractionCreate) error {
	options := i.ApplicationCommandData().Options
	if len(options) < 1 {
		return errors.New("missing seek point")
	}
	tcF := options[0].Value.(float64)
	tc := time.Duration(tcF)
	g.Logger.Infof("seek to %ds", tc)
	err := g.Queue.Tracks[0].seek(time.Second * tc)
	if err != nil {
		return errors.Wrap(err, "seek")
	}
	return g.respondSimpleMessage(i, fmt.Sprintf("seek to %ds", tc))
}

func (g *GuildState) volumeHandler(i *discordgo.InteractionCreate) error {
	options := i.ApplicationCommandData().Options
	if len(options) < 1 {
		return errors.New("missing volume")
	}
	volume := options[0].Value.(float64)
	g.Logger.Infof("set volume to %f%%", volume)
	g.TargetVolume = float32(volume / 100)
	return g.respondSimpleMessage(i, fmt.Sprintf("set volume to %.2f%%", volume))
}

func (g *GuildState) stopHandler(i *discordgo.InteractionCreate) error {
	g.Logger.Info("Stop current track")
	g.Queue.Tracks[0].Stop <- struct{}{}
	return g.respondSimpleMessage(i, "stopped")
}

func (g *GuildState) playSound(i *discordgo.InteractionCreate, channelID, url string) error {
	err := g.respondAck(i)
	if err != nil {
		g.Logger.Error("Error responding with ack to play command: ", err)
	}

	var ms *MusicTrack

	if !urlRx.MatchString(url) {
		url = "ytsearch:" + url
	}

	if g.Queue == nil {
		g.newQueue(channelID)
	}

	ms, err = g.Queue.newMusicTrack(url)
	if err != nil {
		return errors.Wrap(err, "create music track")
	}

	color := getEmbedColor(ms.TrackData.originalURL)
	err = g.respondColoredEmbed(i, color, "Added track", fmt.Sprintf("[%s](%s)", ms.TrackData.title, ms.TrackData.originalURL))
	if err != nil {
		g.Logger.Error("Error responding to play command: ", err)
	}

	g.Queue.add(ms)
	return errors.Wrap(err, "add track to Queue")
}
