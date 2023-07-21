package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ProjectOrca/store"

	pb "ProjectOrca/proto"
	"ProjectOrca/utils"

	"github.com/google/uuid"
	"github.com/joomcode/errorx"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/durationpb"
)

var ErrNoResults = errors.New("no search results")

const (
	RequeueOrdKeyDiff = 50
)

type YTDLData struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Channel     string            `json:"channel"`
	OriginalURL string            `json:"original_url"`
	URL         string            `json:"url"`
	IsLive      bool              `json:"is_live"`
	Duration    float64           `json:"duration"`
	HTTPHeaders map[string]string `json:"http_headers"`
}

type YTDLSearchData struct {
	YTDLData
	Entries []YTDLData `json:"entries"`
}

type RemoteTrack struct {
	bun.BaseModel `bun:"table:tracks" exhaustruct:"optional"`

	ID          string `bun:",pk"`
	BotID       string
	GuildID     string
	Pos         time.Duration
	Duration    time.Duration
	OrdKey      float64
	Title       string
	OriginalURL string
	URL         string
	HTTPHeaders map[string]string
	Live        bool
}

func NewRemoteTracks(logger *zap.SugaredLogger, botID, guildID, url string) ([]*RemoteTrack, error) {
	if !utils.URLRx.MatchString(url) {
		url = "ytsearch:" + url
	}

	data, err := getTracksData(logger, url)
	if err != nil {
		return nil, errorx.Decorate(err, "get tracks data")
	}

	res := make([]*RemoteTrack, len(data))

	for i, datum := range data {
		originalURL := datum.URL
		if datum.OriginalURL != "" {
			originalURL = datum.OriginalURL
		}

		res[i] = &RemoteTrack{
			ID:          uuid.New().String(),
			BotID:       botID,
			GuildID:     guildID,
			Pos:         0,
			Duration:    time.Duration(datum.Duration * float64(time.Second)),
			OrdKey:      0,
			Title:       datum.Title,
			OriginalURL: originalURL,
			URL:         "",
			HTTPHeaders: datum.HTTPHeaders,
			Live:        datum.IsLive,
		}
	}

	return res, nil
}

func (t *RemoteTrack) ToProto() *pb.TrackData {
	return &pb.TrackData{
		Title:       t.Title,
		OriginalURL: t.OriginalURL,
		Live:        t.Live,
		Position:    durationpb.New(t.Pos),
		Duration:    durationpb.New(t.Duration),
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
		ModelTableExpr("tracks").
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
		Set("tracks.ord_key = last.ord_key + ?, pos=0", RequeueOrdKeyDiff).
		Where("tracks.id = ?", t.ID). // WherePK does not seem to response ModelTableExpr
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

func getTracksData(logger *zap.SugaredLogger, url string) ([]YTDLData, error) {
	jsonB, err := utils.GetYTDLPOutput(logger, "--flat-playlist", "-J", url)
	if err != nil {
		return nil, errorx.Decorate(err, "get ytdlp output")
	}

	var vd YTDLSearchData
	err = json.Unmarshal(jsonB, &vd)

	if err != nil {
		return nil, errorx.Decorate(err, "unmarshal ytdl output")
	}

	var ad []YTDLData

	if vd.Entries == nil {
		ad = []YTDLData{vd.YTDLData}
	} else {
		if len(vd.Entries) < 1 {
			return nil, ErrNoResults
		}
		ad = vd.Entries
	}

	return ad, nil
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
