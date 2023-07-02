package main

import (
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"os"
	"os/signal"
	"syscall"
)

var minVolume = 0.1
var commands = []*discordgo.ApplicationCommand{
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

	state := newState(logger, rcb)

	rcb.AddHandler(state.readyHandlerWrapper())
	rcb.AddHandler(state.interactionCreateHandlerWrapper())

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
