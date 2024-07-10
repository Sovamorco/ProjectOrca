package orca

import (
	"context"
	"errors"
	"os"
	"sync"

	"ProjectOrca/vk"

	"ProjectOrca/config"
	"ProjectOrca/spotify"
	"ProjectOrca/yandexmusic"
	"ProjectOrca/ytdl"

	"github.com/go-redsync/redsync/v4"
	"github.com/rs/zerolog"

	"ProjectOrca/extractor"

	"ProjectOrca/models"
	pb "ProjectOrca/proto"
	"ProjectOrca/store"

	"github.com/google/uuid"
	"github.com/joomcode/errorx"
)

type Orca struct {
	pb.UnimplementedOrcaServer `exhaustruct:"optional"`

	// static values
	id         string
	store      *store.Store
	extractors *extractor.Extractors
	config     *config.Config

	// concurrency-safe
	states sync.Map
	// broadcast channel for shutdown
	shutdown chan struct{}
}

func New(
	ctx context.Context, st *store.Store, config *config.Config,
) (*Orca, error) {
	orca := &Orca{
		id:         uuid.New().String(),
		store:      st,
		extractors: extractor.NewExtractors(),
		config:     config,

		states:   sync.Map{},
		shutdown: make(chan struct{}),
	}

	err := orca.initExtractors(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "init extractors")
	}

	return orca, nil
}

func (o *Orca) initExtractors(ctx context.Context) error {
	if o.config.Spotify != nil {
		s, err := spotify.New(
			ctx,
			o.config.Spotify.ClientID,
			o.config.Spotify.ClientSecret,
		)
		if err != nil {
			return errorx.Decorate(err, "init spotify module")
		}

		o.extractors.AddExtractor(s)
	}

	if o.config.VK != nil {
		v, err := vk.New(
			ctx,
			o.config.VK.Token,
		)
		if err != nil {
			return errorx.Decorate(err, "init vk module")
		}

		o.extractors.AddExtractor(v)
	}

	cookiesFileName := ""

	if o.config.Youtube.Cookies != "" {
		cookiesFile, err := os.CreateTemp(os.TempDir(), "cookies*")
		if err != nil {
			return errorx.Decorate(err, "create cookies file")
		}

		defer func() {
			_ = cookiesFile.Close()
		}()

		_, err = cookiesFile.WriteString(o.config.Youtube.Cookies)
		if err != nil {
			return errorx.Decorate(err, "write cookies file")
		}

		cookiesFileName = cookiesFile.Name()
	}

	ytex := ytdl.New(cookiesFileName)

	if o.config.YandexMusic != nil {
		ym := yandexmusic.New(ytex, o.config.YandexMusic.Token)

		o.extractors.AddExtractor(ym)
	}

	o.extractors.AddExtractor(ytex)

	return nil
}

func (o *Orca) Init(ctx context.Context) error {
	o.initBroker(ctx)

	err := o.initFromStore(ctx)
	if err != nil {
		return errorx.Decorate(err, "init from store")
	}

	return nil
}

func (o *Orca) ShutdownStreams() {
	close(o.shutdown)
}

func (o *Orca) Shutdown(ctx context.Context) {
	logger := zerolog.Ctx(ctx)
	logger.Info().Msg("Shutting down")

	o.states.Range(func(_, value any) bool {
		state, _ := value.(*models.Bot)

		state.Shutdown(ctx)

		return true
	})

	o.store.Unsubscribe(ctx)

	o.store.Shutdown(ctx)
}

func (o *Orca) initFromStore(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)

	var bots []*models.RemoteBot

	err := o.store.
		NewSelect().
		Model(&bots).
		Column("id", "token").
		Scan(ctx)
	if err != nil {
		return errorx.Decorate(err, "select bots")
	}

	for _, state := range bots {
		logger := logger.With().Str("botID", state.ID).Logger()
		ctx := logger.WithContext(ctx)

		err = o.store.Lock(ctx, state.ID)
		if err != nil {
			var et *redsync.ErrTaken

			if !errors.As(err, &et) {
				logger.Error().Err(err).Msg("Error locking state")
			}

			continue
		}

		logger.Info().Msg("Locked state")

		if s := o.getStateByToken(state.Token); s != nil {
			// state already controlled, no need to init
			continue
		}

		localState, err := models.NewBot(o.store, o.extractors, state.Token)
		if err != nil {
			logger.Error().Err(err).Msg("Could not create local state for remote state")

			continue
		}

		o.states.Store(localState.GetID(), localState)

		go localState.FullResync(ctx)
	}

	return nil
}

func (o *Orca) getStateByToken(token string) *models.Bot {
	var res *models.Bot

	o.states.Range(func(_, value any) bool {
		state, _ := value.(*models.Bot)
		if state.GetToken() == token {
			res = state

			return false
		}

		return true
	})

	return res
}
