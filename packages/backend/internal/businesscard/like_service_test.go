package businesscard

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeLikeStore struct {
	state       LikeState
	setErr      error
	setCalls    int
	setCardID   int64
	setUserID   int64
	setLiked    bool
	states      map[int64]LikeState
	statesErr   error
	seeded      map[int64]int64
	seededLiked map[int64]bool
	seededUser  int64
}

func (f *fakeLikeStore) SetLiked(_ context.Context, cardID, userID int64, liked bool) (LikeState, error) {
	f.setCalls++
	f.setCardID, f.setUserID, f.setLiked = cardID, userID, liked
	if f.setErr != nil {
		return LikeState{}, f.setErr
	}
	return f.state, nil
}

func (f *fakeLikeStore) States(context.Context, []int64, int64) (map[int64]LikeState, error) {
	if f.statesErr != nil {
		return nil, f.statesErr
	}
	return f.states, nil
}

func (f *fakeLikeStore) Counts(context.Context, []int64) (map[int64]int64, error) {
	return map[int64]int64{}, nil
}

func (f *fakeLikeStore) SeedCount(_ context.Context, cardID, count int64) error {
	if f.seeded == nil {
		f.seeded = map[int64]int64{}
	}
	f.seeded[cardID] = count
	return nil
}

func (f *fakeLikeStore) SeedState(_ context.Context, cardID, userID int64, state LikeState) error {
	if f.seeded == nil {
		f.seeded = map[int64]int64{}
	}
	if f.seededLiked == nil {
		f.seededLiked = map[int64]bool{}
	}
	f.seeded[cardID] = state.Count
	f.seededLiked[cardID] = state.Liked
	f.seededUser = userID
	return nil
}

type fakeLikeRepo struct {
	counts     map[int64]int64
	liked      map[int64]bool
	countErr   error
	likedErr   error
	countIDs   []int64
	likedCalls int
}

func (f *fakeLikeRepo) Counts(_ context.Context, cardIDs []int64) (map[int64]int64, error) {
	f.countIDs = append([]int64{}, cardIDs...)
	if f.countErr != nil {
		return nil, f.countErr
	}
	return f.counts, nil
}

func (f *fakeLikeRepo) Liked(_ context.Context, cardID, _ int64) (bool, error) {
	f.likedCalls++
	if f.likedErr != nil {
		return false, f.likedErr
	}
	return f.liked[cardID], nil
}

func TestLikeServiceSetResolvesPublishedCard(t *testing.T) {
	store := &fakeLikeStore{state: LikeState{Liked: true, Count: 1}}
	svc := &LikeService{
		Store: store,
		Cards: fakeCardReader{card: &CardRecord{ID: 10, PublicID: "card-1", Published: true}},
	}
	state, err := svc.Set(context.Background(), "card-1", 7, true)
	if err != nil || !state.Liked || state.Count != 1 {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	if store.setCalls != 1 || store.setCardID != 10 || store.setUserID != 7 || !store.setLiked {
		t.Fatalf("store call = %+v", store)
	}
}

func TestLikeServiceSetRejectsUnpublishedHiddenOrMissingCard(t *testing.T) {
	cases := []struct {
		name string
		card *CardRecord
		err  error
	}{
		{name: "unpublished", card: &CardRecord{ID: 10, Published: false}},
		{name: "hidden", card: &CardRecord{ID: 10, Published: true, Hidden: true}},
		{name: "missing", err: ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeLikeStore{state: LikeState{Liked: true, Count: 1}}
			svc := &LikeService{Store: store, Cards: fakeCardReader{card: tc.card, err: tc.err}}
			if _, err := svc.Set(context.Background(), "card-1", 7, true); !errors.Is(err, ErrNotFound) {
				t.Fatalf("err = %v, want ErrNotFound", err)
			}
			if store.setCalls != 0 {
				t.Fatalf("store should not be called, calls=%d", store.setCalls)
			}
		})
	}
}

