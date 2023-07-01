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

type musicStream interface {
	stop()
	seek(time.Duration)
	sendToVC(chan struct{}, *discordgo.VoiceConnection)
}

type CommandHandler func(*zap.SugaredLogger, *discordgo.Session, *discordgo.InteractionCreate) error

type mediaFormat string

type musicStreamData struct {
	streamURL string
	format    mediaFormat
	title     string
}

type streamExtractor struct {
	key       func(string) bool
	extractor func(*zap.SugaredLogger, string) (*musicStreamData, error)
}

var (
	mediaFormatWebmOpus mediaFormat = "webm,opus"
	mediaFormatMp4      mediaFormat = "mp4"

	streamExtractors = []streamExtractor{
		{
			key:       ytKey,
			extractor: ytExtractor,
		},
	}

	currentms musicStream
)

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, message string) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	})
}

func constructMusicStream(format mediaFormat, stream io.ReadSeeker) (musicStream, error) {
	switch format {
	case mediaFormatWebmOpus:
		return newWebmMusicStream(stream)
	case mediaFormatMp4:
		return newMp4MusicStream(stream)
	}
	return nil, fmt.Errorf("format %s does not have media stream constructor", format)
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

	var data *musicStreamData
	for _, extractor := range streamExtractors {
		if extractor.key(url) {
			data, err = extractor.extractor(logger, url)
			if err != nil {
				return errors.Wrap(err, "extract stream url")
			}
			break
		}
	}
	if data == nil {
		return fmt.Errorf("URL \"%s\" does not match any extractors", url)
	}

	err = vc.Speaking(true)
	if err != nil {
		return errors.Wrap(err, "set speaking state")
	}
	defer func(vc *discordgo.VoiceConnection, b bool) {
		err := vc.Speaking(b)
		if err != nil {
			logger.Error("Error setting speaking state: ", err)
		}
	}(vc, false)

	err = respond(s, i, fmt.Sprintf("playing %s", data.title))
	if err != nil {
		logger.Error("Error responding to play command: ", err)
	}

	stream, err := streamFromURL(data.streamURL)
	if err != nil {
		return errors.Wrap(err, "get stream")
	}
	logger.Debug(stream)

	done := make(chan struct{}, 1)

	ms, err := constructMusicStream(data.format, stream)
	if err != nil {
		return errors.Wrap(err, "create music stream")
	}
	currentms = ms

	go ms.sendToVC(done, vc)

	<-done

	return nil
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
			if err != nil {
				return errors.Wrap(err, "play sound")
			}
			break
		}
	}
	return nil
}

func seekHandler(logger *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	options := i.ApplicationCommandData().Options
	if len(options) < 1 {
		return errors.New("missing seek point")
	}
	tcF := options[0].Value.(float64)
	tc := time.Duration(tcF)
	logger.Infof("seek to %ds", tc)
	currentms.seek(time.Second * tc)
	return respond(s, i, fmt.Sprintf("seek to %ds", tc))
}

func stopHandler(logger *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	logger.Info("Stop current track")
	currentms.stop()
	return respond(s, i, "stopped")
}

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "test",
		Description: "testing command",
	},
	{
		Name:        "rcplay",
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
		Name:        "stop",
		Description: "stop current track",
	},
}

var commandHandlers = map[string]CommandHandler{
	"test": func(_ *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
		return respond(s, i, "test command pog")
	},
	"rcplay": playHandler,
	"seek":   seekHandler,
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
		delete(registeredCommandsMap, v.Name)
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
