package businesscard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"vibebiz/internal/dao/model"
)

type CommentRecord struct {
	ID                 int64
	PublicID           string
	CardID             int64
	CardPublicID       string
	UserID             int64
	ParentID           int64
	ParentPublicID     string
	RootID             int64
	RootPublicID       string
	Depth              int
	Content            string
	Status             int8
	EditedAt           *time.Time
	ReplyCount         int
	CreatedAt          time.Time
	AuthorName         string
	AuthorAvatarS3Path string
	ReplyToName        string
}

type CommentInput struct {
	PublicID string
	CardID   int64
	UserID   int64
	ParentID int64
	RootID   int64
	Depth    int
	Content  string
}

type CommentDeleteResult struct {
	Tombstoned   bool `json:"tombstoned"`
	CommentCount int  `json:"commentCount"`
}

type CommentRepository interface {
	ListRoots(ctx context.Context, cardID, cursor int64, limit int) ([]*CommentRecord, bool, error)
	ListReplies(ctx context.Context, rootID, cursor int64, limit int) ([]*CommentRecord, bool, error)
	GetCommentByPublicID(ctx context.Context, publicID string) (*CommentRecord, error)
	CreateComment(ctx context.Context, input CommentInput) (*CommentRecord, error)
	UpdateComment(ctx context.Context, cardID, commentID, userID int64, content string, at time.Time) (*CommentRecord, error)
	DeleteComment(ctx context.Context, cardID, commentID, userID int64) (CommentDeleteResult, error)
}

var _ CommentRepository = (*MySQLRepository)(nil)

