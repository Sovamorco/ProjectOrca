package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"ProjectOrca/models"

	"github.com/joomcode/errorx"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type ResyncTarget int

const (
	// resync targets.
	ResyncTargetCurrent ResyncTarget = iota + 1 // current playing track.
	ResyncTargetGuild                           // guild-level state (pause, connection, etc.).

	ResyncsChannel = "resync"
)

type ResyncMessage struct {
	Bot     string         `json:"bot"`
	Guild   string         `json:"guild"`
	Targets []ResyncTarget `json:"targets"`
}

func (o *Orca) initBroker(ctx context.Context) {
	o.store.Subscribe(ctx, o.handleResync, fmt.Sprintf("%s:%s", o.config.Redis.Prefix, ResyncsChannel))
	o.store.Subscribe(ctx, o.handleKeyDel,
		"__keyevent@0__:del",
		"__keyevent@0__:expired",
	)
}

func (o *Orca) handleResync(ctx context.Context, logger *zap.SugaredLogger, msg *redis.Message) error {
	r := new(ResyncMessage)

	err := json.Unmarshal([]byte(msg.Payload), r)
	if err != nil {
		return errorx.Decorate(err, "unmarshal resync")
	}

	managed := o.getStateByID(r.Bot)
	if managed == nil {
		logger.Debugf("Resync message for non-managed recipient %s, skipping", r.Bot)

		return nil
	}

	o.doResync(ctx, logger, managed, r)

	return nil
}

// do not use for authentication-related purposes.
func (o *Orca) getStateByID(id string) *models.Bot {
	val, ok := o.states.Load(id)
	if !ok {
		return nil
	}

	state, _ := val.(*models.Bot)

	return state
}

func (o *Orca) doResync(ctx context.Context, logger *zap.SugaredLogger, managed *models.Bot, r *ResyncMessage) {
	var wg sync.WaitGroup

	for _, target := range r.Targets {
		wg.Add(1)

		go func() {
			defer wg.Done()

			switch target {
			case ResyncTargetGuild:
				err := managed.ResyncGuild(ctx, r.Guild)
				if err != nil {
					logger.Errorf("Error resyncing guild: %+v", err)
				}
			case ResyncTargetCurrent:
				managed.ResyncGuildTrack(ctx, r.Guild)
			}
		}()
	}

	wg.Wait()
}

func (o *Orca) sendResync(ctx context.Context, botID, guildID string, targets ...ResyncTarget) error {
	b, err := json.Marshal(ResyncMessage{
		Bot:     botID,
		Guild:   guildID,
		Targets: targets,
	})
	if err != nil {
		return errorx.Decorate(err, "marshal resync message")
	}

	err = o.store.Publish(ctx, fmt.Sprintf("%s:%s", o.store.RedisPrefix, ResyncsChannel), b).Err()
	if err != nil {
		return errorx.Decorate(err, "publish resync message")
	}

	return nil
}

func (o *Orca) queueStartResync(ctx context.Context, guild *models.RemoteGuild, botID, channelID string) error {
	if guild.ChannelID != channelID {
		guild.ChannelID = channelID

		_, err := guild.UpdateQuery(o.store).Column("channel_id").Exec(ctx)
		if err != nil {
			o.logger.Errorf("Error updating guild channel id: %+v", err)

			return ErrInternal
		}
	}

	err := o.sendResync(ctx, botID, guild.ID, ResyncTargetCurrent, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync message: %+v", err)

		return ErrInternal
	}

	return nil
}

func (o *Orca) handleKeyDel(ctx context.Context, logger *zap.SugaredLogger, msg *redis.Message) error {
	val := msg.Payload
	pref := o.store.RedisPrefix + ":"

	if !strings.HasPrefix(val, pref) {
		return nil
	}

	val = strings.TrimPrefix(val, pref)

	_, exists := o.states.Load(val)
	if exists {
		logger.Fatalf("Own managed state %s expired", val) // TODO: gracefully remove this state?

		return nil // never reached because logger.Fatal calls os.Exit, more of an IDE/linter hint
	}

	logger.Infof("Lock for %s expired, trying takeover", val)

	err := o.initFromStore(ctx)
	if err != nil {
		return errorx.Decorate(err, "init after takeover")
	}

	return nil
}
