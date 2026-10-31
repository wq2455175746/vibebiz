package projecttemplate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"vibebiz/internal/client"
	"vibebiz/internal/dao/model"
	appmodel "vibebiz/internal/model"
	"vibebiz/internal/store"
)

var topicCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

var (
	ErrInvalid  = errors.New("invalid template")
	ErrNotFound = errors.New("template not found")
	ErrConflict = errors.New("topic code already exists")
)

const (
	DefaultPageSize = 12
	MaxPageSize     = 100
)

type Repository interface {
	Create(ctx context.Context, item *model.ProjectTemplate) error
	Get(ctx context.Context, id int64) (*model.ProjectTemplate, error)
	UpdateArchive(ctx context.Context, id int64, path string, fileTree []byte) error
	List(ctx context.Context) ([]model.ProjectTemplate, error)
	ListAdmin(ctx context.Context) ([]model.ProjectTemplate, error)
	Update(ctx context.Context, id int64, item *model.ProjectTemplate) error
	Delete(ctx context.Context, id int64) error
	Topics(ctx context.Context) ([]model.ProjectTopic, error)
	AllTopics(ctx context.Context) ([]model.ProjectTopic, error)
	Topic(ctx context.Context, id int64) (*model.ProjectTopic, error)
	CreateTopic(ctx context.Context, item *model.ProjectTopic) error
	UpdateTopic(ctx context.Context, id int64, item *model.ProjectTopic) error
}

type Sandbox interface {
	ExportArchive(ctx context.Context, id string) ([]byte, error)
	ListFiles(ctx context.Context, id string) ([]client.FileEntry, error)
}

type S3 interface {
	Put(ctx context.Context, key, contentType string, size int64, body io.Reader) error
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}

type Service struct {
	Repo     Repository
	Projects store.ProjectStore
	Sandbox  Sandbox
	S3       S3
	Expiry   time.Duration
	Logger   *slog.Logger
	Now      func() time.Time
	NewID    func() string
	HTTP     *http.Client
}

func NewService(repo Repository, projects store.ProjectStore, sandbox Sandbox, s3 S3, expiry time.Duration) *Service {
	return &Service{
		Repo: repo, Projects: projects, Sandbox: sandbox, S3: s3, Expiry: expiry,
		Now: time.Now, NewID: uuid.NewString, HTTP: &http.Client{Timeout: 2 * time.Minute},
	}
}

type CreateInput struct {
	TemplateName   string
	Description    string
	TopicID        int64
	RecommendScore int
}

type UpdateInput struct {
	TemplateName   string
	Description    string
	TopicID        int64
	RecommendScore int
	Status         int8
}

type Query struct {
	Keyword       string
	ParentTopicID int64
	TopicID       int64
	Page          int
	PageSize      int
	Sort          string
}

type Template struct {
	ID              int64  `json:"id"`
	TemplateCode    string `json:"templateCode"`
	TemplateName    string `json:"templateName"`
	Description     string `json:"description"`
	TopicID         int64  `json:"topicId"`
	TopicName       string `json:"topicName"`
	ParentTopicID   int64  `json:"parentTopicId"`
	ParentTopicName string `json:"parentTopicName"`
	CoverURL        string `json:"coverUrl"`
	RecommendScore  int    `json:"recommendScore"`
	Status          int8   `json:"status"`
}

