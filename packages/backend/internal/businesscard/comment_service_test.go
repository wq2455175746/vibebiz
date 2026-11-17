package businesscard

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

var testCommentNow = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

type fakeCardReader struct {
	card *CardRecord
	err  error
}

func (f fakeCardReader) GetByPublicID(context.Context, string) (*CardRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.card == nil {
		return &CardRecord{ID: 1, PublicID: "card-1", Published: true}, nil
	}
	return f.card, nil
}

type fakeCommentLimiter struct {
	allow bool
	calls int
}

func (f *fakeCommentLimiter) Allow(context.Context, int64, int64) bool {
	f.calls++
	return f.allow
}

type fakeCommentRepo struct {
	byPublicID      map[string]*CommentRecord
	lastInput       CommentInput
	updateAllowed   bool
	updateCalls     int
	deleteCalls     int
	replyCalls      int
	lastRootsLimit  int
	lastRootsCursor int64
	lastReplyLimit  int
	lastReplyCursor int64
	rootsRecords    []*CommentRecord
	rootsHasMore    bool
	repliesRecords  map[int64][]*CommentRecord
	repliesHasMore  bool
	deleteResult    CommentDeleteResult
	createErr       error
}

func newFakeCommentRepo() *fakeCommentRepo {
	return &fakeCommentRepo{
		byPublicID: map[string]*CommentRecord{
			"c-1": {
				ID: 1, PublicID: "c-1", CardID: 1, CardPublicID: "card-1",
				UserID: 8, Depth: 1, Status: 1, Content: "old", AuthorName: "bob",
				CreatedAt: testCommentNow.Add(-time.Hour),
			},
		},
		updateAllowed:  true,
		repliesRecords: map[int64][]*CommentRecord{},
	}
}

func (f *fakeCommentRepo) ListRoots(_ context.Context, _ int64, cursor int64, limit int) ([]*CommentRecord, bool, error) {
	f.lastRootsCursor = cursor
	f.lastRootsLimit = limit
	return f.rootsRecords, f.rootsHasMore, nil
}

func (f *fakeCommentRepo) ListReplies(_ context.Context, rootID, cursor int64, limit int) ([]*CommentRecord, bool, error) {
	f.replyCalls++
	f.lastReplyCursor = cursor
	f.lastReplyLimit = limit
	records := f.repliesRecords[rootID]
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}
	return records, f.repliesHasMore, nil
}

func (f *fakeCommentRepo) GetCommentByPublicID(_ context.Context, publicID string) (*CommentRecord, error) {
	rec, ok := f.byPublicID[publicID]
	if !ok {
		return nil, ErrNotFound
	}
	return rec, nil
}

func (f *fakeCommentRepo) CreateComment(_ context.Context, input CommentInput) (*CommentRecord, error) {
	f.lastInput = input
	if f.createErr != nil {
		return nil, f.createErr
	}
	rec := &CommentRecord{
		ID: 100, PublicID: input.PublicID, CardID: input.CardID, CardPublicID: "card-1",
		UserID: input.UserID, ParentID: input.ParentID, RootID: input.RootID,
		Depth: input.Depth, Content: input.Content, Status: 1, AuthorName: "alice",
		CreatedAt: testCommentNow,
	}
	if parent, ok := f.byPublicIDByID(input.ParentID); ok {
		rec.ParentPublicID = parent.PublicID
		rec.ReplyToName = parent.AuthorName
	}
	if root, ok := f.byPublicIDByID(input.RootID); ok {
		rec.RootPublicID = root.PublicID
	}
	f.byPublicID[input.PublicID] = rec
	return rec, nil
}

func (f *fakeCommentRepo) byPublicIDByID(id int64) (*CommentRecord, bool) {
	for _, rec := range f.byPublicID {
		if rec.ID == id {
			return rec, true
		}
	}
	return nil, false
}