type commentRow struct {
	ID                 int64      `gorm:"column:id"`
	PublicID           string     `gorm:"column:public_id"`
	CardID             int64      `gorm:"column:card_id"`
	UserID             int64      `gorm:"column:user_id"`
	ParentID           int64      `gorm:"column:parent_id"`
	RootID             int64      `gorm:"column:root_id"`
	ParentPublicID     string     `gorm:"column:parent_public_id"`
	RootPublicID       string     `gorm:"column:root_public_id"`
	CardPublicID       string     `gorm:"column:card_public_id"`
	Depth              int        `gorm:"column:depth"`
	Content            string     `gorm:"column:content"`
	Status             int8       `gorm:"column:status"`
	EditedAt           *time.Time `gorm:"column:edited_at"`
	ReplyCount         int        `gorm:"column:reply_count"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	AuthorName         string     `gorm:"column:author_name"`
	AuthorNickname     string     `gorm:"column:author_nickname"`
	AuthorAvatarS3Path string     `gorm:"column:author_avatar_s3_path"`
	ReplyToName        string     `gorm:"column:reply_to_name"`
	ReplyToNickname    string     `gorm:"column:reply_to_nickname"`
}

func (r commentRow) record() *CommentRecord {
	name := r.AuthorNickname
	if name == "" {
		name = r.AuthorName
	}
	replyName := r.ReplyToNickname
	if replyName == "" {
		replyName = r.ReplyToName
	}
	return &CommentRecord{
		ID: r.ID, PublicID: r.PublicID, CardID: r.CardID, UserID: r.UserID,
		ParentID: r.ParentID, RootID: r.RootID, Depth: r.Depth, Content: r.Content,
		ParentPublicID: r.ParentPublicID, RootPublicID: r.RootPublicID, CardPublicID: r.CardPublicID,
		Status: r.Status, EditedAt: r.EditedAt, ReplyCount: r.ReplyCount, CreatedAt: r.CreatedAt,
		AuthorName: name, AuthorAvatarS3Path: r.AuthorAvatarS3Path, ReplyToName: replyName,
	}
}

const commentSelect = `c.*, bc.public_id AS card_public_id, pc.public_id AS parent_public_id,
	rc.public_id AS root_public_id, u.username AS author_name, u.nickname AS author_nickname,
	u.avatar_s3_path AS author_avatar_s3_path, pu.username AS reply_to_name, pu.nickname AS reply_to_nickname`

func (r *MySQLRepository) commentBase(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("business_card_comment AS c").
		Select(commentSelect).
		Joins("LEFT JOIN business_card AS bc ON bc.id = c.card_id").
		Joins("LEFT JOIN user AS u ON u.id = c.user_id").
		Joins("LEFT JOIN business_card_comment AS pc ON pc.id = c.parent_id").
		Joins("LEFT JOIN user AS pu ON pu.id = pc.user_id").
		Joins("LEFT JOIN business_card_comment AS rc ON rc.id = c.root_id").
		Where("c.deleted_at IS NULL")
}

func (r *MySQLRepository) ListRoots(ctx context.Context, cardID, cursor int64, limit int) ([]*CommentRecord, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	query := r.commentBase(ctx).
		Where("c.card_id = ? AND c.parent_id = 0", cardID).
		Where("(c.status = 1 OR c.reply_count > 0)")
	if cursor > 0 {
		query = query.Where("c.id < ?", cursor)
	}
	var rows []commentRow
	if err := query.Order("c.id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, false, fmt.Errorf("list root comments: %w", err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]*CommentRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.record())
	}
	return out, hasMore, nil
}

func (r *MySQLRepository) ListReplies(ctx context.Context, rootID, cursor int64, limit int) ([]*CommentRecord, bool, error) {
	if limit <= 0 {
		limit = 10
	}
	query := r.commentBase(ctx).
		Where("c.root_id = ?", rootID).
		Where("(c.status = 1 OR EXISTS (SELECT 1 FROM business_card_comment AS ch WHERE ch.parent_id = c.id AND ch.card_id = c.card_id AND ch.deleted_at IS NULL))")
	if cursor > 0 {
		query = query.Where("c.id > ?", cursor)
	}
	var rows []commentRow
	if err := query.Order("c.id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, false, fmt.Errorf("list comment replies: %w", err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]*CommentRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.record())
	}
	return out, hasMore, nil
}

func (r *MySQLRepository) getComment(ctx context.Context, where string, args ...any) (*CommentRecord, error) {
	var row commentRow
	err := r.commentBase(ctx).Where(where, args...).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get comment: %w", err)
	}
	return row.record(), nil
}

func (r *MySQLRepository) GetCommentByPublicID(ctx context.Context, publicID string) (*CommentRecord, error) {
	return r.getComment(ctx, "c.public_id = ?", publicID)
}

func (r *MySQLRepository) CreateComment(ctx context.Context, input CommentInput) (*CommentRecord, error) {
	row := &model.BusinessCardComment{
		PublicID: input.PublicID,
		CardID:   input.CardID,
		UserID:   input.UserID,
		ParentID: input.ParentID,
		RootID:   input.RootID,
		Depth:    uint8(input.Depth),
		Content:  input.Content,
		Status:   1,
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if input.ParentID > 0 {
			var parent struct {
				ID     int64 `gorm:"column:id"`
				Status int8  `gorm:"column:status"`
			}
			lockErr := tx.Model(&model.BusinessCardComment{}).
				Select("id, status").
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND card_id = ?", input.ParentID, input.CardID).
				Take(&parent).Error
			if errors.Is(lockErr, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			if lockErr != nil {
				return fmt.Errorf("lock parent comment: %w", lockErr)
			}
			if parent.Status != 1 {
				return ErrNotFound
			}
		}
		if err := tx.Create(row).Error; err != nil {
			return fmt.Errorf("insert comment: %w", err)
		}
		if err := tx.Model(&model.BusinessCard{}).Where("id = ?", input.CardID).
			Update("comment_count", gorm.Expr("comment_count + 1")).Error; err != nil {
			return fmt.Errorf("increment card comment count: %w", err)
		}
		if input.RootID > 0 {
			if err := tx.Model(&model.BusinessCardComment{}).Where("id = ?", input.RootID).
				Update("reply_count", gorm.Expr("reply_count + 1")).Error; err != nil {
				return fmt.Errorf("increment root reply count: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetCommentByPublicID(ctx, input.PublicID)
}

func (r *MySQLRepository) UpdateComment(ctx context.Context, cardID, commentID, userID int64, content string, at time.Time) (*CommentRecord, error) {
	res := r.db.WithContext(ctx).Model(&model.BusinessCardComment{}).
		Where("id = ? AND card_id = ? AND user_id = ? AND status = 1", commentID, cardID, userID).
		Updates(map[string]any{"content": content, "edited_at": at})
	if res.Error != nil {
		return nil, fmt.Errorf("update comment: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.getComment(ctx, "c.id = ?", commentID)
}

type commentDeleteRow struct {
	ID       int64 `gorm:"column:id"`
	CardID   int64 `gorm:"column:card_id"`
	UserID   int64 `gorm:"column:user_id"`
	ParentID int64 `gorm:"column:parent_id"`
	RootID   int64 `gorm:"column:root_id"`
	Status   int8  `gorm:"column:status"`
}

func decrementCardCommentCount(tx *gorm.DB, cardID int64) error {
	return tx.Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Update("comment_count", gorm.Expr("GREATEST(comment_count, 1) - 1")).Error
}

func decrementRootReplyCount(tx *gorm.DB, rootID int64) error {
	return tx.Model(&model.BusinessCardComment{}).Where("id = ?", rootID).
		Update("reply_count", gorm.Expr("GREATEST(reply_count, 1) - 1")).Error
}

func (r *MySQLRepository) DeleteComment(ctx context.Context, cardID, commentID, userID int64) (CommentDeleteResult, error) {
	var children int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row commentDeleteRow
		err := tx.Model(&model.BusinessCardComment{}).
			Select("id, card_id, user_id, parent_id, root_id, status").
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND card_id = ?", commentID, cardID).
			Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock comment: %w", err)
		}
		if row.UserID != userID || row.Status != 1 {
			return ErrNotFound
		}

		if err := tx.Model(&model.BusinessCardComment{}).
			Where("parent_id = ? AND card_id = ?", commentID, cardID).Count(&children).Error; err != nil {
			return fmt.Errorf("count comment replies: %w", err)
		}

		if err := decrementCardCommentCount(tx, cardID); err != nil {
			return fmt.Errorf("decrement card comment count: %w", err)
		}

		if children > 0 {
			if err := tx.Model(&model.BusinessCardComment{}).Where("id = ?", commentID).
				Updates(map[string]any{"status": int8(2), "content": ""}).Error; err != nil {
				return fmt.Errorf("tombstone comment: %w", err)
			}
			return nil
		}

		if err := tx.Unscoped().Delete(&model.BusinessCardComment{}, commentID).Error; err != nil {
			return fmt.Errorf("delete comment: %w", err)
		}
		if row.RootID > 0 {
			if err := decrementRootReplyCount(tx, row.RootID); err != nil {
				return fmt.Errorf("decrement root reply count: %w", err)
			}
		}
		parentID := row.ParentID
		for parentID > 0 {
			var ancestor commentDeleteRow
			lockErr := tx.Model(&model.BusinessCardComment{}).
				Select("id, parent_id, root_id, status").
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND card_id = ?", parentID, cardID).
				Take(&ancestor).Error
			if errors.Is(lockErr, gorm.ErrRecordNotFound) {
				break
			}
			if lockErr != nil {
				return fmt.Errorf("lock tombstone ancestor: %w", lockErr)
			}
			if ancestor.Status != 2 {
				break
			}
			var ancestorChildren int64
			if err := tx.Model(&model.BusinessCardComment{}).
				Where("parent_id = ? AND card_id = ?", ancestor.ID, cardID).Count(&ancestorChildren).Error; err != nil {
				return fmt.Errorf("count tombstone replies: %w", err)
			}
			if ancestorChildren > 0 {
				break
			}
			if err := tx.Unscoped().Delete(&model.BusinessCardComment{}, ancestor.ID).Error; err != nil {
				return fmt.Errorf("delete childless tombstone: %w", err)
			}
			if row.RootID > 0 && ancestor.ID != row.RootID {
				if err := decrementRootReplyCount(tx, row.RootID); err != nil {
					return fmt.Errorf("decrement root reply count: %w", err)
				}
			}
			parentID = ancestor.ParentID
		}
		return nil
	})
	if err != nil {
		return CommentDeleteResult{}, err
	}

	var card struct {
		CommentCount int `gorm:"column:comment_count"`
	}
	if err := r.db.WithContext(ctx).Model(&model.BusinessCard{}).
		Select("comment_count").Where("id = ?", cardID).Take(&card).Error; err != nil {
		return CommentDeleteResult{}, fmt.Errorf("read card comment count: %w", err)
	}
	return CommentDeleteResult{Tombstoned: children > 0, CommentCount: card.CommentCount}, nil
}