type Topic struct {
	ID             int64   `json:"id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	PromptTemplate string  `json:"promptTemplate"`
	ParentID       *int64  `json:"parentId,omitempty"`
	SortOrder      int     `json:"sortOrder"`
	Status         int8    `json:"status"`
	Children       []Topic `json:"children"`
}

type TopicInput struct {
	ParentID       *int64
	Code           string
	Name           string
	Description    string
	PromptTemplate string
	SortOrder      int
	Status         int8
}

type Page struct {
	Items    []Template `json:"templates"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

func (s *Service) Create(ctx context.Context, projectID string, input CreateInput, createdBy string) (Template, error) {
	if s.S3 == nil || s.Sandbox == nil {
		return Template{}, ErrInvalid
	}
	project, err := s.Projects.Get(ctx, projectID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Template{}, ErrNotFound
		}
		return Template{}, err
	}
	if project.InternalID <= 0 || project.State == appmodel.ProjectStateDisabled {
		return Template{}, ErrInvalid
	}
	name := strings.TrimSpace(input.TemplateName)
	if name == "" || len([]rune(name)) > 128 {
		return Template{}, ErrInvalid
	}
	if len([]rune(strings.TrimSpace(input.Description))) > 1000 {
		return Template{}, ErrInvalid
	}
	if input.RecommendScore < 0 || input.RecommendScore > 100 {
		return Template{}, ErrInvalid
	}
	topic, err := s.Repo.Topic(ctx, input.TopicID)
	if err != nil || topic.ParentID == nil {
		return Template{}, ErrInvalid
	}
	item := &model.ProjectTemplate{
		Scope: "platform", WorkspaceID: 0,
		TemplateCode: s.newID(), TemplateName: name, Description: strings.TrimSpace(input.Description),
		TopicID: input.TopicID, SourceProjectID: &project.InternalID,
		CoverURL: project.CoverS3Path, RecommendScore: input.RecommendScore,
		Version: 1, IsBuiltin: 0, Status: 0, CreatedBy: createdBy,
	}
	if err := s.Repo.Create(ctx, item); err != nil {
		return Template{}, err
	}
	go s.upload(item.ID, project.ID)
	return s.view(ctx, item, nil), nil
}

