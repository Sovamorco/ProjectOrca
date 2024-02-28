package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/joomcode/errorx"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type MessageHandler func(context.Context, *zap.SugaredLogger, *redis.Message) error

func (s *Store) Subscribe(ctx context.Context, mh MessageHandler, channels ...string) {
	ps := s.Client.Subscribe(ctx, channels...)

	s.unsubscribeFuncs = append(s.unsubscribeFuncs, func(ctx context.Context) {
		err := ps.Unsubscribe(ctx, channels...)
		if err != nil {
			s.logger.Errorf("Error unsubscribing from channels: %+v", err)
		}
	})

	go s.subscriptionHandler(ctx, mh, ps.Channel())
}

func (s *Store) subscriptionHandler(ctx context.Context, mh MessageHandler, ch <-chan *redis.Message) {
	logger := s.logger.Named("sub_handler")

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

		logger := logger.With(
			zap.String("channel", msg.Channel),
		)

		// ignore messages from keyevent del channel
		if msg.Channel == "__keyevent@0__:del" {
			logger = zap.NewNop().Sugar()
		}

		wg.Add(1)

		go func() {
			defer wg.Done()

			logger.Debugf("Received message: %s", msg.Payload)

			err := mh(ctx, logger, msg)
			if err != nil {
				logger.Errorf("Error processing message: %+v", err)
			}
		}()
	}

	wg.Wait()
}

func (s *Store) Lock(ctx context.Context, key string) error {
	mu := s.rs.NewMutex(
		fmt.Sprintf("%s:%s", s.RedisPrefix, key),
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
			s.logger.Errorf("Error unlocking state: %+v", err)
		}
	})

	return nil
}

func (s *Store) extendLoop(ctx context.Context, mu *redsync.Mutex) {
	ticker := time.NewTicker(lockExtendFrequency)
	defer ticker.Stop()

	for {
		_, err := mu.ExtendContext(ctx)
		if err != nil {
			s.logger.Errorf("Error extending lock: %+v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
