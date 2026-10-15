package cards

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Clock interface{ Now() time.Time }

type Service struct {
	Repo  Repository
	Clock Clock
	NewID func() (uuid.UUID, error)
}

func (s Service) id() (uuid.UUID, error) {
	if s.NewID != nil {
		return s.NewID()
	}
	return uuid.NewRandom()
}

func validate(front, back string) (string, string, FieldErrors) {
	front, fe := NormalizeSide(front)
	back, be := NormalizeSide(back)
	fields := FieldErrors{}
	if fe != "" {
		fields["front"] = fe
	}
	if be != "" {
		fields["back"] = be
	}
	if len(fields) > 0 {
		return front, back, fields
	}
	return front, back, nil
}

func (s Service) Create(ctx context.Context, userID uuid.UUID, front, back string) (Card, FieldErrors, error) {
	front, back, fields := validate(front, back)
	if fields != nil {
		return Card{}, fields, nil
	}
	id, err := s.id()
	if err != nil {
		return Card{}, nil, err
	}
	c, err := s.Repo.Create(ctx, id, userID, front, back, s.Clock.Now().UTC())
	return c, nil, err
}
func (s Service) Get(ctx context.Context, userID, cardID uuid.UUID) (Card, error) {
	return s.Repo.Get(ctx, userID, cardID)
}
func (s Service) Update(ctx context.Context, userID, cardID uuid.UUID, front, back *string) (Card, FieldErrors, error) {
	if front == nil && back == nil {
		return Card{}, FieldErrors{"request": "Укажите хотя бы одно поле"}, nil
	}
	current, err := s.Repo.Get(ctx, userID, cardID)
	if err != nil {
		return Card{}, nil, err
	}
	f, b := current.Front, current.Back
	if front != nil {
		f = *front
	}
	if back != nil {
		b = *back
	}
	f, b, fields := validate(f, b)
	if fields != nil {
		return Card{}, fields, nil
	}
	c, err := s.Repo.Update(ctx, userID, cardID, f, b, s.Clock.Now().UTC())
	return c, nil, err
}
func (s Service) Delete(ctx context.Context, userID, cardID uuid.UUID) error {
	return s.Repo.Delete(ctx, userID, cardID)
}
func (s Service) List(ctx context.Context, userID uuid.UUID, cursor string, limit int) ([]Summary, string, error) {
	c, err := DecodeCursor(cursor, userID)
	if err != nil {
		return nil, "", err
	}
	items, err := s.Repo.List(ctx, userID, c, limit+1)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		last := items[limit-1]
		items = items[:limit]
		next, err = EncodeCursor(Cursor{CreatedAt: last.CreatedAt, ID: last.ID, UserID: userID})
	}
	return items, next, err
}
