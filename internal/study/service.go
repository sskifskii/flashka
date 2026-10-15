package study

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
)

type Clock interface{ Now() time.Time }

type Service struct {
	Repo    Repository
	Clock   Clock
	NewID   func() (uuid.UUID, error)
	NewSeed func() (int64, error)
}

func (s Service) id() (uuid.UUID, error) {
	if s.NewID != nil {
		return s.NewID()
	}
	return uuid.NewRandom()
}

func (s Service) seed() (int64, error) {
	if s.NewSeed != nil {
		return s.NewSeed()
	}
	var b [8]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(b[:])), nil
}

func (s Service) Preview(ctx context.Context, userID uuid.UUID, scopeType string, folderID *uuid.UUID) (Preview, FieldErrors, error) {
	scope, fields := ValidateScope(scopeType, folderID)
	if fields != nil {
		return Preview{}, fields, nil
	}
	p, err := s.Repo.Preview(ctx, userID, scope, s.Clock.Now().UTC())
	return p, nil, err
}

func (s Service) Create(ctx context.Context, userID uuid.UUID, scopeType string, folderID *uuid.UUID, count int) (Session, FieldErrors, error) {
	scope, fields := ValidateScope(scopeType, folderID)
	if count < 1 || count > 100 {
		if fields == nil {
			fields = FieldErrors{}
		}
		fields["count"] = "Введите число от 1 до 100"
	}
	if fields != nil {
		return Session{}, fields, nil
	}
	id, err := s.id()
	if err != nil {
		return Session{}, nil, err
	}
	seed, err := s.seed()
	if err != nil {
		return Session{}, nil, err
	}
	session, err := s.Repo.Create(ctx, id, userID, scope, count, s.Clock.Now().UTC(), seed)
	return session, nil, err
}