func (s *Service) List(ctx context.Context, query Query) (Page, error) {
	query.Keyword = strings.TrimSpace(query.Keyword)
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = DefaultPageSize
	}
	if query.PageSize > MaxPageSize {
		query.PageSize = MaxPageSize
	}
	items, err := s.Repo.List(ctx)
	if err != nil {
		return Page{}, err
	}
	if query.Sort == "latest" {
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].CreatedAt.Equal(items[j].CreatedAt) {
				return items[i].ID > items[j].ID
			}
			return items[i].CreatedAt.After(items[j].CreatedAt)
		})
	}
	topics, err := s.Repo.Topics(ctx)
	if err != nil {
		return Page{}, err
	}
	topicByID := make(map[int64]model.ProjectTopic, len(topics))
	for _, topic := range topics {
		topicByID[topic.ID] = topic
	}
	filtered := make([]Template, 0, len(items))
	keyword := strings.ToLower(query.Keyword)
	for index := range items {
		item := items[index]
		topic := topicByID[item.TopicID]
		parentID := int64(0)
		if topic.ParentID != nil {
			parentID = *topic.ParentID
		}
		if query.ParentTopicID > 0 && parentID != query.ParentTopicID {
			continue
		}
		if query.TopicID > 0 && item.TopicID != query.TopicID {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(item.TemplateName+" "+item.Description), keyword) {
			continue
		}
		filtered = append(filtered, s.view(ctx, &item, topicByID))
	}
	total := int64(len(filtered))
	start := (query.Page - 1) * query.PageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + query.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return Page{Items: filtered[start:end], Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) AdminList(ctx context.Context, query Query) (Page, error) {
	query.Keyword = strings.TrimSpace(query.Keyword)
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = DefaultPageSize
	}
	if query.PageSize > MaxPageSize {
		query.PageSize = MaxPageSize
	}
	items, err := s.Repo.ListAdmin(ctx)
	if err != nil {
		return Page{}, err
	}
	topics, err := s.Repo.Topics(ctx)
	if err != nil {
		return Page{}, err
	}
	topicByID := make(map[int64]model.ProjectTopic, len(topics))
	for _, topic := range topics {
		topicByID[topic.ID] = topic
	}
	keyword := strings.ToLower(query.Keyword)
	filtered := make([]Template, 0, len(items))
	for index := range items {
		item := items[index]
		if keyword != "" && !strings.Contains(strings.ToLower(item.TemplateName+" "+item.Description), keyword) {
			continue
		}
		filtered = append(filtered, s.view(ctx, &item, topicByID))
	}
	total := int64(len(filtered))
	start := (query.Page - 1) * query.PageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + query.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return Page{Items: filtered[start:end], Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (s *Service) UpdateTemplate(ctx context.Context, id int64, input UpdateInput, updatedBy string) (Template, error) {
	name := strings.TrimSpace(input.TemplateName)
	if name == "" || len([]rune(name)) > 128 || len([]rune(strings.TrimSpace(input.Description))) > 1000 {
		return Template{}, ErrInvalid
	}
	if input.RecommendScore < 0 || input.RecommendScore > 100 || (input.Status != 0 && input.Status != 1) {
		return Template{}, ErrInvalid
	}
	topic, err := s.Repo.Topic(ctx, input.TopicID)
	if err != nil || topic.ParentID == nil {
		return Template{}, ErrInvalid
	}
	item := &model.ProjectTemplate{
		TemplateName: name, Description: strings.TrimSpace(input.Description),
		TopicID: input.TopicID, RecommendScore: input.RecommendScore, Status: input.Status, UpdatedBy: updatedBy,
	}
	if err := s.Repo.Update(ctx, id, item); err != nil {
		return Template{}, err
	}
	item.ID = id
	topics, _ := s.Repo.Topics(ctx)
	topicByID := make(map[int64]model.ProjectTopic, len(topics))
	for _, topic := range topics {
		topicByID[topic.ID] = topic
	}
	return s.view(ctx, item, topicByID), nil
}

func (s *Service) DeleteTemplate(ctx context.Context, id int64) error {
	return s.Repo.Delete(ctx, id)
}

func (s *Service) FileTree(ctx context.Context, id int64) ([]byte, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if len(item.FileTree) == 0 {
		return []byte("[]"), nil
	}
	return item.FileTree, nil
}

func (s *Service) Topics(ctx context.Context) ([]Topic, error) {
	items, err := s.Repo.Topics(ctx)
	if err != nil {
		return nil, err
	}
	return buildTopics(items), nil
}

func (s *Service) AdminTopics(ctx context.Context) ([]Topic, error) {
	items, err := s.Repo.AllTopics(ctx)
	if err != nil {
		return nil, err
	}
	return buildTopics(items), nil
}

func buildTopics(items []model.ProjectTopic) []Topic {
	parents := make([]Topic, 0)
	indexByID := make(map[int64]int)
	for _, item := range items {
		if item.ParentID != nil {
			continue
		}
		parents = append(parents, Topic{
			ID: item.ID, Code: item.TopicCode, Name: item.TopicName, Description: item.Description,
			PromptTemplate: item.PromptTemplate, ParentID: item.ParentID, SortOrder: item.SortOrder,
			Status: item.Status, Children: []Topic{},
		})
		indexByID[item.ID] = len(parents) - 1
	}
	for _, item := range items {
		if item.ParentID == nil {
			continue
		}
		index, ok := indexByID[*item.ParentID]
		if !ok {
			continue
		}
		parents[index].Children = append(parents[index].Children, Topic{
			ID: item.ID, Code: item.TopicCode, Name: item.TopicName, Description: item.Description,
			PromptTemplate: item.PromptTemplate, ParentID: item.ParentID, SortOrder: item.SortOrder,
			Status: item.Status, Children: []Topic{},
		})
	}
	return parents
}

func (s *Service) CreateTopic(ctx context.Context, input TopicInput, updatedBy string) (Topic, error) {
	item, err := s.validateTopic(ctx, 0, input, updatedBy)
	if err != nil {
		return Topic{}, err
	}
	if err := s.Repo.CreateTopic(ctx, item); err != nil {
		return Topic{}, err
	}
	return topicFromModel(item), nil
}

func (s *Service) UpdateTopic(ctx context.Context, id int64, input TopicInput, updatedBy string) (Topic, error) {
	item, err := s.validateTopic(ctx, id, input, updatedBy)
	if err != nil {
		return Topic{}, err
	}
	if err := s.Repo.UpdateTopic(ctx, id, item); err != nil {
		return Topic{}, err
	}
	item.ID = id
	return topicFromModel(item), nil
}

func topicFromModel(item *model.ProjectTopic) Topic {
	return Topic{
		ID: item.ID, Code: item.TopicCode, Name: item.TopicName, Description: item.Description,
		PromptTemplate: item.PromptTemplate, ParentID: item.ParentID, SortOrder: item.SortOrder,
		Status: item.Status, Children: []Topic{},
	}
}

func (s *Service) validateTopic(ctx context.Context, id int64, input TopicInput, updatedBy string) (*model.ProjectTopic, error) {
	code := strings.TrimSpace(input.Code)
	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	if !topicCodePattern.MatchString(code) || name == "" || len([]rune(name)) > 64 || len([]rune(description)) > 1000 {
		return nil, ErrInvalid
	}
	if input.Status != 0 && input.Status != 1 {
		return nil, ErrInvalid
	}
	if input.ParentID != nil {
		if *input.ParentID <= 0 || *input.ParentID == id {
			return nil, ErrInvalid
		}
		parent, err := s.Repo.Topic(ctx, *input.ParentID)
		if err != nil || parent.ParentID != nil {
			return nil, ErrInvalid
		}
		if id > 0 {
			all, err := s.Repo.AllTopics(ctx)
			if err != nil {
				return nil, err
			}
			for _, topic := range all {
				if topic.ParentID != nil && *topic.ParentID == id {
					return nil, ErrInvalid
				}
			}
		}
	}
	return &model.ProjectTopic{
		ParentID: input.ParentID, TopicCode: code, TopicName: name, Description: description,
		PromptTemplate: strings.TrimSpace(input.PromptTemplate), SortOrder: input.SortOrder,
		Status: input.Status, CreatedBy: updatedBy, UpdatedBy: updatedBy,
	}, nil
}

func (s *Service) Template(ctx context.Context, templateID int64) (*model.ProjectTemplate, error) {
	item, err := s.Repo.Get(ctx, templateID)
	if err != nil {
		return nil, ErrNotFound
	}
	return item, nil
}

func (s *Service) Download(ctx context.Context, templateID int64) ([]byte, error) {
	item, err := s.Repo.Get(ctx, templateID)
	if err != nil {
		return nil, ErrNotFound
	}
	if item.Status != 1 || item.TemplateS3Path == "" {
		return nil, ErrInvalid
	}
	if s.S3 == nil {
		return nil, ErrInvalid
	}
	url, err := s.S3.PresignGet(ctx, item.TemplateS3Path, s.expiry())
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download template: status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 100<<20))
}

