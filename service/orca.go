package orca

import (
	"context"
	"errors"
	"sync"

	"ProjectOrca/vk"

	"ProjectOrca/config"
	"ProjectOrca/spotify"
	"ProjectOrca/ytdl"

	"github.com/go-redsync/redsync/v4"

	"ProjectOrca/extractor"

	"ProjectOrca/models"
	pb "ProjectOrca/proto"
	"ProjectOrca/store"

	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrFailedToAuthenticate = status.Error(codes.Unauthenticated, "failed to authenticate bot")
	ErrInternal             = status.Error(codes.Internal, "internal error")
)

type Orca struct {
	pb.UnimplementedOrcaServer `exhaustruct:"optional"`

	// static values
	id         string
	logger     *zap.SugaredLogger
	store      *store.Store
	extractors *extractor.Extractors
	config     *config.Config

	// concurrency-safe
	states sync.Map
	// broadcast channel for shutdown
	shutdown chan struct{}
}

func New(
	ctx context.Context, logger *zap.SugaredLogger, st *store.Store, config *config.Config,
) (*Orca, error) {
	serverLogger := logger.Named("server")

	orca := &Orca{
		id:         uuid.New().String(),
		logger:     serverLogger,
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
			o.logger,
			o.config.VK.Token,
		)
		if err != nil {
			return errorx.Decorate(err, "init vk module")
		}

		o.extractors.AddExtractor(v)
	}

	o.extractors.AddExtractor(ytdl.New(o.logger))

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
	o.logger.Info("Shutting down")

	o.states.Range(func(_, value any) bool {
		state, _ := value.(*models.Bot)

		state.Shutdown()

		return true
	})

	o.store.Unsubscribe(ctx)

	o.store.Shutdown(ctx)
}

func (o *Orca) initFromStore(ctx context.Context) error {
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
		err = o.store.Lock(ctx, state.ID)
		if err != nil {
			var et *redsync.ErrTaken

			if !errors.As(err, &et) {
				o.logger.Errorf("Error locking state: %+v", err)
			}

			continue
		}

		o.logger.Infof("Locked state %s", state.ID)

		if s := o.getStateByToken(state.Token); s != nil {
			// state already controlled, no need to init
			continue
		}

		localState, err := models.NewBot(o.logger, o.store, o.extractors, state.Token)
		if err != nil {
			o.logger.Errorf("Could not create local state for remote state %s: %+v", state.ID, err)

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
