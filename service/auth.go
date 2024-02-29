package orca

import (
	"context"
	"database/sql"
	"errors"

	"ProjectOrca/models"

	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type (
	botKey   struct{}
	guildKey struct{}
)

func (o *Orca) authenticate(ctx context.Context) (context.Context, error) {
	token, err := o.parseIncomingContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse context")
	}

	var s models.RemoteBot

	err = o.store.
		NewSelect().
		Model(&s).
		Where("token = ?", token).
		Scan(ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, errorx.Decorate(err, "get state from store")
		}

		bot, ierr := o.register(ctx, token)
		if ierr != nil {
			return nil, errorx.Decorate(ierr, "register bot")
		}

		s = *bot
	}

	logger := zerolog.Ctx(ctx).With().Str("botID", s.ID).Logger()
	ctx = logger.WithContext(ctx)
	ctx = context.WithValue(ctx, botKey{}, &s)

	return ctx, nil
}

func (o *Orca) authenticateWithGuild(ctx context.Context, guildID string) (
	context.Context,
	error,
) {
	ctx, err := o.authenticate(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "authenticate bot")
	}

	bot, err := parseBotContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse bot context")
	}

	var r models.RemoteGuild
	r.ID = guildID
	r.BotID = bot.ID

	err = o.store.
		NewSelect().
		Model(&r).
		WherePK().
		Scan(ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, errorx.Decorate(err, "get guild from store")
		}

		g := models.NewRemoteGuild(bot.ID, guildID)

		_, err = o.store.
			NewInsert().
			Model(g).
			Exec(ctx)
		if err != nil {
			return nil, errorx.Decorate(err, "create guild")
		}

		r = *g
	}

	logger := zerolog.Ctx(ctx).With().Str("guildID", r.ID).Logger()
	ctx = logger.WithContext(ctx)
	ctx = context.WithValue(ctx, botKey{}, bot)
	ctx = context.WithValue(ctx, guildKey{}, &r)

	return ctx, nil
}

func (o *Orca) parseIncomingContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	tokenS := md.Get("token")
	if len(tokenS) != 1 {
		return "", status.Error(codes.Unauthenticated, "missing token")
	}

	token := tokenS[0]

	return token, nil
}

func (o *Orca) register(ctx context.Context, token string) (*models.RemoteBot, error) {
	state, err := models.NewBot(o.store, o.extractors, token)
	if err != nil {
		return nil, errorx.Decorate(err, "create local bot state")
	}

	r := models.NewRemoteBot(state.GetID(), state.GetToken())

	_, err = o.store.
		NewInsert().
		Model(r).
		On("CONFLICT (id) DO UPDATE").
		Set("token = EXCLUDED.token").
		Exec(ctx)
	if err != nil {
		state.Shutdown(ctx)

		return nil, errorx.Decorate(err, "store remote bot state")
	}

	o.states.Store(state.GetID(), state)

	return r, nil
}

func parseBotContext(ctx context.Context) (*models.RemoteBot, error) {
	bot, ok := ctx.Value(botKey{}).(*models.RemoteBot)
	if !ok {
		return nil, errorx.IllegalState.New("missing bot")
	}

	return bot, nil
}

func parseGuildContext(ctx context.Context) (*models.RemoteBot, *models.RemoteGuild, error) {
	bot, err := parseBotContext(ctx)
	if err != nil {
		return nil, nil, errorx.Decorate(err, "parse bot context")
	}

	guild, ok := ctx.Value(guildKey{}).(*models.RemoteGuild)
	if !ok {
		return nil, nil, errorx.IllegalState.New("missing guild")
	}

	return bot, guild, nil
}
