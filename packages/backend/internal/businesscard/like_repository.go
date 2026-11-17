package businesscard

import (
	"context"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type LikeReadRepository interface {
	Counts(ctx context.Context, cardIDs []int64) (map[int64]int64, error)
	Liked(ctx context.Context, cardID, userID int64) (bool, error)
}

type LikeEvent struct {
	EventID string
	CardID  int64
	UserID  int64
	Action  string
	TS      int64
}

type LikeSyncRepository interface {
	Apply(ctx context.Context, events []LikeEvent) ([]int64, error)
	Recount(ctx context.Context, cardIDs []int64) (map[int64]int64, error)
}

type MySQLLikeRepository struct {
	db *gorm.DB
}

func NewMySQLLikeRepository(db *gorm.DB) *MySQLLikeRepository {
	return &MySQLLikeRepository{db: db}
}

func (r *MySQLLikeRepository) Counts(ctx context.Context, cardIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(cardIDs))
	if len(cardIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID        int64 `gorm:"column:id"`
		LikeCount int64 `gorm:"column:like_count"`
	}
	err := r.db.WithContext(ctx).Table("business_card").
		Select("id, like_count").
		Where("id IN ?", cardIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.LikeCount
	}
	return out, nil
}

func (r *MySQLLikeRepository) Liked(ctx context.Context, cardID, userID int64) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&model.BusinessCardLike{}).
		Where("card_id = ? AND user_id = ? AND deleted_at IS NULL", cardID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *MySQLLikeRepository) Apply(ctx context.Context, events []LikeEvent) ([]int64, error) {
	affected := make(map[int64]struct{})
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, event := range events {
			var result *gorm.DB
			switch event.Action {
			case "like":
				result = tx.Exec(`INSERT INTO business_card_like (card_id, user_id)
					SELECT id, ? FROM business_card WHERE id = ?
					ON DUPLICATE KEY UPDATE deleted_at = NULL, updated_at = NOW()`,
					event.UserID, event.CardID)
			case "unlike":
				result = tx.Exec(`UPDATE business_card_like
					SET deleted_at = NOW()
					WHERE card_id = ? AND user_id = ? AND deleted_at IS NULL`,
					event.CardID, event.UserID)
			default:
				continue
			}
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				affected[event.CardID] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(affected))
	for id := range affected {
		out = append(out, id)
	}
	return out, nil
}

func (r *MySQLLikeRepository) Recount(ctx context.Context, cardIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(cardIDs))
	for _, id := range cardIDs {
		var count int64
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`UPDATE business_card bc
				SET like_count = (SELECT COUNT(*) FROM business_card_like l WHERE l.card_id = bc.id AND l.deleted_at IS NULL)
				WHERE bc.id = ?`, id).Error; err != nil {
				return err
			}
			var counts []int64
			if err := tx.Model(&model.BusinessCard{}).Where("id = ?", id).
				Pluck("like_count", &counts).Error; err != nil {
				return err
			}
			if len(counts) > 0 {
				count = counts[0]
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		out[id] = count
	}
	return out, nil
}
