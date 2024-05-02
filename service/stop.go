package orca

import (
	"context"
	"database/sql"
	"errors"

	"ProjectOrca/models"
	pb "ProjectOrca/proto"

	"github.com/joomcode/errorx"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (o *Orca) Stop(ctx context.Context, _ *pb.GuildOnlyRequest) (*emptypb.Empty, error) {
	bot, guild, err := parseGuildContext(ctx)
	if err != nil {
		return nil, errorx.Decorate(err, "parse authenticated context")
	}

	_, err = o.store.
		NewDelete().
		Model((*models.RemoteTrack)(nil)).
		Where("bot_id = ?", bot.ID).
		Where("guild_id = ?", guild.ID).
		Exec(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, errorx.Decorate(err, "delete all tracks")
	}

	// set pause to false because of expectation that bot starts playing next time a track is requested
	// because stop usually means end of listening session.
	err = o.updatePauseState(ctx, bot, guild, false)
	if err != nil {
		return nil, errorx.Decorate(err, "update pause state")
	}

	err = o.sendResync(ctx, bot.ID, guild.ID, ResyncTargetCurrent)
	if err != nil {
		return nil, errorx.Decorate(err, "send resync")
	}

	return &emptypb.Empty{}, nil
}
