package orca

import (
	"context"
	"database/sql"
	"errors"

	"ProjectOrca/models"

	"github.com/joomcode/errorx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func (o *Orca) authenticate(ctx context.Context) (*models.RemoteBot, error) {
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
		if errors.Is(err, sql.ErrNoRows) {
			return o.register(ctx, token)
		}

		return nil, errorx.Decorate(err, "get state from store")
	}

	return &s, nil
}

func (o *Orca) authenticateWithGuild(ctx context.Context, guildID string) (
	*models.RemoteBot,
	*models.RemoteGuild,
	error,
) {
	bot, err := o.authenticate(ctx)
	if err != nil {
		return nil, nil, errorx.Decorate(err, "authenticate bot")
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
		if errors.Is(err, sql.ErrNoRows) {
			g := models.NewRemoteGuild(bot.ID, guildID)

			_, err = o.store.
				NewInsert().
				Model(g).
				Exec(ctx)
			if err != nil {
				return nil, nil, errorx.Decorate(err, "create guild")
			}

			return bot, g, nil
		}

		return nil, nil, errorx.Decorate(err, "get guild from store")
	}

	return bot, &r, nil
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
	state, err := models.NewBot(o.logger, o.store, o.extractors, token)
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
		state.Shutdown()

		return nil, errorx.Decorate(err, "store remote bot state")
	}

	o.states.Store(state.GetID(), state)

	return r, nil
}
