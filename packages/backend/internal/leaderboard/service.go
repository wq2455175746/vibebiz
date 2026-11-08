package leaderboard

import (
	"context"
	"log/slog"
	"time"

	"vibebiz/internal/model"
	"vibebiz/internal/publish"
)

type PublishedLister interface {
	List(ctx context.Context) ([]publish.Published, error)
}

type ProjectLister interface {
	List(ctx context.Context) []*model.Project
}

type UserNameResolver interface {
	Usernames(ctx context.Context, ids []int64) (map[int64]string, error)
}

type CoverURLSigner interface {
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}

type Item struct {
	Rank      int
	ProjectID string
	Name      string
	Author    string
	Visits    int64
	CoverURL  string
}

type AdminRow struct {
	ProjectID   string
	Name        string
	Author      string
	Visits      int64
	Exposure    int64
	Clicks      int64
	Hidden      bool
	PublishedAt time.Time
}

type StatInput struct {
	ProjectID string
	Name      string
}

type ProjectStat struct {
	ProjectID    string `json:"projectId"`
	Name         string `json:"name"`
	Exposure     int64  `json:"exposure"`
	Clicks       int64  `json:"clicks"`
	DwellSeconds int64  `json:"dwellSeconds"`
	Spend        int64  `json:"spend"`
}

type Service struct {
	Store       *Store
	Published   PublishedLister
	Projects    ProjectLister
	Users       UserNameResolver
	Covers      CoverURLSigner
	CoverExpiry time.Duration
	Logger      *slog.Logger
}

func NewService(store *Store, published PublishedLister, projects ProjectLister, users UserNameResolver) *Service {
	return &Service{Store: store, Published: published, Projects: projects, Users: users}
}

func (s *Service) logger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *Service) coverURL(ctx context.Context, project *model.Project) string {
	if project == nil {
		return ""
	}
	if project.CoverS3Path == "" || s.Covers == nil {
		return project.CoverURL
	}
	expiry := s.CoverExpiry
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	signed, err := s.Covers.PresignGet(ctx, project.CoverS3Path, expiry)
	if err != nil {
		s.logger().Error("leaderboard sign cover failed", "project_id", project.ID, "error", err)
		return project.CoverURL
	}
	return signed
}

type view struct {
	ranked    []Ranked
	published map[string]publish.Published
	projects  map[string]*model.Project
	hidden    map[string]bool
}

func (s *Service) loadView(ctx context.Context) (*view, error) {
	ranked, err := s.Store.Top(ctx, MaxCandidates)
	if err != nil {
		return nil, err
	}
	published, err := s.Published.List(ctx)
	if err != nil {
		return nil, err
	}
	publishedByID := make(map[string]publish.Published, len(published))
	for _, item := range published {
		publishedByID[item.ProjectID] = item
	}
	projectsByID := make(map[string]*model.Project)
	for _, item := range s.Projects.List(ctx) {
		projectsByID[item.ID] = item
	}
	hidden, err := s.Store.HiddenSet(ctx)
	if err != nil {
		return nil, err
	}
	return &view{ranked: ranked, published: publishedByID, projects: projectsByID, hidden: hidden}, nil
}

func (s *Service) visible(ctx context.Context, v *view, includeHidden bool) []Ranked {
	out := make([]Ranked, 0, len(v.ranked))
	stale := make([]string, 0)
	for _, item := range v.ranked {
		project, ok := v.projects[item.ProjectID]
		_, published := v.published[item.ProjectID]
		if !ok || !published {
			stale = append(stale, item.ProjectID)
			continue
		}
		if project.IsHidden {
			continue
		}
		if v.hidden[item.ProjectID] && !includeHidden {
			continue
		}
		out = append(out, item)
	}
	if len(stale) > 0 {
		_ = s.Store.PruneRanking(ctx, stale)
	}
	return out
}

func (s *Service) ensureMeta(ctx context.Context, v *view, ids []string) (map[string]Meta, error) {
	meta, err := s.Store.Meta(ctx, ids)
	if err != nil {
		return nil, err
	}
	ownerIDs := make([]int64, 0)
	seen := make(map[int64]bool)
	for _, id := range ids {
		if _, ok := meta[id]; ok {
			continue
		}
		project := v.projects[id]
		if project == nil || seen[project.OwnerUserID] {
			continue
		}
		seen[project.OwnerUserID] = true
		ownerIDs = append(ownerIDs, project.OwnerUserID)
	}
	names := make(map[int64]string)
	if len(ownerIDs) > 0 && s.Users != nil {
		resolved, err := s.Users.Usernames(ctx, ownerIDs)
		if err != nil {
			s.logger().Error("leaderboard resolve authors failed", "error", err)
			return meta, nil
		}
		names = resolved
	}
	for _, id := range ids {
		if _, ok := meta[id]; ok {
			continue
		}
		project := v.projects[id]
		if project == nil {
			continue
		}
		title := v.published[id].Title
		if title == "" {
			title = project.Name
		}
		item := Meta{Title: title, Author: names[project.OwnerUserID]}
		meta[id] = item
		if err := s.Store.PutMeta(ctx, id, item); err != nil {
			s.logger().Error("leaderboard save meta failed", "project_id", id, "error", err)
		}
	}
	return meta, nil
}

