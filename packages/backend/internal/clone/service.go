package clone

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"vibebiz/internal/agentrole"
	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/model"
	"vibebiz/internal/resource"
)

const (
	RoleCover    = "cover"
	RoleGallery  = "gallery"
	RoleUnitTest = "unit_test"
	RoleDocs     = "docs"

	StatusReady = "ready"

	maxNameRunes        = 80
	maxDescriptionRunes = 2000
	maxSourceURLLength  = 1024
	maxCloneImages      = 9
)

var (
	ErrInvalid            = errors.New("invalid clone request")
	ErrNotFound           = errors.New("clone project not found")
	ErrConflict           = errors.New("clone project conflict")
	ErrRolesUnavailable   = errors.New("clone roles unavailable")
	ErrSandboxUnavailable = errors.New("sandbox unavailable")
)

type Source struct {
	ProjectID string
	SourceURL string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Asset struct {
	ProjectID  string
	ResourceID string
	Role       string
	SortOrder  int
	CreatedAt  time.Time
}

type Store interface {
	CreateSource(ctx context.Context, source Source) error
	GetSource(ctx context.Context, projectID string) (Source, error)
	ListAssets(ctx context.Context, projectID string) ([]Asset, error)
	CreateAsset(ctx context.Context, asset Asset) error
	DeleteAsset(ctx context.Context, projectID, resourceID string) error
	SetCover(ctx context.Context, projectID, resourceID string) error
	DeleteByProject(ctx context.Context, projectID string) error
}

type ProjectStore interface {
	Create(ctx context.Context, project *model.Project) error
	Get(ctx context.Context, id string) (*model.Project, error)
	ListByWorkspaces(ctx context.Context, workspaceIDs []int64) []*model.Project
	Delete(ctx context.Context, id string) error
	SetCover(ctx context.Context, id, key string) error
}

type ProjectInitializer interface {
	InitProject(ctx context.Context, id string) (int64, error)
}

type StageInitializer interface {
	CreateInitialForMode(ctx context.Context, projectID, mode string) error
	DeleteByProject(ctx context.Context, projectID string) error
}

type HarnessImporter interface {
	Import(ctx context.Context, projectID string) error
}

type MetaStore interface {
	Set(ctx context.Context, projectID string, meta auth.ProjectMeta) error
	Delete(ctx context.Context, projectID string) error
}

type RoleStore interface {
	ListActive(ctx context.Context) ([]agentrole.Role, error)
}

type ResourceService interface {
	Adopt(ctx context.Context, workspaceID, userID int64, projectID, resourceID string) (resource.Item, error)
	List(ctx context.Context, projectID string) (resource.ListResult, error)
	Delete(ctx context.Context, workspaceID, userID int64, projectID, resourceID string) error
}

type CoverSigner interface {
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}

type SandboxClone interface {
	InitClone(ctx context.Context, projectID string, setup client.CloneSetup) error
}

type Service struct {
	Projects     ProjectStore
	Sandbox      ProjectInitializer
	Stages       StageInitializer
	Harness      HarnessImporter
	Metas        MetaStore
	Roles        RoleStore
	Resources    ResourceService
	Store        Store
	SandboxClone SandboxClone
	Covers       CoverSigner
	CoverExpiry  time.Duration
	Now          func() time.Time
	NewID        func() string
}

type CreateInput struct {
	Name               string
	Description        string
	SourceURL          string
	CoverResourceID    string
	GalleryResourceIDs []string
	UnitTestResourceID string
	DocsResourceID     string
}

type AssetView struct {
	ID          string    `json:"id"`
	ResourceID  string    `json:"resourceId"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Path        string    `json:"path"`
	ContentType string    `json:"contentType"`
	Size        int64     `json:"size"`
	Kind        string    `json:"kind"`
	Role        string    `json:"role"`
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
}

type View struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	SourceURL   string      `json:"sourceUrl"`
	Mode        string      `json:"mode"`
	State       string      `json:"state"`
	WorkspaceID int64       `json:"workspaceId"`
	CoverS3Path string      `json:"coverS3Path"`
	CoverURL    string      `json:"coverUrl"`
	Assets      []AssetView `json:"assets"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

type normalizedInput struct {
	name        string
	description string
	sourceURL   string
	images      []string
	cover       string
	unitTest    string
	docs        string
}

func NewService(projects ProjectStore, sandbox ProjectInitializer, stages StageInitializer, harness HarnessImporter, metas MetaStore, roles RoleStore, resources ResourceService, store Store) *Service {
	return &Service{
		Projects: projects, Sandbox: sandbox, Stages: stages, Harness: harness,
		Metas: metas, Roles: roles, Resources: resources, Store: store,
		CoverExpiry: 15 * time.Minute, Now: time.Now, NewID: uuid.NewString,
	}
}

func (s *Service) Create(ctx context.Context, workspaceID, userID int64, input CreateInput) (View, error) {
	draft, err := normalizeInput(input)
	if err != nil {
		return View{}, err
	}
	roleIDs, defaultRoleID, err := s.cloneRoleIDs(ctx)
	if err != nil {
		return View{}, err
	}
	id := s.newID()
	now := s.now()
	project := &model.Project{
		ID: id, Name: draft.name, Prompt: draft.description, Mode: model.ProjectModeClone,
		State: model.ProjectStateReady, WorkspaceID: workspaceID, OwnerUserID: userID,
		DefaultAgentRoleID: defaultRoleID, AgentRoleIDs: roleIDs,
		CreatedAt: now, UpdatedAt: now,
	}
	if s.Sandbox == nil || s.Projects == nil || s.Stages == nil {
		return View{}, ErrSandboxUnavailable
	}
	sandboxID, err := s.Sandbox.InitProject(ctx, id)
	if err != nil {
		return View{}, ErrSandboxUnavailable
	}
	project.SandboxID = &sandboxID

	adopted := make([]string, 0, len(draft.images)+2)
	rollback := func() {
		if s.Store != nil {
			_ = s.Store.DeleteByProject(ctx, id)
		}
		if s.Stages != nil {
			_ = s.Stages.DeleteByProject(ctx, id)
		}
		if s.Metas != nil {
			_ = s.Metas.Delete(ctx, id)
		}
		if s.Projects != nil {
			_ = s.Projects.Delete(ctx, id)
		}
		for _, resourceID := range adopted {
			_ = s.Resources.Delete(ctx, workspaceID, userID, id, resourceID)
		}
	}

	if err := s.Projects.Create(ctx, project); err != nil {
		return View{}, err
	}
	if s.Metas != nil {
		if err := s.Metas.Set(ctx, id, auth.ProjectMeta{WorkspaceID: workspaceID, OwnerUserID: userID}); err != nil {
			rollback()
			return View{}, err
		}
	}
	if err := s.Stages.CreateInitialForMode(ctx, id, model.ProjectModeClone); err != nil {
		rollback()
		return View{}, err
	}
	if s.Harness != nil {
		if err := s.Harness.Import(ctx, id); err != nil {
			rollback()
			return View{}, err
		}
	}

	assets := make([]Asset, 0, len(draft.images)+2)
	items := map[string]resource.Item{}
	nextSort := 0
	if draft.cover != "" {
		item, err := s.Resources.Adopt(ctx, workspaceID, userID, id, draft.cover)
		if err != nil {
			rollback()
			return View{}, err
		}
		adopted = append(adopted, item.ID)
		if item.Kind != "image" {
			rollback()
			return View{}, ErrInvalid
		}
		items[item.ID] = item
		assets = append(assets, Asset{ProjectID: id, ResourceID: item.ID, Role: RoleCover, SortOrder: nextSort})
		if err := s.Projects.SetCover(ctx, id, item.S3Key); err != nil {
			rollback()
			return View{}, err
		}
		project.CoverS3Path = item.S3Key
		nextSort++
	}
	for _, resourceID := range draft.images {
		item, err := s.Resources.Adopt(ctx, workspaceID, userID, id, resourceID)
		if err != nil {
			rollback()
			return View{}, err
		}
		adopted = append(adopted, item.ID)
		if item.Kind != "image" {
			rollback()
			return View{}, ErrInvalid
		}
		items[item.ID] = item
		assets = append(assets, Asset{ProjectID: id, ResourceID: item.ID, Role: RoleGallery, SortOrder: nextSort})
		nextSort++
	}
	if draft.unitTest != "" {
		item, err := s.Resources.Adopt(ctx, workspaceID, userID, id, draft.unitTest)
		if err != nil {
			rollback()
			return View{}, err
		}
		adopted = append(adopted, item.ID)
		if !isZip(item) {
			rollback()
			return View{}, ErrInvalid
		}
		items[item.ID] = item
		assets = append(assets, Asset{ProjectID: id, ResourceID: item.ID, Role: RoleUnitTest, SortOrder: nextSort})
		nextSort++
	}
	if draft.docs != "" {
		item, err := s.Resources.Adopt(ctx, workspaceID, userID, id, draft.docs)
		if err != nil {
			rollback()
			return View{}, err
		}
		adopted = append(adopted, item.ID)
		items[item.ID] = item
		assets = append(assets, Asset{ProjectID: id, ResourceID: item.ID, Role: RoleDocs, SortOrder: nextSort})
	}

	for _, asset := range assets {
		if err := s.Store.CreateAsset(ctx, asset); err != nil {
			rollback()
			return View{}, err
		}
	}
	source := Source{ProjectID: id, SourceURL: draft.sourceURL, Status: StatusReady, CreatedAt: now, UpdatedAt: now}
	if err := s.Store.CreateSource(ctx, source); err != nil {
		rollback()
		return View{}, err
	}
	if err := s.initSandboxClone(ctx, project, source, assets, items); err != nil {
		rollback()
		return View{}, err
	}
	return s.buildViewFrom(ctx, project, source, assets, items), nil
}

func (s *Service) List(ctx context.Context, workspaceIDs []int64) ([]View, error) {
	if s.Projects == nil || len(workspaceIDs) == 0 {
		return []View{}, nil
	}
	projects := s.Projects.ListByWorkspaces(ctx, workspaceIDs)
	views := make([]View, 0, len(projects))
	for _, project := range projects {
		if project.Mode != model.ProjectModeClone {
			continue
		}
		view, err := s.buildView(ctx, project)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	sort.SliceStable(views, func(i, j int) bool {
		return views[i].UpdatedAt.After(views[j].UpdatedAt)
	})
	return views, nil
}

func (s *Service) Get(ctx context.Context, workspaceIDs []int64, projectID string) (View, error) {
	project, err := s.projectFor(ctx, workspaceIDs, projectID)
	if err != nil {
		return View{}, err
	}
	return s.buildView(ctx, project)
}

func (s *Service) Delete(ctx context.Context, workspaceIDs []int64, projectID string) error {
	project, err := s.projectFor(ctx, workspaceIDs, projectID)
	if err != nil {
		return err
	}
	if s.Store != nil {
		if err := s.Store.DeleteByProject(ctx, projectID); err != nil {
			return err
		}
	}
	if s.Stages != nil {
		_ = s.Stages.DeleteByProject(ctx, projectID)
	}
	if s.Metas != nil {
		_ = s.Metas.Delete(ctx, projectID)
	}
	return s.Projects.Delete(ctx, project.ID)
}

func (s *Service) ListAssets(ctx context.Context, workspaceIDs []int64, projectID string) ([]AssetView, error) {
	project, err := s.projectFor(ctx, workspaceIDs, projectID)
	if err != nil {
		return nil, err
	}
	return s.assetViews(ctx, project.ID)
}

func (s *Service) AddAsset(ctx context.Context, workspaceID, userID int64, projectID, resourceID, role string) (AssetView, error) {
	if _, err := s.projectFor(ctx, []int64{workspaceID}, projectID); err != nil {
		return AssetView{}, err
	}
	var err error
	role, err = normalizeAssetRole(role)
	if err != nil {
		return AssetView{}, err
	}
	assets, err := s.Store.ListAssets(ctx, projectID)
	if err != nil {
		return AssetView{}, err
	}
	if err := checkAssetLimit(assets, role); err != nil {
		return AssetView{}, err
	}
	item, err := s.Resources.Adopt(ctx, workspaceID, userID, projectID, resourceID)
	if err != nil {
		return AssetView{}, err
	}
	if err := validateAssetItem(item, role); err != nil {
		_ = s.Resources.Delete(ctx, workspaceID, userID, projectID, item.ID)
		return AssetView{}, err
	}
	sortOrder := len(assets)
	asset := Asset{ProjectID: projectID, ResourceID: item.ID, Role: role, SortOrder: sortOrder, CreatedAt: s.now()}
	if err := s.Store.CreateAsset(ctx, asset); err != nil {
		_ = s.Resources.Delete(ctx, workspaceID, userID, projectID, item.ID)
		return AssetView{}, err
	}
	if role == RoleCover {
		if err := s.Projects.SetCover(ctx, projectID, item.S3Key); err != nil {
			_ = s.Store.DeleteAsset(ctx, projectID, item.ID)
			_ = s.Resources.Delete(ctx, workspaceID, userID, projectID, item.ID)
			return AssetView{}, err
		}
	}
	_ = s.syncSandboxByID(ctx, projectID)
	return assetView(asset, &item), nil
}

func (s *Service) DeleteAsset(ctx context.Context, workspaceID, userID int64, projectID, resourceID string) error {
	project, err := s.projectFor(ctx, []int64{workspaceID}, projectID)
	if err != nil {
		return err
	}
	assets, err := s.Store.ListAssets(ctx, projectID)
	if err != nil {
		return err
	}
	var target *Asset
	for index := range assets {
		if assets[index].ResourceID == resourceID {
			target = &assets[index]
			break
		}
	}
	if target == nil {
		return ErrNotFound
	}
	if err := s.Store.DeleteAsset(ctx, projectID, resourceID); err != nil {
		return err
	}
	_ = s.Resources.Delete(ctx, workspaceID, userID, projectID, resourceID)
	if target.Role != RoleCover {
		_ = s.syncSandboxByID(ctx, projectID)
		return nil
	}
	items, err := s.resourceItems(ctx, project.ID)
	if err != nil {
		return err
	}
	for _, asset := range assets {
		if asset.ResourceID == resourceID || (asset.Role != RoleCover && asset.Role != RoleGallery) {
			continue
		}
		item, ok := items[asset.ResourceID]
		if !ok {
			continue
		}
		if err := s.Store.SetCover(ctx, projectID, asset.ResourceID); err != nil {
			return err
		}
		if err := s.Projects.SetCover(ctx, projectID, item.S3Key); err != nil {
			return err
		}
		_ = s.syncSandboxByID(ctx, projectID)
		return nil
	}
	if err := s.Projects.SetCover(ctx, projectID, ""); err != nil {
		return err
	}
	_ = s.syncSandboxByID(ctx, projectID)
	return nil
}

func (s *Service) SetCover(ctx context.Context, workspaceID, userID int64, projectID, resourceID string) (AssetView, error) {
	project, err := s.projectFor(ctx, []int64{workspaceID}, projectID)
	if err != nil {
		return AssetView{}, err
	}
	assets, err := s.Store.ListAssets(ctx, projectID)
	if err != nil {
		return AssetView{}, err
	}
	var target *Asset
	for index := range assets {
		if assets[index].ResourceID == resourceID {
			target = &assets[index]
			break
		}
	}
	if target == nil || (target.Role != RoleCover && target.Role != RoleGallery) {
		return AssetView{}, ErrNotFound
	}
	items, err := s.resourceItems(ctx, project.ID)
	if err != nil {
		return AssetView{}, err
	}
	item, ok := items[resourceID]
	if !ok || item.Kind != "image" {
		return AssetView{}, ErrInvalid
	}
	if err := s.Store.SetCover(ctx, projectID, resourceID); err != nil {
		return AssetView{}, err
	}
	if err := s.Projects.SetCover(ctx, projectID, item.S3Key); err != nil {
		return AssetView{}, err
	}
	_ = s.syncSandboxByID(ctx, projectID)
	target.Role = RoleCover
	return assetView(*target, &item), nil
}

func (s *Service) initSandboxClone(ctx context.Context, project *model.Project, source Source, assets []Asset, items map[string]resource.Item) error {
	if s.SandboxClone == nil {
		return nil
	}
	setup := client.CloneSetup{
		Name: project.Name, Description: project.Prompt, SourceURL: source.SourceURL,
		Assets: make([]client.CloneAssetMeta, 0, len(assets)),
	}
	for _, asset := range assets {
		item := items[asset.ResourceID]
		setup.Assets = append(setup.Assets, client.CloneAssetMeta{
			Role: asset.Role, Name: item.Name, StorageName: item.StorageName,
			ContentType: item.ContentType, Kind: item.Kind, Size: item.Size,
		})
	}
	return s.SandboxClone.InitClone(ctx, project.ID, setup)
}

func (s *Service) syncSandboxByID(ctx context.Context, projectID string) error {
	if s.SandboxClone == nil || s.Projects == nil || s.Store == nil {
		return nil
	}
	project, err := s.Projects.Get(ctx, projectID)
	if err != nil {
		return err
	}
	source, err := s.Store.GetSource(ctx, projectID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	assets, err := s.Store.ListAssets(ctx, projectID)
	if err != nil {
		return err
	}
	items, err := s.resourceItems(ctx, projectID)
	if err != nil {
		return err
	}
	return s.initSandboxClone(ctx, project, source, assets, items)
}

func (s *Service) projectFor(ctx context.Context, workspaceIDs []int64, projectID string) (*model.Project, error) {
	if s.Projects == nil {
		return nil, ErrNotFound
	}
	project, err := s.Projects.Get(ctx, projectID)
	if err != nil {
		return nil, ErrNotFound
	}
	if project.Mode != model.ProjectModeClone || !containsInt64(workspaceIDs, project.WorkspaceID) {
		return nil, ErrNotFound
	}
	return project, nil
}

func (s *Service) cloneRoleIDs(ctx context.Context) ([]int64, *int64, error) {
	if s.Roles == nil {
		return nil, nil, ErrRolesUnavailable
	}
	roles, err := s.Roles.ListActive(ctx)
	if err != nil {
		return nil, nil, ErrRolesUnavailable
	}
	byCode := make(map[string]int64, len(roles))
	for _, role := range roles {
		byCode[role.RoleCode] = role.ID
	}
	wanted := []string{"team", "emma", "bob", "senior", "qa", "alex"}
	ids := make([]int64, 0, len(wanted))
	for _, code := range wanted {
		id, ok := byCode[code]
		if !ok {
			return nil, nil, ErrRolesUnavailable
		}
		ids = append(ids, id)
	}
	first := ids[0]
	return ids, &first, nil
}

func (s *Service) buildView(ctx context.Context, project *model.Project) (View, error) {
	source, err := s.Store.GetSource(ctx, project.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return View{}, err
	}
	assets, err := s.Store.ListAssets(ctx, project.ID)
	if err != nil {
		return View{}, err
	}
	items, err := s.resourceItems(ctx, project.ID)
	if err != nil {
		return View{}, err
	}
	return s.buildViewFrom(ctx, project, source, assets, items), nil
}

func (s *Service) buildViewFrom(ctx context.Context, project *model.Project, source Source, assets []Asset, items map[string]resource.Item) View {
	view := View{
		ID: project.ID, Name: project.Name, Description: project.Prompt, SourceURL: source.SourceURL,
		Mode: project.Mode, State: project.State, WorkspaceID: project.WorkspaceID,
		CoverS3Path: project.CoverS3Path, CreatedAt: project.CreatedAt, UpdatedAt: project.UpdatedAt,
	}
	if s.Covers != nil && project.CoverS3Path != "" {
		expiry := s.CoverExpiry
		if expiry <= 0 {
			expiry = 15 * time.Minute
		}
		if coverURL, err := s.Covers.PresignGet(ctx, project.CoverS3Path, expiry); err == nil {
			view.CoverURL = coverURL
		}
	}
	view.Assets = make([]AssetView, 0, len(assets))
	for _, asset := range assets {
		if item, ok := items[asset.ResourceID]; ok {
			view.Assets = append(view.Assets, assetView(asset, &item))
			continue
		}
		view.Assets = append(view.Assets, assetView(asset, nil))
	}
	return view
}

func (s *Service) assetViews(ctx context.Context, projectID string) ([]AssetView, error) {
	assets, err := s.Store.ListAssets(ctx, projectID)
	if err != nil {
		return nil, err
	}
	items, err := s.resourceItems(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]AssetView, 0, len(assets))
	for _, asset := range assets {
		if item, ok := items[asset.ResourceID]; ok {
			out = append(out, assetView(asset, &item))
			continue
		}
		out = append(out, assetView(asset, nil))
	}
	return out, nil
}

func (s *Service) resourceItems(ctx context.Context, projectID string) (map[string]resource.Item, error) {
	out := map[string]resource.Item{}
	if s.Resources == nil {
		return out, nil
	}
	result, err := s.Resources.List(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, item := range result.Resources {
		out[item.ID] = item
	}
	return out, nil
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) newID() string {
	if s.NewID != nil {
		return s.NewID()
	}
	return uuid.NewString()
}

func normalizeInput(input CreateInput) (normalizedInput, error) {
	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	sourceURL := strings.TrimSpace(input.SourceURL)
	if name == "" || utf8.RuneCountInString(name) > maxNameRunes {
		return normalizedInput{}, ErrInvalid
	}
	if description == "" || utf8.RuneCountInString(description) > maxDescriptionRunes {
		return normalizedInput{}, ErrInvalid
	}
	if !validSourceURL(sourceURL) {
		return normalizedInput{}, ErrInvalid
	}
	ids := make([]string, 0, len(input.GalleryResourceIDs)+3)
	cover := strings.TrimSpace(input.CoverResourceID)
	if cover != "" {
		ids = append(ids, cover)
	}
	for _, id := range input.GalleryResourceIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	unitTest := strings.TrimSpace(input.UnitTestResourceID)
	if unitTest != "" {
		ids = append(ids, unitTest)
	}
	docs := strings.TrimSpace(input.DocsResourceID)
	if docs != "" {
		ids = append(ids, docs)
	}
	seen := map[string]struct{}{}
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return normalizedInput{}, ErrInvalid
		}
		seen[id] = struct{}{}
	}
	if len(ids)-boolInt(unitTest != "")-boolInt(docs != "") > maxCloneImages {
		return normalizedInput{}, ErrInvalid
	}
	gallery := make([]string, 0, len(input.GalleryResourceIDs))
	for _, id := range input.GalleryResourceIDs {
		if id = strings.TrimSpace(id); id != "" {
			gallery = append(gallery, id)
		}
	}
	return normalizedInput{
		name: name, description: description, sourceURL: sourceURL,
		cover: cover, images: gallery, unitTest: unitTest, docs: docs,
	}, nil
}

func validSourceURL(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > maxSourceURLLength {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func normalizeAssetRole(role string) (string, error) {
	switch role {
	case RoleCover, RoleGallery, RoleUnitTest, RoleDocs:
		return role, nil
	default:
		return "", ErrInvalid
	}
}

func checkAssetLimit(assets []Asset, role string) error {
	images := 0
	for _, asset := range assets {
		if asset.Role == RoleCover || asset.Role == RoleGallery {
			images++
		}
		if role == RoleCover && asset.Role == RoleCover {
			return ErrConflict
		}
		if role == RoleUnitTest && asset.Role == RoleUnitTest {
			return ErrConflict
		}
	}
	if (role == RoleCover || role == RoleGallery) && images >= maxCloneImages {
		return ErrConflict
	}
	return nil
}

func validateAssetItem(item resource.Item, role string) error {
	switch role {
	case RoleCover, RoleGallery:
		if item.Kind != "image" {
			return ErrInvalid
		}
	case RoleUnitTest:
		if !isZip(item) {
			return ErrInvalid
		}
	}
	return nil
}

func isZip(item resource.Item) bool {
	name := strings.ToLower(item.Name + " " + item.ContentType)
	return strings.Contains(name, ".zip") || strings.Contains(name, "application/zip") || strings.Contains(name, "application/x-zip")
}

func assetView(asset Asset, item *resource.Item) AssetView {
	view := AssetView{
		ID: asset.ResourceID, ResourceID: asset.ResourceID, Role: asset.Role,
		SortOrder: asset.SortOrder, CreatedAt: asset.CreatedAt,
	}
	if item != nil {
		view.Name = item.Name
		view.DisplayName = item.DisplayName
		view.Path = item.Path
		view.ContentType = item.ContentType
		view.Size = item.Size
		view.Kind = item.Kind
	}
	return view
}

func containsInt64(ids []int64, target int64) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
