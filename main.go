package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type CommandHandler func(*zap.SugaredLogger, *discordgo.Session, *discordgo.InteractionCreate) error

func empty[T any](c chan T) {
	for len(c) != 0 {
		select {
		case <-c:
		default:
		}
	}
}

func respondAck(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if re, ok := err.(*discordgo.RESTError); ok {
		if re.Message.Code == discordgo.ErrCodeInteractionHasAlreadyBeenAcknowledged {
			err = nil
		}
	}
	return err
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, message string) error {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	})
	if re, ok := err.(*discordgo.RESTError); ok {
		if re.Message.Code == discordgo.ErrCodeInteractionHasAlreadyBeenAcknowledged {
			_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &message,
			})
		}
	}
	return err
}

func playSound(logger *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate, guildID, channelID, url string) error {
	vc, err := s.ChannelVoiceJoin(guildID, channelID, false, true)
	if err != nil {
		return errors.Wrap(err, "join voice channel")
	}
	defer func(vc *discordgo.VoiceConnection) {
		err := vc.Disconnect()
		if err != nil {
			logger.Error("Error leaving voice channel: ", err)
		}
	}(vc)

	err = respondAck(s, i)
	if err != nil {
		logger.Error("Error responding with ack to play command: ", err)
	}

	ms, err := newMusicTrack(logger, url)
	if err != nil {
		return errors.Wrap(err, "create music track")
	}
	currentms = ms

	err = respond(s, i, fmt.Sprintf("playing %s by %s", ms.videoData.Title, ms.videoData.Channel))
	if err != nil {
		logger.Error("Error responding to play command: ", err)
	}

	done := make(chan error, 1)

	go ms.streamToVC(vc, done)

	err = <-done
	return errors.Wrap(err, "stream to vc")
}

func playHandler(logger *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	options := i.ApplicationCommandData().Options
	if len(options) < 1 {
		return errors.New("missing url")
	}
	url := options[0].Value.(string)
	g, err := s.State.Guild(i.GuildID)
	if err != nil {
		return errors.Wrap(err, "find guild")
	}

	// Look for the message sender in that guild's current voice states.
	for _, vs := range g.VoiceStates {
		if vs.UserID == i.Member.User.ID {
			err = playSound(logger, s, i, g.ID, vs.ChannelID, url)
			return errors.Wrap(err, "play sound")
		}
	}
	return respond(s, i, "user not in voice channel")
}

func seekHandler(logger *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	options := i.ApplicationCommandData().Options
	if len(options) < 1 {
		return errors.New("missing seek point")
	}
	tcF := options[0].Value.(float64)
	tc := time.Duration(tcF)
	logger.Infof("seek to %ds", tc)
	err := currentms.seek(time.Second * tc)
	if err != nil {
		return errors.Wrap(err, "seek")
	}
	return respond(s, i, fmt.Sprintf("seek to %ds", tc))
}

func volumeHandler(logger *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	options := i.ApplicationCommandData().Options
	if len(options) < 1 {
		return errors.New("missing volume")
	}
	volume := options[0].Value.(float64)
	logger.Infof("set volume to %f%%", volume)
	currentms.targetVolume = float32(volume / 100)
	return respond(s, i, fmt.Sprintf("set volume to %.2f%%", volume))
}

func stopHandler(logger *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	logger.Info("Stop current track")
	currentms.stop <- struct{}{}
	return respond(s, i, "stopped")
}

var minVolume = 0.1
var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "test",
		Description: "testing command",
	},
	{
		Name:        "play",
		Description: "play music",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "url",
				Description: "URL of whatever to play",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    true,
			},
		},
	},
	{
		Name:        "seek",
		Description: "seek current track",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "time",
				Description: "time to seek to",
				Type:        discordgo.ApplicationCommandOptionInteger,
				Required:    true,
			},
		},
	},
	{
		Name:        "volume",
		Description: "change volume",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "volume",
				Description: "volume in percent to scale to",
				Type:        discordgo.ApplicationCommandOptionNumber,
				Required:    true,
				MaxValue:    100,
				MinValue:    &minVolume,
			},
		},
	},
	{
		Name:        "stop",
		Description: "stop current track",
	},
}

var commandHandlers = map[string]CommandHandler{
	"test": func(_ *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
		return respond(s, i, "test command pog")
	},
	"play":   playHandler,
	"seek":   seekHandler,
	"volume": volumeHandler,
	"stop":   stopHandler,
}

func readyHandlerWrapper(logger *zap.SugaredLogger) func(s *discordgo.Session, _ *discordgo.Ready) {
	return func(s *discordgo.Session, _ *discordgo.Ready) {
		logger.Info("Ready handler called")
		err := s.UpdateGameStatus(0, "testing")
		if err != nil {
			logger.Error("Error updating status: ", err)
		}
	}
}

func interactionCreateHandlerWrapper(logger *zap.SugaredLogger) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		name := i.ApplicationCommandData().Name
		if handler, ok := commandHandlers[name]; ok {
			err := handler(logger, s, i)
			if err != nil {
				logger.Errorf("Error handling \"%s\" interaction: %v", name, err)
			}
		}
	}
}

func main() {
	coreLogger, err := zap.NewDevelopment(zap.IncreaseLevel(zapcore.DebugLevel))
	if err != nil {
		panic(err)
	}
	logger := coreLogger.Sugar()

	// temporary
	tf, err := os.Open(".token")
	if err != nil {
		panic(err)
	}
	token, err := io.ReadAll(tf)
	if err != nil {
		panic(err)
	}

	rcb, err := discordgo.New("Bot " + string(token))
	if err != nil {
		panic(err)
	}

	rcb.AddHandler(readyHandlerWrapper(logger))
	rcb.AddHandler(interactionCreateHandlerWrapper(logger))

	rcb.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates
	err = rcb.Open()
	if err != nil {
		panic(err)
	}

	defer func() {
		err := rcb.Close()
		if err != nil {
			logger.Error("Error closing bot: ", err)
		}
	}()

	registeredCommands, err := rcb.ApplicationCommands(rcb.State.User.ID, "")
	if err != nil {
		panic(err)
	}
	registeredCommandsMap := make(map[string]*discordgo.ApplicationCommand, len(registeredCommands))
	for _, registeredCommand := range registeredCommands {
		registeredCommandsMap[registeredCommand.Name] = registeredCommand
	}

	for _, v := range commands {
		_, err = rcb.ApplicationCommandCreate(rcb.State.User.ID, "", v)
		if err != nil {
			panic(err)
		}
		if _, ok := registeredCommandsMap[v.Name]; ok {
			delete(registeredCommandsMap, v.Name)
		} else {
			logger.Info("Adding new command: ", v.Name)
		}
	}
	for name, oldCommand := range registeredCommandsMap {
		logger.Info("Removing old command: ", name)
		err = rcb.ApplicationCommandDelete(rcb.State.User.ID, "", oldCommand.ID)
		if err != nil {
			panic(err)
		}
	}

	registeredCommands, err = rcb.ApplicationCommands(rcb.State.User.ID, "")
	if err != nil {
		panic(err)
	}
	logger.Infof("Started the bot with %d registered commands", len(registeredCommands))
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