func (f *fakeCommentRepo) UpdateComment(_ context.Context, _, commentID, userID int64, content string, at time.Time) (*CommentRecord, error) {
	f.updateCalls++
	if !f.updateAllowed {
		return nil, ErrNotFound
	}
	var rec *CommentRecord
	for _, candidate := range f.byPublicID {
		if candidate.ID == commentID {
			rec = candidate
			break
		}
	}
	if rec == nil || rec.UserID != userID || rec.Status != 1 {
		return nil, ErrNotFound
	}
	rec.Content = content
	rec.EditedAt = &at
	return rec, nil
}

func (f *fakeCommentRepo) DeleteComment(_ context.Context, _, commentID, userID int64) (CommentDeleteResult, error) {
	f.deleteCalls++
	var rec *CommentRecord
	for _, candidate := range f.byPublicID {
		if candidate.ID == commentID {
			rec = candidate
			break
		}
	}
	if rec == nil || rec.UserID != userID || rec.Status != 1 {
		return CommentDeleteResult{}, ErrNotFound
	}
	return f.deleteResult, nil
}

func newTestCommentService(repo CommentRepository) *CommentService {
	return &CommentService{
		Repo:     repo,
		Cards:    fakeCardReader{},
		Now:      func() time.Time { return testCommentNow },
		MaxDepth: 3,
		NewID:    func() string { return "new-comment" },
	}
}

func TestCreateCommentCapsDepthAtThree(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.byPublicID["parent"] = &CommentRecord{
		ID: 9, PublicID: "parent", CardID: 1, UserID: 8, RootID: 0, Depth: 3, Status: 1,
	}
	svc := newTestCommentService(repo)
	comment, err := svc.Create(context.Background(), "card-1", 7, "parent", "回复")
	if err != nil {
		t.Fatal(err)
	}
	if comment.Depth != 3 || repo.lastInput.ParentID != 9 {
		t.Fatalf("depth=%d parent=%d", comment.Depth, repo.lastInput.ParentID)
	}
	if repo.lastInput.RootID != 9 {
		t.Fatalf("root = %d, want 9", repo.lastInput.RootID)
	}
}

func TestCommentCreateRejectsTombstonedParent(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.byPublicID["parent"] = &CommentRecord{
		ID: 9, PublicID: "parent", CardID: 1, UserID: 8, Depth: 2, Status: 2,
	}
	svc := newTestCommentService(repo)
	if _, err := svc.Create(context.Background(), "card-1", 7, "parent", "回复"); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("err = %v, want ErrCommentNotFound", err)
	}
}

func TestCreateCommentNormalizesContent(t *testing.T) {
	repo := newFakeCommentRepo()
	svc := newTestCommentService(repo)
	comment, err := svc.Create(context.Background(), "card-1", 7, "", "  hi\n\tthere  ")
	if err != nil {
		t.Fatal(err)
	}
	if comment.Content != "hi\n\tthere" {
		t.Fatalf("content = %q", comment.Content)
	}
	if repo.lastInput.Depth != 1 || repo.lastInput.ParentID != 0 || repo.lastInput.RootID != 0 {
		t.Fatalf("input = %+v", repo.lastInput)
	}
}

func TestCreateCommentRejectsInvalidContent(t *testing.T) {
	cases := map[string]string{
		"empty":     "   ",
		"too long":  strings.Repeat("字", 501),
		"control":   "hi\x00there",
		"bell char": "hi\athere",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			repo := newFakeCommentRepo()
			svc := newTestCommentService(repo)
			if _, err := svc.Create(context.Background(), "card-1", 7, "", content); !errors.Is(err, ErrCommentInvalid) {
				t.Fatalf("err = %v, want ErrCommentInvalid", err)
			}
		})
	}
}

func TestCreateCommentRateLimited(t *testing.T) {
	repo := newFakeCommentRepo()
	svc := newTestCommentService(repo)
	limiter := &fakeCommentLimiter{allow: false}
	svc.Limiter = limiter
	if _, err := svc.Create(context.Background(), "card-1", 7, "", "hi"); !errors.Is(err, ErrCommentRateLimit) {
		t.Fatalf("err = %v, want ErrCommentRateLimit", err)
	}
	if limiter.calls != 1 {
		t.Fatalf("limiter calls = %d", limiter.calls)
	}
}

