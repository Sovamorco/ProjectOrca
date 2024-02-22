package orca

import (
	"context"

	pb "ProjectOrca/proto"

	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Join(ctx context.Context, in *pb.JoinRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	if guild.ChannelID == in.ChannelID {
		return &emptypb.Empty{}, nil
	}

	_, err = guild.UpdateQuery(o.store).
		Set("channel_id = ?", in.ChannelID).
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating guild channel id: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync request: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}

func (o *Orca) Leave(ctx context.Context, in *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := o.authenticateWithGuild(ctx, in.GuildID)
	if err != nil {
		o.logger.Errorf("Error authenticating request: %+v", err)

		return nil, ErrFailedToAuthenticate
	}

	if guild.ChannelID == "" {
		return &emptypb.Empty{}, nil
	}

	_, err = guild.
		UpdateQuery(o.store).
		Set("channel_id = NULL").
		Exec(ctx)
	if err != nil {
		o.logger.Errorf("Error updating guild: %+v", err)

		return nil, ErrInternal
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetGuild)
	if err != nil {
		o.logger.Errorf("Error sending resync request: %+v", err)

		return nil, ErrInternal
	}

	return &emptypb.Empty{}, nil
}
