package main

import (
	"bufio"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/pion/opus/pkg/oggreader"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

type CommandHandler func(*zap.SugaredLogger, *discordgo.Session, *discordgo.InteractionCreate) error

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, message string) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	})
}

func getYTDLPStream(url string) (*exec.Cmd, io.Reader, error) {
	ffmpegArgs := []string{
		"-acodec",
		"libopus",
		"-f",
		"ogg",
		"-vbr",
		"on",
		"-ar",
		"48000",
		"-ac",
		"2",
		"-b:a",
		"64000",
		"-application",
		"audio",
		"-frame_duration",
		"20",
		"-packet_loss",
		"1",
	}
	cmd := []string{
		"-x",
		"--audio-format",
		"opus",
		"--postprocessor-args",
		"ffmpeg:" + strings.Join(ffmpegArgs, " "),
		"-o",
		"-",
		url,
	}
	ytdlp := exec.Command("yt-dlp", cmd...)
	stdout, err := ytdlp.StdoutPipe()
	if err != nil {
		return nil, nil, errors.Wrap(err, "get stdout pipe")
	}
	buf := bufio.NewReader(stdout)
	err = ytdlp.Start()
	if err != nil {
		return nil, nil, errors.Wrap(err, "start ytdlp process")
	}
	return ytdlp, buf, nil
}

func streamToVC(reader io.Reader, vc *discordgo.VoiceConnection, done chan error) error {
	r, _, err := oggreader.NewWith(reader)
	if err != nil {
		return errors.Wrap(err, "create oggreader")
	}

	go func() {
		defer close(done)
		for {
			page, _, err := r.ParseNextPage()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					done <- err
				}
				return
			}
			for _, packet := range page {
				vc.OpusSend <- packet
			}
		}
	}()
	return nil
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

	err = respond(s, i, fmt.Sprintf("playing %s", "test"))
	if err != nil {
		logger.Error("Error responding to play command: ", err)
	}

	cmd, stream, err := getYTDLPStream(url)
	if err != nil {
		return errors.Wrap(err, "get ytdlp stream")
	}
	logger.Debug(cmd)

	done := make(chan error, 1)

	err = streamToVC(stream, vc, done)
	if err != nil {
		return errors.Wrap(err, "init stream to vc")
	}

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
			if err != nil {
				return errors.Wrap(err, "play sound")
			}
			break
		}
	}
	return nil
}

//func seekHandler(logger *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
//	options := i.ApplicationCommandData().Options
//	if len(options) < 1 {
//		return errors.New("missing seek point")
//	}
//	tcF := options[0].Value.(float64)
//	tc := time.Duration(tcF)
//	logger.Infof("seek to %ds", tc)
//	currentms.seek(time.Second * tc)
//	return respond(s, i, fmt.Sprintf("seek to %ds", tc))
//}
//
//func stopHandler(logger *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
//	logger.Info("Stop current track")
//	currentms.stop()
//	return respond(s, i, "stopped")
//}

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
	//"seek":   seekHandler,
	//"stop":   stopHandler,
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