func (s *Service) ListPublic(ctx context.Context, limit int) ([]Item, error) {
	if limit <= 0 || limit > MaxEntries {
		limit = MaxEntries
	}
	v, err := s.loadView(ctx)
	if err != nil {
		return nil, err
	}
	visible := s.visible(ctx, v, false)
	if len(visible) > limit {
		visible = visible[:limit]
	}
	ids := make([]string, 0, len(visible))
	for _, item := range visible {
		ids = append(ids, item.ProjectID)
	}
	meta, err := s.ensureMeta(ctx, v, ids)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(visible))
	for index, item := range visible {
		m := meta[item.ProjectID]
		items = append(items, Item{
			Rank: index + 1, ProjectID: item.ProjectID,
			Name: m.Title, Author: m.Author, Visits: item.Visits,
			CoverURL: s.coverURL(ctx, v.projects[item.ProjectID]),
		})
	}
	if err := s.Store.RecordExposure(ctx, ids); err != nil {
		s.logger().Error("leaderboard record exposure failed", "error", err)
	}
	return items, nil
}

func (s *Service) ListAdmin(ctx context.Context) ([]AdminRow, error) {
	v, err := s.loadView(ctx)
	if err != nil {
		return nil, err
	}
	visible := s.visible(ctx, v, true)
	ids := make([]string, 0, len(visible))
	for _, item := range visible {
		ids = append(ids, item.ProjectID)
	}
	meta, err := s.ensureMeta(ctx, v, ids)
	if err != nil {
		return nil, err
	}
	counters, err := s.Store.Counters(ctx, ids)
	if err != nil {
		return nil, err
	}
	rows := make([]AdminRow, 0, len(visible))
	for _, item := range visible {
		m := meta[item.ProjectID]
		c := counters[item.ProjectID]
		rows = append(rows, AdminRow{
			ProjectID: item.ProjectID, Name: m.Title, Author: m.Author,
			Visits: item.Visits, Exposure: c.Exposure, Clicks: c.Clicks,
			Hidden: v.hidden[item.ProjectID], PublishedAt: v.published[item.ProjectID].PublishedAt,
		})
	}
	return rows, nil
}

func (s *Service) Stats(ctx context.Context, inputs []StatInput) ([]ProjectStat, error) {
	if len(inputs) == 0 {
		return []ProjectStat{}, nil
	}
	ids := make([]string, 0, len(inputs))
	for _, input := range inputs {
		ids = append(ids, input.ProjectID)
	}
	counters, err := s.Store.Counters(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectStat, 0, len(inputs))
	for _, input := range inputs {
		c := counters[input.ProjectID]
		out = append(out, ProjectStat{
			ProjectID: input.ProjectID, Name: input.Name,
			Exposure: c.Exposure, Clicks: c.Clicks, DwellSeconds: 0, Spend: 0,
		})
	}
	return out, nil
}

func (s *Service) UpsertMeta(ctx context.Context, project *model.Project, title string) error {
	if s == nil || s.Store == nil || project == nil {
		return nil
	}
	author := ""
	if s.Users != nil {
		names, err := s.Users.Usernames(ctx, []int64{project.OwnerUserID})
		if err != nil {
			return err
		}
		author = names[project.OwnerUserID]
	}
	if title == "" {
		title = project.Name
	}
	return s.Store.PutMeta(ctx, project.ID, Meta{Title: title, Author: author})
}

func (s *Service) RecordView(ctx context.Context, projectID string) error {
	if s == nil || s.Store == nil {
		return nil
	}
	err := s.Store.RecordView(ctx, projectID)
	if err != nil {
		s.logger().Error("leaderboard record view failed", "project_id", projectID, "error", err)
	}
	return err
}

func (s *Service) RecordClick(ctx context.Context, projectID string) error {
	if s == nil || s.Store == nil {
		return nil
	}
	err := s.Store.RecordClick(ctx, projectID)
	if err != nil {
		s.logger().Error("leaderboard record click failed", "project_id", projectID, "error", err)
	}
	return err
}

func (s *Service) Hide(ctx context.Context, projectID string, hidden bool) error {
	if s == nil || s.Store == nil {
		return nil
	}
	return s.Store.Hide(ctx, projectID, hidden)
}

func (s *Service) Delete(ctx context.Context, projectID string) error {
	if s == nil || s.Store == nil {
		return nil
	}
	return s.Store.Delete(ctx, projectID)
}
