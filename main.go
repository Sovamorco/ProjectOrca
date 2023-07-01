package main

import (
	"bufio"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/ogg"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type musicSession struct {
	sync.Mutex
	logger    *zap.SugaredLogger
	cmd       *exec.Cmd
	stream    io.ReadCloser
	streamURL string
	decoder   *ogg.PacketDecoder
	stop      chan struct{}
}

type CommandHandler func(*zap.SugaredLogger, *discordgo.Session, *discordgo.InteractionCreate) error

var currentms *musicSession

func (ms *musicSession) seek(pos time.Duration) error {
	cmd, stream, err := getStream(ms.streamURL, pos)
	if err != nil {
		return errors.Wrap(err, "get stream")
	}
	ms.Lock()
	err = ms.stream.Close()
	if err != nil {
		ms.logger.Error("Error closing old stream: ", err)
	}
	ms.stream = stream
	ms.cmd = cmd
	ms.decoder = ogg.NewPacketDecoder(ogg.NewDecoder(stream))
	ms.Unlock()
	return nil
}

func empty[T any](c chan T) {
	for len(c) != 0 {
		select {
		case <-c:
		default:
		}
	}
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, message string) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	})
}

func getStreamURL(url string) (string, error) {
	ytdlpArgs := []string{
		"-f", "ba[acodec=opus]/ba/ba*[acodec=opus]/ba*",
		"--get-url",
		url,
	}
	ytdlp := exec.Command("yt-dlp", ytdlpArgs...)
	stdout, err := ytdlp.StdoutPipe()
	if err != nil {
		return "", errors.Wrap(err, "get ytdlp stdout pipe")
	}
	err = ytdlp.Start()
	if err != nil {
		return "", errors.Wrap(err, "start ytdlp")
	}
	streamURLB, err := io.ReadAll(stdout)
	if err != nil {
		return "", errors.Wrap(err, "read stream url")
	}
	err = ytdlp.Wait()
	if err != nil {
		return "", errors.Wrap(err, "wait for ytdlp")
	}
	return strings.TrimSpace(string(streamURLB)), nil
}

func getStream(streamURL string, pos time.Duration) (*exec.Cmd, io.ReadCloser, error) {
	ffmpegArgs := []string{
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "2",
	}
	if !strings.Contains(streamURL, ".m3u8") {
		ffmpegArgs = append(ffmpegArgs,
			"-reconnect_at_eof", "1",
			"-ss", fmt.Sprintf("%f", pos.Seconds()),
		)
	}
	ffmpegArgs = append(ffmpegArgs,
		"-i", streamURL,
		"-map", "0:a",
		"-acodec", "libopus",
		"-f", "ogg",
		"-ar", "48000",
		"-ac", "2",
		"-frame_duration", "20",
		"-packet_loss", "1",
		"pipe:1",
	)
	ffmpeg := exec.Command("ffmpeg", ffmpegArgs...)
	stdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		return nil, nil, errors.Wrap(err, "get ffmpeg stdout pipe")
	}
	// make a.. BufferedReadCloser I guess?
	buf := struct {
		io.Reader
		io.Closer
	}{
		bufio.NewReader(stdout),
		stdout,
	}
	err = ffmpeg.Start()
	if err != nil {
		return nil, nil, errors.Wrap(err, "start ffmpeg process")
	}
	return ffmpeg, buf, nil
}

func (ms *musicSession) streamToVC(vc *discordgo.VoiceConnection, done chan error) {
	defer close(done)
	defer func(stream io.ReadCloser) {
		err := stream.Close()
		if err != nil && !errors.Is(err, os.ErrClosed) {
			ms.logger.Error("Error closing stream: ", err)
		}
	}(ms.stream)
	for {
		ms.Lock()
		packet, _, err := ms.decoder.Decode()
		ms.Unlock()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				done <- err
			}
			return
		}
		select {
		case <-ms.stop:
			empty(ms.stop)
			return
		case vc.OpusSend <- packet:
		}
	}
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

	streamURL, err := getStreamURL(url)
	if err != nil {
		return errors.Wrap(err, "get stream url")
	}
	cmd, stream, err := getStream(streamURL, 0)
	if err != nil {
		return errors.Wrap(err, "get stream")
	}
	ms := musicSession{
		logger:    logger,
		cmd:       cmd,
		stream:    stream,
		streamURL: streamURL,
		decoder:   ogg.NewPacketDecoder(ogg.NewDecoder(stream)),
		stop:      make(chan struct{}),
	}
	currentms = &ms

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
	err := currentms.seek(time.Second * tc)
	if err != nil {
		return errors.Wrap(err, "seek")
	}
	return respond(s, i, fmt.Sprintf("seek to %ds", tc))
}

func stopHandler(logger *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	logger.Info("Stop current track")
	currentms.stop <- struct{}{}
	return respond(s, i, "stopped")
}

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
		Name:        "stop",
		Description: "stop current track",
	},
}

var commandHandlers = map[string]CommandHandler{
	"test": func(_ *zap.SugaredLogger, s *discordgo.Session, i *discordgo.InteractionCreate) error {
		return respond(s, i, "test command pog")
	},
	"play": playHandler,
	"seek": seekHandler,
	"stop": stopHandler,
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
