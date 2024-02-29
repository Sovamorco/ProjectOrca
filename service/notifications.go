package orca

import (
	"context"
	"encoding/json"

	"ProjectOrca/models/notifications"
	pb "ProjectOrca/proto"

	"github.com/joomcode/errorx"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Subscribe(_ *emptypb.Empty, srv pb.Orca_SubscribeServer) error {
	ctx := srv.Context()

	bot, err := parseBotContext(ctx)
	if err != nil {
		return errorx.Decorate(err, "parse bot context")
	}

	o.store.Subscribe(ctx,
		o.handleQueueNotification(srv),
		notifications.QueueNotificationChannelForBot(o.config.Redis.Prefix, bot.ID),
	)

	select {
	case <-ctx.Done():
	case <-o.shutdown:
	}

	return nil
}

func (o *Orca) handleQueueNotification(
	srv pb.Orca_SubscribeServer,
) func(context.Context, *redis.Message) error {
	return func(_ context.Context, m *redis.Message) error {
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
