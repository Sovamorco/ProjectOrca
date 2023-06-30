package main

import (
	"fmt"
	"github.com/jonas747/dca"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/kkdai/youtube/v2"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var AudioQuality = map[string]int{
	"AUDIO_QUALITY_LOW":    1,
	"AUDIO_QUALITY_MEDIUM": 2,
	"AUDIO_QUALITY_HIGH":   3,
}

type CommandHandler func(*zap.SugaredLogger, *discordgo.Session, *discordgo.InteractionCreate) error

var ytClient = youtube.Client{}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, message string) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	})
}

func selectAudioFormat(logger *zap.SugaredLogger, video *youtube.Video) (*youtube.Format, error) {
	var selected *youtube.Format
	selectedPriority := 0
	for _, format := range video.Formats.WithAudioChannels() {
		format := format
		priority := 0
		// we would prefer not to convert sample rate
		if format.AudioSampleRate != "48000" {
			priority--
		}
		// we would prefer not to convert codec
		if !strings.Contains(format.MimeType, "opus") {
			priority--
		}
		// we would really prefer not to have to download video
		if strings.Contains(format.MimeType, "video/") {
			priority -= 5
		}
		if selected == nil {
			selected = &format
			selectedPriority = priority
			continue
		}
		formatAudioQuality, ok := AudioQuality[format.AudioQuality]
		if !ok {
			formatAudioQuality = 0
		}
		selectedAudioQuality, ok := AudioQuality[selected.AudioQuality]
		if !ok {
			selectedAudioQuality = 0
		}
		// we value audio quality a lot
		if formatAudioQuality > selectedAudioQuality {
			priority += 2
		}
		if priority > selectedPriority {
			selected = &format
			selectedPriority = priority
		}
	}
	if selected == nil {
		return nil, errors.New("failed to find selected format")
	}

	logger.Infof("selected: %s, %s", selected.MimeType, selected.AudioSampleRate)

	return selected, nil
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

	video, err := ytClient.GetVideo(url)
	if err != nil {
		return errors.Wrap(err, "get video")
	}
	format, err := selectAudioFormat(logger, video)
	if err != nil {
		return errors.Wrap(err, "select audio format")
	}

	options := dca.StdEncodeOptions
	options.RawOutput = true
	options.Bitrate = 96
	options.Application = dca.AudioApplicationAudio
	sess, err := dca.EncodeFile(format.URL, options)
	if err != nil {
		return errors.Wrap(err, "encode to dca")
	}
	defer sess.Cleanup()

	done := make(chan error)
	dca.NewStream(sess, vc, done)

	err = respond(s, i, fmt.Sprintf("playing %s by %s", video.Title, video.Author))
	if err != nil {
		logger.Error("Error responding to play command: ", err)
	}

	err = <-done
	if !errors.Is(err, io.EOF) {
		return errors.Wrap(err, "stream audio")
	}
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
}

var commandHandlers = map[string]CommandHandler{
	"test": func(_ *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
		return respond(s, i, "test command pog")
	},
	"rcplay": playHandler,
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
