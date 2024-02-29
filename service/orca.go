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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ProjectOrca/extractor"

	"ProjectOrca/models"
	pb "ProjectOrca/proto"
	"ProjectOrca/store"

	"github.com/google/uuid"
	"github.com/joomcode/errorx"
)

var (
	ErrInternal        = status.Error(codes.Internal, "internal error")
	ErrUnauthenticated = status.Error(codes.Unauthenticated, "unauthenticated")
)

// wrappedStream wraps around the embedded grpc.ServerStream, and intercepts the RecvMsg and
// SendMsg method call.
//
//nolint:containedctx
type wrappedStream struct {
	ctx context.Context
	grpc.ServerStream
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

func newWrappedStream(ctx context.Context, s grpc.ServerStream) *wrappedStream {
	return &wrappedStream{ctx, s}
}

type GuildIDGetter interface {
	GetGuildID() string
}

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

	o.extractors.AddExtractor(ytdl.New())

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

//nolint:ireturn // required by function signature.
func (o *Orca) UnaryInterceptor(
	ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (any, error) {
	// put default logger in context.
	ctx = log.Logger.WithContext(ctx)

	var err error

	in, ok := req.(GuildIDGetter)
	if ok {
		guildID := in.GetGuildID()

		ctx, err = o.authenticateWithGuild(ctx, guildID)
		if err != nil {
			log.Error().Str("guildID", guildID).Err(err).Msg("Error authenticating with guild")

			return nil, ErrUnauthenticated
		}
	} else {
		ctx, err = o.authenticate(ctx)
		if err != nil {
			log.Error().Err(err).Msg("Error authenticating")

			return nil, ErrUnauthenticated
		}
	}

	logger := zerolog.Ctx(ctx).With().Str("method", info.FullMethod).Logger()
	ctx = logger.WithContext(ctx)

	logger.Trace().Msg("Called")

	resp, err := handler(ctx, req)
	if err != nil {
		if statusErr, ok := status.FromError(err); ok {
			return nil, statusErr.Err()
		}

		logger.Error().Err(err).Msg("Error in handler")

		return nil, ErrInternal
	}

	return resp, nil
}

func (o *Orca) StreamInterceptor(
	srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler,
) error {
	ctx := stream.Context()

	// put default logger in context.
	ctx = log.Logger.WithContext(ctx)

	ctx, err := o.authenticate(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error authenticating")

		return ErrUnauthenticated
	}

	logger := zerolog.Ctx(ctx).With().Str("method", info.FullMethod).Logger()
	ctx = logger.WithContext(ctx)

	ws := newWrappedStream(ctx, stream)

	logger.Trace().Msg("Called")

	return handler(srv, ws)
}
