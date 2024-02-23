package notifications

import (
	"context"
	"encoding/json"
	"fmt"

	"ProjectOrca/store"

	"github.com/joomcode/errorx"
	"go.uber.org/zap"
)

const (
	QueueNotificationsChannel = "queue_notifications"
)

func QueueNotificationChannelForBot(redisPrefix, botID string) string {
	return fmt.Sprintf("%s:%s_%s", redisPrefix, QueueNotificationsChannel, botID)
}

type QueueNotificationMessage struct {
	Bot   string `json:"bot"`
	Guild string `json:"guild"`
}

func SendQueueNotification(
	ctx context.Context,
	store *store.Store,
	botID string, guildID string,
) error {
	b, err := json.Marshal(QueueNotificationMessage{
		Bot:   botID,
		Guild: guildID,
	})
	if err != nil {
		return errorx.Decorate(err, "marshal queue notification")
	}

	err = store.Publish(ctx, QueueNotificationChannelForBot(store.RedisPrefix, botID), b).Err()
	if err != nil {
		return errorx.Decorate(err, "publish queue notification")
	}

	return nil
}

func SendQueueNotificationLog(
	ctx context.Context,
	logger *zap.SugaredLogger,
	store *store.Store,
	botID string, guildID string,
) {
	err := SendQueueNotification(ctx, store, botID, guildID)
	if err != nil {
		logger.Errorf("Error sending queue notification: %+v", err)
	}
}
