package orca

import (
	"context"

	pb "ProjectOrca/proto"

	"github.com/sovamorco/errorx"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Join(ctx context.Context, in *pb.JoinRequest) (*emptypb.Empty, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	if guild.ChannelID == in.ChannelID {
		return &emptypb.Empty{}, nil
	}

	_, err = guild.UpdateQuery(o.store).
		Set("channel_id = ?", in.ChannelID).
		Exec(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "update guild channel id to %s", in.ChannelID)
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		return nil, errorx.Decorate(err, "send resync")
	}

	return &emptypb.Empty{}, nil
}

func (o *Orca) Leave(ctx context.Context, _ *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	if guild.ChannelID == "" {
		return &emptypb.Empty{}, nil
	}

	_, err = guild.
		UpdateQuery(o.store).
		Set("channel_id = NULL").
		Exec(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "update guild channel id to NULL")
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		return nil, errorx.Decorate(err, "send resync")
	}

	return &emptypb.Empty{}, nil
}
