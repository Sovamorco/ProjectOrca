package store

import (
	"context"
	"sync"

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
