package models

import (
	"context"
	"sync"

	"ProjectOrca/extractor"

	"ProjectOrca/store"

	"github.com/bwmarrin/discordgo"
	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"
	"github.com/sovamorco/errorx"
)

type Bot struct {
	// constant values
	store      *store.Store
	extractors *extractor.Extractors
	session    *discordgo.Session

	// concurrency-safe
	guilds sync.Map
}

func NewBot(
	store *store.Store, extractors *extractor.Extractors, token string,
) (*Bot, error) {
	b := Bot{
		store:      store,
		extractors: extractors,
		session:    nil,
		guilds:     sync.Map{},
	}

	sess, err := startSession(token)
	if err != nil {
		return nil, errorx.Decorate(err, "start session")
	}

	b.session = sess

	return &b, nil
}

func startSession(token string) (*discordgo.Session, error) {
	s, err := discordgo.New(token)
	if err != nil {
		return nil, errorx.Decorate(err, "create new session")
	}

	s.Identify.Intents = discordgo.IntentGuildVoiceStates
	s.ShouldReconnectVoiceOnSessionError = false

	err = s.Open()
	if err != nil {
		return nil, errorx.Decorate(err, "open session")
	}

	return s, nil
}

func (b *Bot) Shutdown(ctx context.Context) {
	logger := zerolog.Ctx(ctx)

	var wg sync.WaitGroup

	b.guilds.Range(func(_, value any) bool {
		wg.Add(1)

		go func() {
			defer wg.Done()

			guild, _ := value.(*Guild)

			guild.gracefulShutdown(ctx)
		}()

		return true
	})

	wg.Wait()

	err := b.session.Close()
	if err != nil {
		logger.Error().Err(err).Msg("Error closing session")
	}
}

func (b *Bot) GetID() string {
	return b.session.State.User.ID
}

func (b *Bot) GetToken() string {
	return b.session.Identify.Token
}

func (b *Bot) FullResync(ctx context.Context) {
	logger := zerolog.Ctx(ctx)

	err := b.ResyncGuilds(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Error resyncing guilds")

		return
	}
}

func (b *Bot) resyncGuild(ctx context.Context, guild *RemoteGuild) error {
	local := b.getGuild(ctx, guild.ID)

	err := local.connect(ctx, guild.ChannelID)
	if err != nil {
		return errorx.Decorate(err, "error connecting to voice channel")
	}

	// make sure this does not block
	select {
	case local.resync <- struct{}{}:
	default:
	}

	if guild.Paused {
		// try to consume from playing
		select {
		case <-local.playing:
		default:
		}
	} else {
		// try to fill playing
		select {
		case local.playing <- struct{}{}:
		default:
		}
	}

	return nil
}

func (b *Bot) ResyncGuild(ctx context.Context, guildID string) error {
	var guild RemoteGuild
	guild.BotID = b.GetID()
	guild.ID = guildID

	err := b.store.
		NewSelect().
		Model(&guild).
		WherePK().
		Scan(ctx)
	if err != nil {
		return errorx.Decorate(err, "get guild from store")
	}

	return b.resyncGuild(ctx, &guild)
}

func (b *Bot) ResyncGuilds(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)

	var guilds []*RemoteGuild

	err := b.store.
		NewSelect().
		Model(&guilds).
		Where("bot_id = ?", b.GetID()).
		Scan(ctx)
	if err != nil {
		return errorx.Decorate(err, "get guilds from store")
	}

	var eg multierror.Group

	for _, guild := range guilds {
		logger := logger.With().Str("guildID", guild.ID).Logger()
		ctx := logger.WithContext(ctx)

		eg.Go(func() error {
			return b.resyncGuild(ctx, guild)
		})
	}

	err = eg.Wait().ErrorOrNil()
	if err != nil {
		return errorx.Decorate(err, "resync guilds")
	}

	return nil
}

func (b *Bot) ResyncGuildTrack(ctx context.Context, guildID string) {
	local := b.getGuild(ctx, guildID)
	local.ResyncPlaying()
}

func (b *Bot) getGuild(ctx context.Context, guildID string) *Guild {
	local, ok := b.guilds.Load(guildID)
	if !ok {
		local = NewGuild(ctx, guildID, b.GetID(), b.session, b.store, b.extractors)
		b.guilds.Store(guildID, local)
	}

	guild, _ := local.(*Guild)

	return guild
}
