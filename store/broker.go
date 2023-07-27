package store

import (
	"context"
	"fmt"
	"github.com/go-redsync/redsync/v4"
	"github.com/joomcode/errorx"
	"sync"
	"time"

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

	for msg := range ch {
		msg := msg
		logger := logger.With("channel", msg.Channel)

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

	cancel := make(chan struct{}, 1)

	go s.extendLoop(ctx, mu, cancel)

	s.shutdownFuncs = append(s.shutdownFuncs, func(ctx context.Context) {
		// make sure this does not lock
		select {
		case cancel <- struct{}{}:
		default:
		}

		_, err := mu.UnlockContext(ctx)
		if err != nil {
			s.logger.Errorf("Error unlocking state: %+v", err)
		}
	})

	return nil
}

func (s *Store) extendLoop(ctx context.Context, mu *redsync.Mutex, cancel chan struct{}) {
	ticker := time.NewTicker(lockExtendFrequency)
	defer ticker.Stop()

	for {
		_, err := mu.ExtendContext(ctx)
		if err != nil {
			s.logger.Errorf("Error extending lock: %+v", err)
		}

		select {
		case <-cancel:
			return
		case <-ticker.C:
		}
	}
}
