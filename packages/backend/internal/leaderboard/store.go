package leaderboard

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const (
	MaxEntries    = 50
	MaxCandidates = 200
)

type Meta struct {
	Title  string `json:"title"`
	Author string `json:"author"`
}

type Ranked struct {
	ProjectID string
	Visits    int64
}

type Counters struct {
	Exposure int64
	Clicks   int64
}

type Store struct {
	client *redis.Client
	prefix string
}

func NewStore(client *redis.Client, prefix string) *Store {
	return &Store{client: client, prefix: prefix}
}

func (s *Store) pvKey() string       { return s.prefix + ":leaderboard:pv" }
func (s *Store) hiddenKey() string   { return s.prefix + ":leaderboard:hidden" }
func (s *Store) exposureKey() string { return s.prefix + ":leaderboard:exposure" }
func (s *Store) clicksKey() string   { return s.prefix + ":leaderboard:clicks" }
func (s *Store) metaKey() string     { return s.prefix + ":leaderboard:meta" }

func (s *Store) RecordView(ctx context.Context, projectID string) error {
	pipe := s.client.TxPipeline()
	pipe.ZIncrBy(ctx, s.pvKey(), 1, projectID)
	pipe.ZRemRangeByRank(ctx, s.pvKey(), 0, -(MaxCandidates + 1))
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) RecordClick(ctx context.Context, projectID string) error {
	return s.client.HIncrBy(ctx, s.clicksKey(), projectID, 1).Err()
}

func (s *Store) RecordExposure(ctx context.Context, projectIDs []string) error {
	if len(projectIDs) == 0 {
		return nil
	}
	pipe := s.client.TxPipeline()
	for _, id := range projectIDs {
		pipe.HIncrBy(ctx, s.exposureKey(), id, 1)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Top(ctx context.Context, limit int) ([]Ranked, error) {
	if limit <= 0 {
		limit = MaxEntries
	}
	rows, err := s.client.ZRevRangeWithScores(ctx, s.pvKey(), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Ranked, 0, len(rows))
	for _, row := range rows {
		member, ok := row.Member.(string)
		if !ok {
			continue
		}
		out = append(out, Ranked{ProjectID: member, Visits: int64(row.Score)})
	}
	return out, nil
}

func (s *Store) Counters(ctx context.Context, projectIDs []string) (map[string]Counters, error) {
	out := make(map[string]Counters, len(projectIDs))
	if len(projectIDs) == 0 {
		return out, nil
	}
	exposure, err := s.client.HMGet(ctx, s.exposureKey(), projectIDs...).Result()
	if err != nil {
		return nil, err
	}
	clicks, err := s.client.HMGet(ctx, s.clicksKey(), projectIDs...).Result()
	if err != nil {
		return nil, err
	}
	for i, id := range projectIDs {
		out[id] = Counters{Exposure: parseInt64(exposure[i]), Clicks: parseInt64(clicks[i])}
	}
	return out, nil
}

func (s *Store) Meta(ctx context.Context, projectIDs []string) (map[string]Meta, error) {
	out := make(map[string]Meta, len(projectIDs))
	if len(projectIDs) == 0 {
		return out, nil
	}
	values, err := s.client.HMGet(ctx, s.metaKey(), projectIDs...).Result()
	if err != nil {
		return nil, err
	}
	for i, id := range projectIDs {
		raw, ok := values[i].(string)
		if !ok || raw == "" {
			continue
		}
		var meta Meta
		if err := json.Unmarshal([]byte(raw), &meta); err != nil {
			continue
		}
		out[id] = meta
	}
	return out, nil
}

func (s *Store) PutMeta(ctx context.Context, projectID string, meta Meta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return s.client.HSet(ctx, s.metaKey(), projectID, data).Err()
}

func (s *Store) HiddenSet(ctx context.Context) (map[string]bool, error) {
	ids, err := s.client.SMembers(ctx, s.hiddenKey()).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

func (s *Store) Hide(ctx context.Context, projectID string, hidden bool) error {
	if hidden {
		return s.client.SAdd(ctx, s.hiddenKey(), projectID).Err()
	}
	return s.client.SRem(ctx, s.hiddenKey(), projectID).Err()
}

func (s *Store) Delete(ctx context.Context, projectID string) error {
	pipe := s.client.TxPipeline()
	pipe.ZRem(ctx, s.pvKey(), projectID)
	pipe.HDel(ctx, s.exposureKey(), projectID)
	pipe.HDel(ctx, s.clicksKey(), projectID)
	pipe.HDel(ctx, s.metaKey(), projectID)
	pipe.SRem(ctx, s.hiddenKey(), projectID)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) PruneRanking(ctx context.Context, projectIDs []string) error {
	if len(projectIDs) == 0 {
		return nil
	}
	pipe := s.client.TxPipeline()
	for _, id := range projectIDs {
		pipe.ZRem(ctx, s.pvKey(), id)
		pipe.HDel(ctx, s.metaKey(), id)
		pipe.SRem(ctx, s.hiddenKey(), id)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func parseInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}