func TestCreateCommentRejectsUnpublishedCard(t *testing.T) {
	repo := newFakeCommentRepo()
	svc := newTestCommentService(repo)
	svc.Cards = fakeCardReader{card: &CardRecord{ID: 1, PublicID: "card-1", Published: false}}
	if _, err := svc.Create(context.Background(), "card-1", 7, "", "hi"); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("err = %v, want ErrCommentNotFound", err)
	}
}

func TestListRootsIncludesReplyPreviewAndCursor(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.rootsRecords = []*CommentRecord{
		{ID: 12, PublicID: "c-12", CardID: 1, CardPublicID: "card-1", UserID: 8, Depth: 1, Status: 1, Content: "hello", ReplyCount: 1, AuthorName: "alice", CreatedAt: testCommentNow},
		{ID: 11, PublicID: "c-11", CardID: 1, CardPublicID: "card-1", UserID: 9, Depth: 1, Status: 1, Content: "world", AuthorName: "bob", CreatedAt: testCommentNow},
	}
	repo.rootsHasMore = true
	repo.repliesRecords[12] = []*CommentRecord{
		{ID: 20, PublicID: "c-20", CardID: 1, CardPublicID: "card-1", UserID: 9, ParentID: 12, RootID: 12, Depth: 2, Status: 1, Content: "first", AuthorName: "bob", CreatedAt: testCommentNow},
	}
	svc := newTestCommentService(repo)
	page, err := svc.ListRoots(context.Background(), "card-1", 7, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasMore || len(page.Comments) != 2 {
		t.Fatalf("page = %+v", page)
	}
	if len(page.Comments[0].Replies) != 1 || page.Comments[0].Replies[0].ID != "c-20" {
		t.Fatalf("replies = %+v", page.Comments[0].Replies)
	}
	if repo.lastRootsLimit != 20 || repo.lastReplyLimit != 2 {
		t.Fatalf("limits roots=%d replies=%d", repo.lastRootsLimit, repo.lastReplyLimit)
	}
	id, err := decodeCommentCursor(page.NextCursor)
	if err != nil || id != 11 {
		t.Fatalf("next cursor = %q id=%d err=%v", page.NextCursor, id, err)
	}
}

func TestListRootsUsesCursor(t *testing.T) {
	repo := newFakeCommentRepo()
	svc := newTestCommentService(repo)
	if _, err := svc.ListRoots(context.Background(), "card-1", 7, encodeCommentCursor(12), 5); err != nil {
		t.Fatal(err)
	}
	if repo.lastRootsCursor != 12 || repo.lastRootsLimit != 5 {
		t.Fatalf("cursor=%d limit=%d", repo.lastRootsCursor, repo.lastRootsLimit)
	}
}

func TestListRootsAndRepliesAllowFiftyLimit(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.byPublicID["root"] = &CommentRecord{ID: 12, PublicID: "root", CardID: 1, Depth: 1, Status: 1}
	svc := newTestCommentService(repo)

	if _, err := svc.ListRoots(context.Background(), "card-1", 7, "", 50); err != nil {
		t.Fatal(err)
	}
	if repo.lastRootsLimit != 50 {
		t.Fatalf("roots limit = %d, want 50", repo.lastRootsLimit)
	}

	if _, err := svc.ListReplies(context.Background(), "card-1", "root", 7, "", 50); err != nil {
		t.Fatal(err)
	}
	if repo.lastReplyLimit != 50 {
		t.Fatalf("replies limit = %d, want 50", repo.lastReplyLimit)
	}
}

func TestListRootsRejectsInvalidCursor(t *testing.T) {
	svc := newTestCommentService(newFakeCommentRepo())
	if _, err := svc.ListRoots(context.Background(), "card-1", 7, "not-a-cursor", 0); !errors.Is(err, ErrCommentInvalid) {
		t.Fatalf("err = %v, want ErrCommentInvalid", err)
	}
}

func TestListRepliesRejectsRootFromOtherCard(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.byPublicID["root"] = &CommentRecord{ID: 12, PublicID: "root", CardID: 2, Depth: 1, Status: 1}
	svc := newTestCommentService(repo)
	if _, err := svc.ListReplies(context.Background(), "card-1", "root", 7, "", 0); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("err = %v, want ErrCommentNotFound", err)
	}
}

