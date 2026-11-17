package businesscard

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrCommentNotFound  = errors.New("comment not found")
	ErrCommentForbidden = errors.New("comment forbidden")
	ErrCommentInvalid   = errors.New("invalid comment")
	ErrCommentRateLimit = errors.New("comment rate limited")
)

const (
	defaultRootCommentLimit  = 20
	defaultReplyCommentLimit = 10
	maxCommentLimit          = 50
	rootCommentPreviewLimit  = 2
)

type CommentAuthor struct {
	UserID    int64  `json:"userId"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

type Comment struct {
	ID          string        `json:"id"`
	CardID      string        `json:"cardId"`
	ParentID    string        `json:"parentId,omitempty"`
	RootID      string        `json:"rootId,omitempty"`
	Depth       int           `json:"depth"`
	Content     string        `json:"content"`
	Author      CommentAuthor `json:"author"`
	ReplyToName string        `json:"replyToName,omitempty"`
	ReplyCount  int           `json:"replyCount"`
	Status      string        `json:"status"`
	Editable    bool          `json:"editable"`
	Deletable   bool          `json:"deletable"`
	EditedAt    *time.Time    `json:"editedAt"`
	CreatedAt   time.Time     `json:"createdAt"`
	Replies     []Comment     `json:"replies,omitempty"`
}

type CommentPage struct {
	Comments   []Comment `json:"comments"`
	NextCursor string    `json:"nextCursor"`
	HasMore    bool      `json:"hasMore"`
}

type CommentCardReader interface {
	GetByPublicID(ctx context.Context, publicID string) (*CardRecord, error)
}

type CommentService struct {
	Repo     CommentRepository
	Cards    CommentCardReader
	Limiter  CommentRateLimiter
	Now      func() time.Time
	NewID    func() string
	MaxDepth int
}

func (s *CommentService) ListRoots(ctx context.Context, cardPublicID string, viewerID int64, cursor string, limit int) (*CommentPage, error) {
	card, err := s.publishedCard(ctx, cardPublicID)
	if err != nil {
		return nil, err
	}
	cursorID, err := decodeCommentCursor(cursor)
	if err != nil {
		return nil, err
	}
	records, hasMore, err := s.Repo.ListRoots(ctx, card.ID, cursorID, clampCommentLimit(limit, defaultRootCommentLimit))
	if err != nil {
		return nil, err
	}
	comments := make([]Comment, 0, len(records))
	for _, rec := range records {
		comment := s.toComment(rec, viewerID)
		if rec.ReplyCount > 0 {
			previews, _, err := s.Repo.ListReplies(ctx, rec.ID, 0, rootCommentPreviewLimit)
			if err != nil {
				return nil, err
			}
			for _, preview := range previews {
				comment.Replies = append(comment.Replies, *s.toComment(preview, viewerID))
			}
		}
		comments = append(comments, *comment)
	}
	page := &CommentPage{Comments: comments, HasMore: hasMore}
	if hasMore && len(records) > 0 {
		page.NextCursor = encodeCommentCursor(records[len(records)-1].ID)
	}
	return page, nil
}

func (s *CommentService) ListReplies(ctx context.Context, cardPublicID, rootPublicID string, viewerID int64, cursor string, limit int) (*CommentPage, error) {
	card, err := s.publishedCard(ctx, cardPublicID)
	if err != nil {
		return nil, err
	}
	root, err := s.Repo.GetCommentByPublicID(ctx, rootPublicID)
	if err != nil || root.CardID != card.ID || root.ParentID != 0 {
		return nil, ErrCommentNotFound
	}
	cursorID, err := decodeCommentCursor(cursor)
	if err != nil {
		return nil, err
	}
	records, hasMore, err := s.Repo.ListReplies(ctx, root.ID, cursorID, clampCommentLimit(limit, defaultReplyCommentLimit))
	if err != nil {
		return nil, err
	}
	comments := make([]Comment, 0, len(records))
	for _, rec := range records {
		comments = append(comments, *s.toComment(rec, viewerID))
	}
	page := &CommentPage{Comments: comments, HasMore: hasMore}
	if hasMore && len(records) > 0 {
		page.NextCursor = encodeCommentCursor(records[len(records)-1].ID)
	}
	return page, nil
}

func (s *CommentService) Create(ctx context.Context, cardPublicID string, userID int64, parentPublicID, content string) (*Comment, error) {
	card, err := s.publishedCard(ctx, cardPublicID)
	if err != nil {
		return nil, err
	}
	content, err = normalizeCommentContent(content)
	if err != nil {
		return nil, err
	}
	if s.Limiter != nil && !s.Limiter.Allow(ctx, userID, card.ID) {
		return nil, ErrCommentRateLimit
	}
	parentID, rootID, depth := int64(0), int64(0), 1
	if parentPublicID != "" {
		parent, err := s.Repo.GetCommentByPublicID(ctx, parentPublicID)
		if err != nil || parent.CardID != card.ID || parent.Status != 1 {
			return nil, ErrCommentNotFound
		}
		parentID = parent.ID
		rootID = parent.ID
		if parent.RootID > 0 {
			rootID = parent.RootID
		}
		depth = parent.Depth + 1
		if maxDepth := s.maxDepth(); depth > maxDepth {
			depth = maxDepth
		}
	}
	rec, err := s.Repo.CreateComment(ctx, CommentInput{
		PublicID: s.nextID(), CardID: card.ID, UserID: userID,
		ParentID: parentID, RootID: rootID, Depth: depth, Content: content,
	})
	if err != nil {
		return nil, err
	}
	return s.toComment(rec, userID), nil
}

func (s *CommentService) Update(ctx context.Context, cardPublicID, commentPublicID string, userID int64, content string) (*Comment, error) {
	card, err := s.publishedCard(ctx, cardPublicID)
	if err != nil {
		return nil, err
	}
	content, err = normalizeCommentContent(content)
	if err != nil {
		return nil, err
	}
	rec, err := s.Repo.GetCommentByPublicID(ctx, commentPublicID)
	if err != nil || rec.CardID != card.ID || rec.Status != 1 {
		return nil, ErrCommentNotFound
	}
	if rec.UserID != userID {
		return nil, ErrCommentForbidden
	}
	updated, err := s.Repo.UpdateComment(ctx, card.ID, rec.ID, userID, content, s.now())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrCommentNotFound
		}
		return nil, err
	}
	return s.toComment(updated, userID), nil
}

func (s *CommentService) Delete(ctx context.Context, cardPublicID, commentPublicID string, userID int64) (CommentDeleteResult, error) {
	card, err := s.publishedCard(ctx, cardPublicID)
	if err != nil {
		return CommentDeleteResult{}, err
	}
	rec, err := s.Repo.GetCommentByPublicID(ctx, commentPublicID)
	if err != nil || rec.CardID != card.ID || rec.Status != 1 {
		return CommentDeleteResult{}, ErrCommentNotFound
	}
	if rec.UserID != userID {
		return CommentDeleteResult{}, ErrCommentForbidden
	}
	result, err := s.Repo.DeleteComment(ctx, card.ID, rec.ID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return CommentDeleteResult{}, ErrCommentNotFound
		}
		return CommentDeleteResult{}, err
	}
	return CommentDeleteResult{Tombstoned: result.Tombstoned, CommentCount: result.CommentCount}, nil
}

func (s *CommentService) publishedCard(ctx context.Context, publicID string) (*CardRecord, error) {
	if s.Cards == nil {
		return nil, ErrCommentNotFound
	}
	card, err := s.Cards.GetByPublicID(ctx, publicID)
	if err != nil || card == nil || !card.Published || card.Hidden {
		return nil, ErrCommentNotFound
	}
	return card, nil
}

func (s *CommentService) maxDepth() int {
	if s.MaxDepth > 0 {
		return s.MaxDepth
	}
	return 3
}

func (s *CommentService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *CommentService) nextID() string {
	if s.NewID != nil {
		return s.NewID()
	}
	return uuid.NewString()
}

func (s *CommentService) toComment(rec *CommentRecord, viewerID int64) *Comment {
	status := "normal"
	if rec.Status == 2 {
		status = "deleted"
	}
	owned := rec.Status == 1 && rec.UserID == viewerID
	return &Comment{
		ID: rec.PublicID, CardID: rec.CardPublicID, ParentID: rec.ParentPublicID,
		RootID: rec.RootPublicID, Depth: rec.Depth, Content: rec.Content,
		Author:      CommentAuthor{UserID: rec.UserID, Name: rec.AuthorName, AvatarURL: rec.AuthorAvatarS3Path},
		ReplyToName: rec.ReplyToName, ReplyCount: rec.ReplyCount, Status: status,
		Editable: owned, Deletable: owned,
		EditedAt: rec.EditedAt, CreatedAt: rec.CreatedAt,
	}
}

func normalizeCommentContent(raw string) (string, error) {
	content := strings.TrimSpace(raw)
	if content == "" || utf8.RuneCountInString(content) > 500 {
		return "", ErrCommentInvalid
	}
	for _, r := range content {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return "", ErrCommentInvalid
		}
	}
	return content, nil
}

func clampCommentLimit(limit, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > maxCommentLimit {
		return maxCommentLimit
	}
	return limit
}

func encodeCommentCursor(id int64) string {
	if id <= 0 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10)))
}

func decodeCommentCursor(cursor string) (int64, error) {
	if cursor == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, ErrCommentInvalid
	}
	id, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || id < 0 {
		return 0, ErrCommentInvalid
	}
	return id, nil
}