func TestLikeServiceSetRejectsNilCard(t *testing.T) {
	store := &fakeLikeStore{state: LikeState{Liked: true, Count: 1}}
	svc := &LikeService{Store: store, Cards: nilCardReader{}}
	if _, err := svc.Set(context.Background(), "card-1", 7, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if store.setCalls != 0 {
		t.Fatalf("store should not be called, calls=%d", store.setCalls)
	}
}

type nilCardReader struct{}

func (nilCardReader) GetByPublicID(context.Context, string) (*CardRecord, error) {
	return nil, nil
}

func TestLikeServiceSetPropagatesCardLookupError(t *testing.T) {
	lookupErr := errors.New("mysql unavailable")
	store := &fakeLikeStore{state: LikeState{Liked: true, Count: 1}}
	svc := &LikeService{Store: store, Cards: fakeCardReader{err: lookupErr}}
	_, err := svc.Set(context.Background(), "card-1", 7, true)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("err = %v, want lookup error", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("lookup error collapsed to ErrNotFound")
	}
	if store.setCalls != 0 {
		t.Fatalf("store should not be called, calls=%d", store.setCalls)
	}
}

func TestLikeServiceStatesFallsBackToMySQL(t *testing.T) {
	store := &fakeLikeStore{states: map[int64]LikeState{
		10: {Found: false, Count: 0},
		11: {Found: true, Count: 5, Liked: false},
	}}
	repo := &fakeLikeRepo{
		counts: map[int64]int64{10: 3},
		liked:  map[int64]bool{10: true},
	}
	svc := &LikeService{Store: store, Repo: repo}
	cards := []Card{{InternalID: 10}, {InternalID: 11}}
	if err := svc.States(context.Background(), cards, 7); err != nil {
		t.Fatalf("err = %v", err)
	}
	if cards[0].LikeCount != 3 || !cards[0].Liked {
		t.Fatalf("card[0] = %+v", cards[0])
	}
	if cards[1].LikeCount != 5 || cards[1].Liked {
		t.Fatalf("card[1] = %+v", cards[1])
	}
	if len(repo.countIDs) != 1 || repo.countIDs[0] != 10 || repo.likedCalls != 1 {
		t.Fatalf("repo calls counts=%v liked=%d", repo.countIDs, repo.likedCalls)
	}
	if store.seeded[10] != 3 {
		t.Fatalf("redis count not seeded: %v", store.seeded)
	}
	if !store.seededLiked[10] || store.seededUser != 7 {
		t.Fatalf("redis membership not seeded: liked=%v user=%d", store.seededLiked, store.seededUser)
	}
}

func TestLikeServiceStatesSeedsMembershipForRepeatRead(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewRedisLikeStore(client, "vibebiz", 1_000_000, 1)
	repo := &fakeLikeRepo{
		counts: map[int64]int64{10: 1},
		liked:  map[int64]bool{10: true},
	}
	svc := &LikeService{Store: store, Repo: repo}
	ctx := context.Background()

	first := []Card{{InternalID: 10}}
	if err := svc.States(ctx, first, 7); err != nil {
		t.Fatalf("first read: %v", err)
	}
	if first[0].LikeCount != 1 || !first[0].Liked {
		t.Fatalf("first card = %+v", first[0])
	}

	second := []Card{{InternalID: 10}}
	if err := svc.States(ctx, second, 7); err != nil {
		t.Fatalf("second read: %v", err)
	}
	if second[0].LikeCount != 1 || !second[0].Liked {
		t.Fatalf("second card = %+v", second[0])
	}
	if len(repo.countIDs) != 1 {
		t.Fatalf("second read should be served by redis, repo calls=%v", repo.countIDs)
	}
}

func TestLikeServiceStatesFallsBackWhenRedisErrors(t *testing.T) {
	store := &fakeLikeStore{statesErr: errors.New("redis down")}
	repo := &fakeLikeRepo{
		counts: map[int64]int64{10: 4},
		liked:  map[int64]bool{10: true},
	}
	svc := &LikeService{Store: store, Repo: repo}
	cards := []Card{{InternalID: 10}}
	if err := svc.States(context.Background(), cards, 7); err != nil {
		t.Fatalf("err = %v", err)
	}
	if cards[0].LikeCount != 4 || !cards[0].Liked {
		t.Fatalf("card = %+v", cards[0])
	}
}

func TestLikeServiceStatesReturnsRedisErrorWithoutRepo(t *testing.T) {
	store := &fakeLikeStore{statesErr: errors.New("redis down")}
	svc := &LikeService{Store: store}
	if err := svc.States(context.Background(), []Card{{InternalID: 10}}, 7); err == nil {
		t.Fatal("expected redis error")
	}
}

func TestLikeServiceStatesReturnsRepositoryErrors(t *testing.T) {
	store := &fakeLikeStore{states: map[int64]LikeState{10: {Found: false}}}
	svc := &LikeService{Store: store, Repo: &fakeLikeRepo{countErr: errors.New("mysql down")}}
	if err := svc.States(context.Background(), []Card{{InternalID: 10}}, 7); err == nil {
		t.Fatal("expected count error")
	}

	svc = &LikeService{Store: store, Repo: &fakeLikeRepo{
		counts:   map[int64]int64{10: 1},
		likedErr: errors.New("mysql down"),
	}}
	if err := svc.States(context.Background(), []Card{{InternalID: 10}}, 7); err == nil {
		t.Fatal("expected liked error")
	}
}

func TestLikeServiceStatesHandlesMissingMapFromStore(t *testing.T) {
	store := &fakeLikeStore{}
	repo := &fakeLikeRepo{counts: map[int64]int64{10: 2}, liked: map[int64]bool{10: true}}
	svc := &LikeService{Store: store, Repo: repo}
	cards := []Card{{InternalID: 10}}
	if err := svc.States(context.Background(), cards, 7); err != nil {
		t.Fatalf("err = %v", err)
	}
	if cards[0].LikeCount != 2 || !cards[0].Liked {
		t.Fatalf("card = %+v", cards[0])
	}
}

func TestLikeServiceStatesWithoutRepoUsesRedisOnly(t *testing.T) {
	store := &fakeLikeStore{states: map[int64]LikeState{10: {Found: true, Count: 9, Liked: true}}}
	svc := &LikeService{Store: store}
	cards := []Card{{InternalID: 10}}
	if err := svc.States(context.Background(), cards, 7); err != nil {
		t.Fatalf("err = %v", err)
	}
	if cards[0].LikeCount != 9 || !cards[0].Liked {
		t.Fatalf("card = %+v", cards[0])
	}
}