func (s *Service) upload(templateID int64, projectID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := s.Upload(ctx, templateID, projectID); err != nil && s.Logger != nil {
		s.Logger.Warn("template.upload", "template_id", templateID, "project_id", projectID, "status", "error", "error", err)
	}
}

func (s *Service) Upload(ctx context.Context, templateID int64, projectID string) error {
	archive, err := s.Sandbox.ExportArchive(ctx, projectID)
	if err != nil {
		return err
	}
	if s.S3 == nil {
		return ErrInvalid
	}
	key := fmt.Sprintf("templates/%d.tar.gz", templateID)
	if err := s.S3.Put(ctx, key, "application/gzip", int64(len(archive)), bytes.NewReader(archive)); err != nil {
		return err
	}
	files, err := s.Sandbox.ListFiles(ctx, projectID)
	if err != nil {
		files = nil
	}
	tree, _ := json.Marshal(files)
	return s.Repo.UpdateArchive(ctx, templateID, key, tree)
}

func (s *Service) view(ctx context.Context, item *model.ProjectTemplate, topics map[int64]model.ProjectTopic) Template {
	topic := topics[item.TopicID]
	parentID := int64(0)
	parentName := ""
	if topic.ParentID != nil {
		parentID = *topic.ParentID
		if parent, ok := topics[parentID]; ok {
			parentName = parent.TopicName
		}
	}
	coverURL := ""
	if item.CoverURL != "" && s.S3 != nil {
		if signed, err := s.S3.PresignGet(ctx, item.CoverURL, s.expiry()); err == nil {
			coverURL = signed
		}
	}
	return Template{
		ID: item.ID, TemplateCode: item.TemplateCode, TemplateName: item.TemplateName,
		Description: item.Description, TopicID: item.TopicID, TopicName: topic.TopicName,
		ParentTopicID: parentID, ParentTopicName: parentName, CoverURL: coverURL,
		RecommendScore: item.RecommendScore, Status: item.Status,
	}
}

func (s *Service) expiry() time.Duration {
	if s.Expiry <= 0 {
		return 15 * time.Minute
	}
	return s.Expiry
}

func (s *Service) newID() string {
	if s.NewID != nil {
		return s.NewID()
	}
	return uuid.NewString()
}
