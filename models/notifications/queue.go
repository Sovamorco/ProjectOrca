package notifications

import (
	"context"
	"encoding/json"
	"fmt"

	"ProjectOrca/store"

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

func SendQueueNotificationLog(
	ctx context.Context,
	logger *zap.SugaredLogger,
	store *store.Store,
	botID string, guildID string,
) {
	b, err := json.Marshal(QueueNotificationMessage{
		Bot:   botID,
		Guild: guildID,
	})
	if err != nil {
		logger.Errorf("Error marshalling queue notification message: %+v", err)

		return
	}

	err = store.Publish(ctx, QueueNotificationChannelForBot(store.RedisPrefix, botID), b).Err()
	if err != nil {
		logger.Errorf("Error publishing queue notification: %+v", err)

		return
	}
}
