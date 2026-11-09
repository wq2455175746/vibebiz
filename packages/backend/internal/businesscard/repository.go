package businesscard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

var ErrNotFound = errors.New("business card not found")

type CardRecord struct {
	ID                int64
	PublicID          string
	UserID            int64
	OwnerName         string
	OwnerNickname     string
	OwnerAvatarS3Path string
	Headline          string
	Tags              []string
	FeaturedType      string
	FeaturedProjectID string
	FeaturedS3Key     string
	FeaturedFileName  string
	FeaturedFileSize  int64
	Published         bool
	PublishedAt       *time.Time
	CommentCount      int
	Hidden            bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Items             []ItemRecord
}

type ItemRecord struct {
	ProjectID string
	SortOrder int
}

type OwnerProfile struct {
	UserID       int64
	Username     string
	Nickname     string
	AvatarS3Path string
}

type Repository interface {
	GetOrCreate(ctx context.Context, userID int64) (*CardRecord, error)
	GetByUser(ctx context.Context, userID int64) (*CardRecord, error)
	GetByPublicID(ctx context.Context, publicID string) (*CardRecord, error)
	UpdateProfile(ctx context.Context, cardID int64, headline string, tags []string) error
	ListItems(ctx context.Context, cardID int64) ([]ItemRecord, error)
	AddItem(ctx context.Context, cardID int64, projectID string, sortOrder int) error
	RemoveItem(ctx context.Context, cardID int64, projectID string) error
	SetFeaturedProject(ctx context.Context, cardID int64, projectID string) error
	SetFeaturedFile(ctx context.Context, cardID int64, key, name, fileType string, size int64) error
	ClearFeatured(ctx context.Context, cardID int64) error
	Publish(ctx context.Context, cardID int64, at time.Time) error
	SetHidden(ctx context.Context, cardID int64, hidden bool) error
	Delete(ctx context.Context, cardID int64) (featuredKey string, err error)
	Discover(ctx context.Context, page, pageSize int) ([]*CardRecord, int64, error)
	AdminList(ctx context.Context, keyword string, page, pageSize int) ([]*CardRecord, int64, error)
	FindByProject(ctx context.Context, projectID string) (*CardRecord, error)
	OwnerProfile(ctx context.Context, userID int64) (*OwnerProfile, error)
}

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

type cardRow struct {
	ID                      int64      `gorm:"column:id"`
	PublicID                string     `gorm:"column:public_id"`
	UserID                  int64      `gorm:"column:user_id"`
	Headline                string     `gorm:"column:headline"`
	Tags                    model.JSON `gorm:"column:tags"`
	FeaturedType            string     `gorm:"column:featured_type"`
	FeaturedProjectPublicID string     `gorm:"column:featured_project_public_id"`
	FeaturedS3Key           string     `gorm:"column:featured_s3_key"`
	FeaturedFileName        string     `gorm:"column:featured_file_name"`
	FeaturedFileSize        int64      `gorm:"column:featured_file_size"`
	Published               int8       `gorm:"column:published"`
	PublishedAt             *time.Time `gorm:"column:published_at"`
	CommentCount            int        `gorm:"column:comment_count"`
	IsHidden                int8       `gorm:"column:is_hidden"`
	CreatedAt               time.Time  `gorm:"column:created_at"`
	UpdatedAt               time.Time  `gorm:"column:updated_at"`
	OwnerUsername           string     `gorm:"column:owner_username"`
	OwnerNickname           string     `gorm:"column:owner_nickname"`
	OwnerAvatarS3Path       string     `gorm:"column:owner_avatar_s3_path"`
}

const cardSelect = "bc.*, u.username AS owner_username, u.nickname AS owner_nickname, u.avatar_s3_path AS owner_avatar_s3_path"

func (r cardRow) record(items []ItemRecord) *CardRecord {
	tags := []string{}
	if len(r.Tags) > 0 {
		_ = json.Unmarshal(r.Tags, &tags)
	}
	if tags == nil {
		tags = []string{}
	}
	name := r.OwnerNickname
	if name == "" {
		name = r.OwnerUsername
	}
	return &CardRecord{
		ID: r.ID, PublicID: r.PublicID, UserID: r.UserID,
		OwnerName: name, OwnerNickname: r.OwnerNickname, OwnerAvatarS3Path: r.OwnerAvatarS3Path,
		Headline: r.Headline, Tags: tags,
		FeaturedType: r.FeaturedType, FeaturedProjectID: r.FeaturedProjectPublicID,
		FeaturedS3Key: r.FeaturedS3Key, FeaturedFileName: r.FeaturedFileName, FeaturedFileSize: r.FeaturedFileSize,
		Published: r.Published == 1, PublishedAt: r.PublishedAt, CommentCount: r.CommentCount,
		Hidden: r.IsHidden == 1, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Items: items,
	}
}