func TestListRepliesAllowsTombstonedRootAndCapsLimit(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.byPublicID["root"] = &CommentRecord{ID: 12, PublicID: "root", CardID: 1, Depth: 1, Status: 2}
	repo.repliesRecords[12] = []*CommentRecord{
		{ID: 20, PublicID: "c-20", CardID: 1, CardPublicID: "card-1", UserID: 8, ParentID: 12, RootID: 12, Depth: 2, Status: 1, Content: "hi", AuthorName: "alice", CreatedAt: testCommentNow},
	}
	svc := newTestCommentService(repo)
	page, err := svc.ListReplies(context.Background(), "card-1", "root", 7, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Comments) != 1 || repo.lastReplyLimit != 10 {
		t.Fatalf("page=%+v limit=%d", page, repo.lastReplyLimit)
	}
}

func TestListRepliesCursor(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.byPublicID["root"] = &CommentRecord{ID: 12, PublicID: "root", CardID: 1, Depth: 1, Status: 1}
	repo.repliesHasMore = true
	repo.repliesRecords[12] = []*CommentRecord{
		{ID: 21, PublicID: "c-21", CardID: 1, CardPublicID: "card-1", UserID: 8, ParentID: 12, RootID: 12, Depth: 2, Status: 1, Content: "hi", AuthorName: "alice", CreatedAt: testCommentNow},
	}
	svc := newTestCommentService(repo)
	page, err := svc.ListReplies(context.Background(), "card-1", "root", 7, encodeCommentCursor(20), 10)
	if err != nil {
		t.Fatal(err)
	}
	if repo.lastReplyCursor != 20 || !page.HasMore {
		t.Fatalf("cursor=%d hasMore=%v", repo.lastReplyCursor, page.HasMore)
	}
	id, err := decodeCommentCursor(page.NextCursor)
	if err != nil || id != 21 {
		t.Fatalf("next cursor=%q id=%d err=%v", page.NextCursor, id, err)
	}
}

