package orca

import (
	"context"

	pb "ProjectOrca/proto"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

//nolint:ireturn // required by function signature.
func (o *Orca) UnaryInterceptor(
	ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (any, error) {
	// special case for health checks.
	_, ok := req.(*pb.HealthRequest)
	if ok {
		return handler(ctx, req)
	}

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