func (r *MySQLRepository) baseCardQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("business_card AS bc").
		Joins("LEFT JOIN user AS u ON u.id = bc.user_id").
		Where("bc.deleted_at IS NULL")
}

func (r *MySQLRepository) getCard(ctx context.Context, where string, args ...any) (*CardRecord, error) {
	var row cardRow
	err := r.baseCardQuery(ctx).Select(cardSelect).Where(where, args...).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get card: %w", err)
	}
	items, err := r.loadItems(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	return row.record(items), nil
}

func (r *MySQLRepository) GetByUser(ctx context.Context, userID int64) (*CardRecord, error) {
	return r.getCard(ctx, "bc.user_id = ?", userID)
}

func (r *MySQLRepository) GetByPublicID(ctx context.Context, publicID string) (*CardRecord, error) {
	return r.getCard(ctx, "bc.public_id = ?", publicID)
}

func (r *MySQLRepository) GetOrCreate(ctx context.Context, userID int64) (*CardRecord, error) {
	card, err := r.GetByUser(ctx, userID)
	if err == nil {
		return card, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	row := &model.BusinessCard{
		PublicID: uuid.NewString(),
		UserID:   userID,
		Tags:     model.JSON("[]"),
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		if card, retryErr := r.GetByUser(ctx, userID); retryErr == nil {
			return card, nil
		}
		return nil, fmt.Errorf("create card: %w", err)
	}
	return r.GetByUser(ctx, userID)
}

func (r *MySQLRepository) UpdateProfile(ctx context.Context, cardID int64, headline string, tags []string) error {
	payload, _ := json.Marshal(tags)
	return r.db.WithContext(ctx).Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Updates(map[string]any{"headline": headline, "tags": model.JSON(payload)}).Error
}

func (r *MySQLRepository) loadItems(ctx context.Context, cardID int64) ([]ItemRecord, error) {
	var rows []model.BusinessCardItem
	if err := r.db.WithContext(ctx).Where("card_id = ?", cardID).
		Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list card items: %w", err)
	}
	out := make([]ItemRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, ItemRecord{ProjectID: row.ProjectPublicID, SortOrder: row.SortOrder})
	}
	return out, nil
}

func (r *MySQLRepository) ListItems(ctx context.Context, cardID int64) ([]ItemRecord, error) {
	return r.loadItems(ctx, cardID)
}

func (r *MySQLRepository) loadItemsByCards(ctx context.Context, cardIDs []int64) (map[int64][]ItemRecord, error) {
	out := make(map[int64][]ItemRecord, len(cardIDs))
	if len(cardIDs) == 0 {
		return out, nil
	}
	var rows []model.BusinessCardItem
	if err := r.db.WithContext(ctx).Where("card_id IN ?", cardIDs).
		Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list card items: %w", err)
	}
	for _, row := range rows {
		out[row.CardID] = append(out[row.CardID], ItemRecord{ProjectID: row.ProjectPublicID, SortOrder: row.SortOrder})
	}
	return out, nil
}

func (r *MySQLRepository) AddItem(ctx context.Context, cardID int64, projectID string, sortOrder int) error {
	row := &model.BusinessCardItem{CardID: cardID, ProjectPublicID: projectID, SortOrder: sortOrder}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("add card item: %w", err)
	}
	return nil
}

func (r *MySQLRepository) RemoveItem(ctx context.Context, cardID int64, projectID string) error {
	return r.db.WithContext(ctx).Unscoped().
		Where("card_id = ? AND project_public_id = ?", cardID, projectID).
		Delete(&model.BusinessCardItem{}).Error
}

func (r *MySQLRepository) SetFeaturedProject(ctx context.Context, cardID int64, projectID string) error {
	return r.db.WithContext(ctx).Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Updates(map[string]any{
			"featured_type": "project", "featured_project_public_id": projectID,
			"featured_s3_key": "", "featured_file_name": "", "featured_file_size": 0,
		}).Error
}

