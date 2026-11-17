package businesscard

import (
	"context"
	"errors"
)

type CardLikeReader interface {
	GetByPublicID(ctx context.Context, publicID string) (*CardRecord, error)
}

type LikeService struct {
	Store LikeStore
	Cards CardLikeReader
	Repo  LikeReadRepository
}

func (s *LikeService) Set(ctx context.Context, cardPublicID string, userID int64, liked bool) (LikeState, error) {
	card, err := s.Cards.GetByPublicID(ctx, cardPublicID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return LikeState{}, ErrNotFound
		}
		return LikeState{}, err
	}
	if card == nil || !card.Published || card.Hidden {
		return LikeState{}, ErrNotFound
	}
	return s.Store.SetLiked(ctx, card.ID, userID, liked)
}

func (s *LikeService) States(ctx context.Context, cards []Card, userID int64) error {
	ids := make([]int64, 0, len(cards))
	for _, card := range cards {
		ids = append(ids, card.InternalID)
	}
	states, err := s.Store.States(ctx, ids, userID)
	if err != nil && s.Repo == nil {
		return err
	}
	if err != nil || states == nil {
		states = map[int64]LikeState{}
	}
	missing := make([]int64, 0)
	for _, id := range ids {
		if state, ok := states[id]; !ok || !state.Found {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 && s.Repo != nil {
		counts, countErr := s.Repo.Counts(ctx, missing)
		if countErr != nil {
			return countErr
		}
		for _, id := range missing {
			liked, likedErr := s.Repo.Liked(ctx, id, userID)
			if likedErr != nil {
				return likedErr
			}
			state := LikeState{Count: counts[id], Liked: liked, Found: true}
			states[id] = state
			_ = s.Store.SeedState(ctx, id, userID, state)
		}
	}
	for i := range cards {
		state := states[cards[i].InternalID]
		cards[i].LikeCount = state.Count
		cards[i].Liked = state.Liked
	}
	return nil
}
