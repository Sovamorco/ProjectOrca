package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ProjectOrca/extractor"

	"ProjectOrca/store"

	pb "ProjectOrca/proto"
	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"github.com/uptrace/bun"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	RequeueOrdKeyDiff = 50
)

type RemoteTrack struct {
	bun.BaseModel `bun:"table:tracks" exhaustruct:"optional"`

	ID            string `bun:",pk"`
	BotID         string
	GuildID       string
	Pos           time.Duration
	Duration      time.Duration
	OrdKey        float64
	Title         string
	ExtractionURL string
	DisplayURL    string
	StreamURL     string
	HTTPHeaders   map[string]string
	Live          bool
}

func NewRemoteTracks(
	ctx context.Context,
	botID, guildID, url string,
	extractors *extractor.Extractors,
) ([]*RemoteTrack, error) {
	data, err := extractors.ExtractTracksData(ctx, url)
	if err != nil {
		return nil, errorx.Decorate(err, "get tracks data")
	}

	res := make([]*RemoteTrack, len(data))

	for i, datum := range data {
		res[i] = &RemoteTrack{
			ID:            uuid.New().String(),
			BotID:         botID,
			GuildID:       guildID,
			Pos:           0,
			Duration:      datum.Duration,
			OrdKey:        0,
			Title:         datum.Title,
			ExtractionURL: datum.ExtractionURL,
			DisplayURL:    datum.DisplayURL,
			StreamURL:     datum.StreamURL,
			HTTPHeaders:   datum.HTTPHeaders,
			Live:          datum.Live,
		}
	}

	return res, nil
}

func (t *RemoteTrack) ToProto() *pb.TrackData {
	return &pb.TrackData{
		Title:      t.Title,
		DisplayURL: t.DisplayURL,
		Live:       t.Live,
		Position:   durationpb.New(t.Pos),
		Duration:   durationpb.New(t.Duration),
	}
}

func (t *RemoteTrack) Delete(ctx context.Context, store *store.Store) error {
	_, err := store.
		NewDelete().
		Model(t).
		WherePK().
		Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "delete track")
	}

	return nil
}

func (t *RemoteTrack) Requeue(ctx context.Context, store *store.Store) error {
	_, err := store.
		NewUpdate().
		Model(t).
		TableExpr(
			"(?) AS last",
			store.
				NewSelect().
				Model((*RemoteTrack)(nil)).
				Column("ord_key").
				Where("bot_id = ? AND guild_id = ?", t.BotID, t.GuildID).
				Order("ord_key DESC").
				Limit(1),
		).
		Set("ord_key = last.ord_key + ?, pos=0", RequeueOrdKeyDiff).
		WherePK().
		Exec(ctx)
	if err != nil {
		return errorx.Decorate(err, "requeue track")
	}

	return nil
}

func (t *RemoteTrack) DeleteOrRequeue(ctx context.Context, store *store.Store) error {
	var g RemoteGuild

	g.BotID = t.BotID
	g.ID = t.GuildID

	err := store.
		NewSelect().
		Model(&g).
		Column("loop").
		WherePK().
		Scan(ctx)
	if err != nil {
		return errorx.Decorate(err, "get guild looping state")
	}

	if g.Loop {
		return t.Requeue(ctx, store)
	}

	return t.Delete(ctx, store)
}

func (t *RemoteTrack) getFormattedHeaders() string {
	fmtd := make([]string, 0, len(t.HTTPHeaders))

	for k, v := range t.HTTPHeaders {
		fmtd = append(fmtd, fmt.Sprintf("%s:%s", k, v))
	}

	return strings.Join(fmtd, "\r\n")
}

func SetOrdKeys(tracks []*RemoteTrack, prevOrdKey, nextOrdKey float64) {
	// calculate step
	// len(tracks) + 1 because we need the last new ordKey to not be equal to nextOrdKey
	step := (nextOrdKey - prevOrdKey) / float64(len(tracks)+1)

	for i, track := range tracks {
		// i + 1 because we need the first new ordKey to not be equal to prevOrdKey
		track.OrdKey = prevOrdKey + step*float64(i+1)
	}
}
