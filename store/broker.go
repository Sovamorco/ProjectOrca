package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"

	"github.com/redis/go-redis/v9"
)

type MessageHandler func(context.Context, *redis.Message) error

func (s *Store) Subscribe(ctx context.Context, mh MessageHandler, channels ...string) {
	logger := zerolog.Ctx(ctx)

	ps := s.Client.Subscribe(ctx, channels...)

	s.unsubscribeFuncs = append(s.unsubscribeFuncs, func(ctx context.Context) {
		err := ps.Unsubscribe(ctx, channels...)
		if err != nil {
			logger.Error().Err(err).Msg("Error unsubscribing from channels")
		}
	})

	go s.subscriptionHandler(ctx, mh, ps.Channel())
}

func (s *Store) subscriptionHandler(ctx context.Context, mh MessageHandler, ch <-chan *redis.Message) {
	logger := zerolog.Ctx(ctx).With().Str("component", "subscriptionHandler").Logger()

	var wg sync.WaitGroup

	var msg *redis.Message

	var ok bool

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case msg, ok = <-ch:
			if !ok {
				break loop
			}
		}

		logger := logger.With().Str("channel", msg.Channel).Logger()
		ctx := logger.WithContext(ctx)

		// ignore messages from keyevent del channel
		if msg.Channel == "__keyevent@0__:del" {
			logger = logger.Level(zerolog.Disabled)
		}

		wg.Add(1)

		go func() {
			defer wg.Done()

			logger.Debug().Str("payload", msg.Payload).Msg("Received message")

			err := mh(ctx, msg)
			if err != nil {
				logger.Error().Err(err).Msg("Error processing message")
			}
		}()
	}

	wg.Wait()
}

func (s *Store) Lock(ctx context.Context, key string) error {
	lockName := fmt.Sprintf("%s:%s", s.RedisPrefix, key)

	logger := zerolog.Ctx(ctx).With().Str("lockName", lockName).Logger()
	ctx = logger.WithContext(ctx)

	mu := s.rs.NewMutex(
		lockName,
		redsync.WithExpiry(lockExpiry),
		redsync.WithTries(lockTries),
	)

	err := mu.LockContext(ctx)
	if err != nil {
		return errorx.Decorate(err, "lock")
	}

	go s.extendLoop(ctx, mu)

	s.shutdownFuncs = append(s.shutdownFuncs, func(ctx context.Context) {
		_, err := mu.UnlockContext(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("Error unlocking state")
		}
	})

	return nil
}

func (s *Store) extendLoop(ctx context.Context, mu *redsync.Mutex) {
	logger := zerolog.Ctx(ctx)

	ticker := time.NewTicker(lockExtendFrequency)
	defer ticker.Stop()

	for {
		_, err := mu.ExtendContext(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("Error extending lock")
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
