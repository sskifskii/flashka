package folders

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

func (s Service) Tree(ctx context.Context, userID uuid.UUID) (*Node, error) {
	records, err := s.Repo.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	return BuildTree(records)
}

func (s Service) Create(ctx context.Context, userID, parentID uuid.UUID, name string) (Folder, FieldErrors, error) {
	name, normalized, message := NormalizeName(name)
	if message != "" {
		return Folder{}, FieldErrors{"name": message}, nil
	}
	id, err := s.id()
	if err != nil {
		return Folder{}, nil, err
	}
	folder, err := s.Repo.Create(ctx, id, userID, parentID, name, normalized, s.Clock.Now().UTC())
	return folder, nil, err
}

func (s Service) Update(ctx context.Context, userID, folderID uuid.UUID, name *string, parentID *uuid.UUID) (Folder, FieldErrors, error) {
	if name == nil && parentID == nil {
		return Folder{}, FieldErrors{"request": "Укажите хотя бы одно поле"}, nil
	}
	var normalized *string
	if name != nil {
		clean, value, message := NormalizeName(*name)
		if message != "" {
			return Folder{}, FieldErrors{"name": message}, nil
		}
		name, normalized = &clean, &value
	}
	folder, err := s.Repo.Update(ctx, userID, folderID, name, normalized, parentID, s.Clock.Now().UTC())
	return folder, nil, err
}

func (s Service) Delete(ctx context.Context, userID, folderID uuid.UUID) error {
	return s.Repo.Delete(ctx, userID, folderID)
}