func (r *MySQLRepository) SetFeaturedFile(ctx context.Context, cardID int64, key, name, fileType string, size int64) error {
	return r.db.WithContext(ctx).Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Updates(map[string]any{
			"featured_type": fileType, "featured_project_public_id": "",
			"featured_s3_key": key, "featured_file_name": name, "featured_file_size": size,
		}).Error
}

func (r *MySQLRepository) ClearFeatured(ctx context.Context, cardID int64) error {
	return r.db.WithContext(ctx).Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Updates(map[string]any{
			"featured_type": "", "featured_project_public_id": "",
			"featured_s3_key": "", "featured_file_name": "", "featured_file_size": 0,
		}).Error
}

func (r *MySQLRepository) Publish(ctx context.Context, cardID int64, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Updates(map[string]any{"published": 1, "published_at": at}).Error
}

func (r *MySQLRepository) SetHidden(ctx context.Context, cardID int64, hidden bool) error {
	value := int8(0)
	if hidden {
		value = 1
	}
	return r.db.WithContext(ctx).Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Update("is_hidden", value).Error
}

func (r *MySQLRepository) Delete(ctx context.Context, cardID int64) (string, error) {
	var row model.BusinessCard
	if err := r.db.WithContext(ctx).Select("id", "featured_s3_key").First(&row, cardID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get card for delete: %w", err)
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("card_id = ?", cardID).Delete(&model.BusinessCardItem{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&model.BusinessCard{}, cardID).Error
	})
	if err != nil {
		return "", err
	}
	return row.FeaturedS3Key, nil
}

func (r *MySQLRepository) listCards(ctx context.Context, scope func(*gorm.DB) *gorm.DB, order string, page, pageSize int) ([]*CardRecord, int64, error) {
	var total int64
	if err := scope(r.baseCardQuery(ctx)).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count cards: %w", err)
	}
	var rows []cardRow
	if err := scope(r.baseCardQuery(ctx).Select(cardSelect)).
		Order(order).Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list cards: %w", err)
	}
	cardIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		cardIDs = append(cardIDs, row.ID)
	}
	itemsByCard, err := r.loadItemsByCards(ctx, cardIDs)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*CardRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.record(itemsByCard[row.ID]))
	}
	return out, total, nil
}

func (r *MySQLRepository) Discover(ctx context.Context, page, pageSize int) ([]*CardRecord, int64, error) {
	scope := func(tx *gorm.DB) *gorm.DB {
		return tx.Where("bc.published = 1 AND bc.is_hidden = 0")
	}
	return r.listCards(ctx, scope, "bc.updated_at DESC, bc.id DESC", page, pageSize)
}

func (r *MySQLRepository) AdminList(ctx context.Context, keyword string, page, pageSize int) ([]*CardRecord, int64, error) {
	scope := func(tx *gorm.DB) *gorm.DB {
		if strings.TrimSpace(keyword) != "" {
			like := "%" + strings.TrimSpace(keyword) + "%"
			tx = tx.Where("(u.username LIKE ? OR u.nickname LIKE ? OR bc.headline LIKE ?)", like, like, like)
		}
		return tx
	}
	return r.listCards(ctx, scope, "bc.updated_at DESC, bc.id DESC", page, pageSize)
}

func (r *MySQLRepository) FindByProject(ctx context.Context, projectID string) (*CardRecord, error) {
	var row cardRow
	err := r.db.WithContext(ctx).Table("business_card_item AS i").
		Select(cardSelect).
		Joins("JOIN business_card AS bc ON bc.id = i.card_id AND bc.deleted_at IS NULL").
		Joins("LEFT JOIN user AS u ON u.id = bc.user_id").
		Where("i.deleted_at IS NULL AND i.project_public_id = ?", projectID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find card by project: %w", err)
	}
	items, err := r.loadItems(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	return row.record(items), nil
}

func (r *MySQLRepository) OwnerProfile(ctx context.Context, userID int64) (*OwnerProfile, error) {
	var row struct {
		ID           int64  `gorm:"column:id"`
		Username     string `gorm:"column:username"`
		Nickname     string `gorm:"column:nickname"`
		AvatarS3Path string `gorm:"column:avatar_s3_path"`
	}
	err := r.db.WithContext(ctx).Table("user").
		Select("id, username, nickname, avatar_s3_path").
		Where("id = ? AND deleted_at IS NULL", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get owner profile: %w", err)
	}
	return &OwnerProfile{UserID: row.ID, Username: row.Username, Nickname: row.Nickname, AvatarS3Path: row.AvatarS3Path}, nil
}
