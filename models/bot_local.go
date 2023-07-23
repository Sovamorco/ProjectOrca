package models

import (
	"context"
	"sync"
	"time"

	"ProjectOrca/extractor"

	"ProjectOrca/store"

	"github.com/bwmarrin/discordgo"
	"github.com/hashicorp/go-multierror"
	"github.com/joomcode/errorx"
	"go.uber.org/zap"
)

type Bot struct {
	// constant values
	logger     *zap.SugaredLogger
	store      *store.Store
	extractors *extractor.Extractors
	session    *discordgo.Session

	// potentially changeable lockable values
	guilds   map[string]*Guild
	guildsMu sync.Mutex `exhaustruct:"optional"`
}

func NewBot(
	logger *zap.SugaredLogger, store *store.Store, extractors *extractor.Extractors, token string,
) (*Bot, error) {
	b := Bot{
		logger:     logger.Named("bot"),
		store:      store,
		extractors: extractors,
		session:    nil,
		guilds:     make(map[string]*Guild),
	}

	sess, err := startSession(token)
	if err != nil {
		return nil, errorx.Decorate(err, "start session")
	}

	b.session = sess
	b.logger = b.logger.With("bot_id", b.GetID())

	return &b, nil
}

func startSession(token string) (*discordgo.Session, error) {
	s, err := discordgo.New(token)
	if err != nil {
		return nil, errorx.Decorate(err, "create new session")
	}

	s.Identify.Intents = discordgo.IntentGuildVoiceStates

	err = s.Open()
	if err != nil {
		return nil, errorx.Decorate(err, "open session")
	}

	return s, nil
}

func (b *Bot) GracefulShutdown() {
	var wg sync.WaitGroup

	b.guildsMu.Lock()
	for _, guild := range b.guilds {
		guild := guild

		wg.Add(1)

		go func() {
			defer wg.Done()

			guild.gracefulShutdown()
		}()
	}
	b.guildsMu.Unlock()

	wg.Wait()

	err := b.session.Close()
	if err != nil {
		b.logger.Errorf("Error closing session: %+v", err)
	}
}

func (b *Bot) GetID() string {
	return b.session.State.User.ID
}

func (b *Bot) GetToken() string {
	return b.session.Identify.Token
}

func (b *Bot) FullResync(ctx context.Context) {
	err := b.ResyncGuilds(ctx)
	if err != nil {
		b.logger.Errorf("Error resyncing guilds: %+v", err)

		return
	}
}

func (b *Bot) resyncGuild(ctx context.Context, guild *RemoteGuild) error {
	local := b.getGuild(ctx, guild.ID)

	err := local.connect(guild.ChannelID)
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
		guild := guild

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

func (b *Bot) ResyncGuildTrack(ctx context.Context, guildID string, seekPos time.Duration) {
	local := b.getGuild(ctx, guildID)
	local.ResyncPlaying()
}

func (b *Bot) getGuild(ctx context.Context, guildID string) *Guild {
	b.guildsMu.Lock()
	defer b.guildsMu.Unlock()

	local, ok := b.guilds[guildID]
	if !ok {
		local = NewGuild(ctx, guildID, b.GetID(), b.session, b.logger, b.store, b.extractors)
		b.guilds[guildID] = local
	}

	return local
}