func TestUpdateRejectsOtherAuthor(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.updateAllowed = false
	svc := newTestCommentService(repo)
	_, err := svc.Update(context.Background(), "card-1", "c-1", 7, "改")
	if !errors.Is(err, ErrCommentForbidden) {
		t.Fatalf("err = %v", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("repo update called %d times", repo.updateCalls)
	}
}

func TestUpdateNormalizesContent(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.byPublicID["own"] = &CommentRecord{ID: 2, PublicID: "own", CardID: 1, UserID: 7, Depth: 1, Status: 1, Content: "old"}
	svc := newTestCommentService(repo)
	comment, err := svc.Update(context.Background(), "card-1", "own", 7, "  new  ")
	if err != nil {
		t.Fatal(err)
	}
	if comment.Content != "new" {
		t.Fatalf("content = %q", comment.Content)
	}
	if comment.EditedAt == nil || !comment.EditedAt.Equal(testCommentNow) {
		t.Fatalf("editedAt = %v", comment.EditedAt)
	}
}

func TestUpdateRejectsInvalidContentBeforeRepo(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.byPublicID["own"] = &CommentRecord{ID: 2, PublicID: "own", CardID: 1, UserID: 7, Depth: 1, Status: 1}
	svc := newTestCommentService(repo)
	if _, err := svc.Update(context.Background(), "card-1", "own", 7, "   "); !errors.Is(err, ErrCommentInvalid) {
		t.Fatalf("err = %v, want ErrCommentInvalid", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("repo update called %d times", repo.updateCalls)
	}
}

func TestUpdateRejectsTombstone(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.byPublicID["gone"] = &CommentRecord{ID: 2, PublicID: "gone", CardID: 1, UserID: 7, Depth: 1, Status: 2}
	svc := newTestCommentService(repo)
	if _, err := svc.Update(context.Background(), "card-1", "gone", 7, "new"); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("err = %v, want ErrCommentNotFound", err)
	}
}

func TestDeleteForbiddenBeforeRepo(t *testing.T) {
	repo := newFakeCommentRepo()
	svc := newTestCommentService(repo)
	if _, err := svc.Delete(context.Background(), "card-1", "c-1", 7); !errors.Is(err, ErrCommentForbidden) {
		t.Fatalf("err = %v, want ErrCommentForbidden", err)
	}
	if repo.deleteCalls != 0 {
		t.Fatalf("repo delete called %d times", repo.deleteCalls)
	}
}

func TestDeleteMapsResult(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.byPublicID["own"] = &CommentRecord{ID: 2, PublicID: "own", CardID: 1, UserID: 7, Depth: 1, Status: 1}
	repo.deleteResult = CommentDeleteResult{Tombstoned: true, CommentCount: 4}
	svc := newTestCommentService(repo)
	result, err := svc.Delete(context.Background(), "card-1", "own", 7)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Tombstoned || result.CommentCount != 4 {
		t.Fatalf("result = %+v", result)
	}
}

func TestDeleteRejectsMissingComment(t *testing.T) {
	svc := newTestCommentService(newFakeCommentRepo())
	if _, err := svc.Delete(context.Background(), "card-1", "missing", 7); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("err = %v, want ErrCommentNotFound", err)
	}
}

func newTestRedisLimiter(t *testing.T) (*RedisCommentRateLimiter, *redis.Client, *time.Time) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	now := testCommentNow
	limiter := &RedisCommentRateLimiter{
		Client: client,
		Prefix: "vibebiz",
		Now:    func() time.Time { return now },
	}
	return limiter, client, &now
}

func TestRedisCommentRateLimiterShortWindow(t *testing.T) {
	limiter, _, _ := newTestRedisLimiter(t)
	for i := 0; i < 5; i++ {
		if !limiter.Allow(context.Background(), 7, 1) {
			t.Fatalf("request %d rejected", i+1)
		}
	}
	if limiter.Allow(context.Background(), 7, 1) {
		t.Fatal("sixth request in window should be rejected")
	}
	if !limiter.Allow(context.Background(), 8, 1) {
		t.Fatal("other user should not be limited")
	}
	if !limiter.Allow(context.Background(), 7, 2) {
		t.Fatal("other card should not be limited")
	}
}

func TestRedisCommentRateLimiterFailsOpenOnRedisError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = client.Close() })
	mr.Close()

	var buf bytes.Buffer
	limiter := &RedisCommentRateLimiter{
		Client: client,
		Prefix: "vibebiz",
		Now:    func() time.Time { return testCommentNow },
		Logger: slog.New(slog.NewTextHandler(&buf, nil)),
	}
	if !limiter.Allow(context.Background(), 7, 1) {
		t.Fatal("Allow() = false, want true on redis error")
	}
	if !strings.Contains(buf.String(), "comment rate limiter failed") {
		t.Fatalf("warning log missing: %s", buf.String())
	}
}

func TestRedisCommentRateLimiterHourWindow(t *testing.T) {
	limiter, _, now := newTestRedisLimiter(t)
	for i := 0; i < 30; i++ {
		*now = testCommentNow.Add(time.Duration(i) * 10 * time.Second)
		if !limiter.Allow(context.Background(), 7, 1) {
			t.Fatalf("request %d rejected", i+1)
		}
	}
	*now = testCommentNow.Add(30 * 10 * time.Second)
	if limiter.Allow(context.Background(), 7, 1) {
		t.Fatal("31st request in hour should be rejected")
	}
}
