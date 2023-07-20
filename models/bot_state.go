package models

import (
	"context"

	"ProjectOrca/store"

	"github.com/bwmarrin/discordgo"
	"github.com/joomcode/errorx"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

type BotState struct {
	bun.BaseModel `bun:"table:bots" exhaustruct:"optional"`

	// -- private non-stored values
	logger  *zap.SugaredLogger
	session *discordgo.Session
	store   *store.Store
	// -- end private non-stored values

	Guilds        []*GuildState `bun:"rel:has-many,join:id=bot_id"`
	ID            string        `bun:",pk"`
	StateToken    string
	Token         string
	Locker        string
	LockerAddress string
}

func NewState(
	ctx context.Context,
	logger *zap.SugaredLogger,
	store *store.Store,
	token,
	stateToken,
	locker,
	lockerAddress string,
) (*BotState, error) {
	s := BotState{
		logger:        logger,
		session:       nil,
		store:         store,
		Guilds:        make([]*GuildState, 0),
		ID:            "", // is set by CreateSession
		StateToken:    stateToken,
		Token:         token,
		Locker:        locker,
		LockerAddress: lockerAddress,
	}

	session, err := s.CreateSession()
	if err != nil {
		return nil, errorx.Decorate(err, "create session")
	}

	s.session = session
	logger = logger.Named("bot_state")
	logger.Infof("Started the bot")

	_, err = store.NewInsert().Model(&s).Exec(ctx)
	if err != nil {
		s.GracefulShutdown() // shutdown state if we failed to store it

		return nil, errorx.Decorate(err, "store state")
	}

	return &s, nil
}

func (s *BotState) CreateSession() (*discordgo.Session, error) {
	session, err := discordgo.New(s.Token)
	if err != nil {
		return nil, errorx.Decorate(err, "start new session")
	}

	session.Identify.Intents = discordgo.IntentGuildVoiceStates

	err = session.Open()
	if err != nil {
		return nil, errorx.Decorate(err, "open session")
	}

	s.ID = session.State.User.ID
	s.logger = s.logger.With("bot_id", s.ID)
	s.logger.Infof("Started bot")

	return session, nil
}

// Restore restores the state from database stored values.
func (s *BotState) Restore(ctx context.Context, logger *zap.SugaredLogger, store *store.Store) error {
	logger = logger.Named("bot_state")

	logger.Infow("Restoring bot state", "bot_id", s.ID)

	s.logger = logger
	s.store = store

	session, err := s.CreateSession()
	if err != nil {
		return errorx.Decorate(err, "create session")
	}

	s.session = session

	for _, gs := range s.Guilds {
		gs.Restore(ctx, s)
	}

	return nil
}

func (s *BotState) GracefulShutdown() {
	var err error

	for _, gs := range s.Guilds {
		gs.gracefulShutdown()
	}

	err = s.session.Close()
	if err != nil {
		s.logger.Errorf("Error closing session: %+v", err)
	}
}

func (s *BotState) SetGuildState(guildID string, gs *GuildState) {
	for i := range s.Guilds {
		if s.Guilds[i].GuildID == guildID {
			s.Guilds[i] = gs

			return
		}
	}

	s.Guilds = append(s.Guilds, gs)
}

func (s *BotState) GetGuildState(guildID string) *GuildState {
	for _, gs := range s.Guilds {
		if gs.GuildID == guildID {
			return gs
		}
	}

	return nil
}

func (s *BotState) GetOrCreateGuildState(ctx context.Context, guildID string) (*GuildState, error) {
	gs := s.GetGuildState(guildID)
	if gs != nil {
		return gs, nil
	}

	gs, err := s.NewGuildState(ctx, guildID)
	if err != nil {
		return nil, errorx.Decorate(err, "create guild state")
	}

	return gs, nil
}
