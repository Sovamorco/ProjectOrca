package orca

import (
	"context"

	"ProjectOrca/models"
	"ProjectOrca/models/notifications"
	pb "ProjectOrca/proto"

	"github.com/sovamorco/errorx"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) updatePauseState(
	ctx context.Context,
	bot *models.RemoteBot, guild *models.RemoteGuild,
	paused bool,
) error {
	_, err := guild.
		UpdateQuery(o.store).
		Set("paused = ?", paused).
		Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "update guild pause state to %t", paused)
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		return errorx.Decorate(err, "send resync")
	}

	go notifications.SendQueueNotificationLog(context.WithoutCancel(ctx), o.store, bot.ID, guild.ID)

	return nil
}

func (o *Orca) Pause(ctx context.Context, _ *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	err = o.updatePauseState(ctx, bot, guild, true)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (o *Orca) Resume(ctx context.Context, _ *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	err = o.updatePauseState(ctx, bot, guild, false)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
