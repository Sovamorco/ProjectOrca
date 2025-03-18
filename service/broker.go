package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"ProjectOrca/models"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sovamorco/errorx"
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

func (o *Orca) handleResync(ctx context.Context, msg *redis.Message) error {
	logger := zerolog.Ctx(ctx)

	r := new(ResyncMessage)

	err := json.Unmarshal([]byte(msg.Payload), r)
	if err != nil {
		return errorx.Decorate(err, "unmarshal resync")
	}

	tl := logger.With().Str("botID", r.Bot).Str("guildID", r.Guild).Logger()
	logger = &tl
	ctx = logger.WithContext(ctx)

	managed := o.getStateByID(r.Bot)
	if managed == nil {
		logger.Debug().Msg("Resync message for non-managed recipient, skipping")

		return nil
	}

	o.doResync(ctx, managed, r)

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

func (o *Orca) doResync(ctx context.Context, managed *models.Bot, r *ResyncMessage) {
	logger := zerolog.Ctx(ctx)

	var wg sync.WaitGroup

	for _, target := range r.Targets {
		wg.Add(1)

		go func() {
			defer wg.Done()

			switch target {
			case ResyncTargetGuild:
				err := managed.ResyncGuild(ctx, r.Guild)
				if err != nil {
					logger.Error().Err(err).Msg("Error resyncing guild")
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
			return errorx.Decorate(err, "update guild channel id")
		}
	}

	err := o.sendResync(ctx, botID, guild.ID, ResyncTargetCurrent, ResyncTargetGuild)
	if err != nil {
		return errorx.Decorate(err, "send resync")
	}

	return nil
}

func (o *Orca) handleKeyDel(ctx context.Context, msg *redis.Message) error {
	logger := zerolog.Ctx(ctx)

	val := msg.Payload
	pref := o.store.RedisPrefix + ":"

	if !strings.HasPrefix(val, pref) {
		return nil
	}

	val = strings.TrimPrefix(val, pref)

	tl := logger.With().Str("stateId", val).Logger()
	logger = &tl
	ctx = logger.WithContext(ctx)

	_, exists := o.states.Load(val)
	if exists {
		logger.Fatal().Msg("Own managed state expired") // TODO: gracefully remove this state?

		return nil // never reached because logger.Fatal calls os.Exit, more of an IDE/linter hint
	}

	logger.Info().Msg("Lock expired, trying takeover")

	// reset to default logger so we don't pass subscription handler fields to long-running states.
	initCtx := log.Logger.WithContext(ctx)

	err := o.initFromStore(initCtx)
	if err != nil {
		return errorx.Decorate(err, "init after takeover")
	}

	return nil
}
