package orca

import (
	"context"
	"encoding/json"

	"ProjectOrca/models/notifications"
	pb "ProjectOrca/proto"

	"github.com/joomcode/errorx"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Subscribe(_ *emptypb.Empty, srv pb.Orca_SubscribeServer) error {
	bot, err := o.authenticate(srv.Context())
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return ErrFailedToAuthenticate
	}

	o.store.Subscribe(srv.Context(),
		o.handleQueueNotification(srv),
		notifications.QueueNotificationChannelForBot(o.config.Redis.Prefix, bot.ID),
	)

	select {
	case <-srv.Context().Done():
	case <-o.shutdown:
	}

	return nil
}

func (o *Orca) handleQueueNotification(
	srv pb.Orca_SubscribeServer,
) func(context.Context, *zap.SugaredLogger, *redis.Message) error {
	return func(_ context.Context, _ *zap.SugaredLogger, m *redis.Message) error {
		var msg notifications.QueueNotificationMessage

		err := json.Unmarshal([]byte(m.Payload), &msg)
		if err != nil {
			return errorx.Decorate(err, "unmarshal queue notification message")
		}

		err = srv.Send(&pb.QueueChangeNotification{
			Bot:   msg.Bot,
			Guild: msg.Guild,
		})
		if err != nil {
			return errorx.Decorate(err, "send queue change notification")
		}

		return nil
	}
}
