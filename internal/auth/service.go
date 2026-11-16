package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Clock interface{ Now() time.Time }
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

type Hasher interface {
	Hash(string) (string, error)
	Verify(string, string) (bool, error)
}

type Service struct {
	Repo      Repository
	Hasher    Hasher
	Tokens    TokenSource
	Clock     Clock
	TTL       time.Duration
	LastSeen  time.Duration
	DummyHash string
	NewID     func() (uuid.UUID, error)
}

func (s Service) id() (uuid.UUID, error) {
	if s.NewID != nil {
		return s.NewID()
	}
	return uuid.NewRandom()
}
func (s Service) Register(ctx context.Context, login, password string) (User, string, FieldErrors, error) {
	if fe := ValidateCredentials(login, password); fe != nil {
		return User{}, "", fe, nil
	}
	ph, e := s.Hasher.Hash(password)
	if e != nil {
		return User{}, "", nil, e
	}
	tok, th, e := s.Tokens.New()
	if e != nil {
		return User{}, "", nil, e
	}
	now := s.Clock.Now().UTC()
	userID, e := s.id()
	if e != nil {
		return User{}, "", nil, e
	}
	sessionID, e := s.id()
	if e != nil {
		return User{}, "", nil, e
	}
	u := User{ID: userID, Login: login}
	e = s.Repo.Register(ctx, u, NormalizeLogin(login), ph, sessionID, th, now, now.Add(s.TTL))
	return u, tok, nil, e
}
func (s Service) Login(ctx context.Context, login, password string) (User, string, FieldErrors, error) {
	if fe := ValidateCredentials(login, password); fe != nil {
		return User{}, "", fe, nil
	}
	u, ph, e := s.Repo.Credentials(ctx, NormalizeLogin(login))
	if errors.Is(e, ErrNotFound) {
		ph = s.DummyHash
		u = User{}
	} else if e != nil {
		return User{}, "", nil, e
	}
	ok, e := s.Hasher.Verify(password, ph)
	if e != nil {
		return User{}, "", nil, e
	}
	if !ok || u.ID == uuid.Nil {
		return User{}, "", nil, ErrNotFound
	}
	tok, th, e := s.Tokens.New()
	if e != nil {
		return User{}, "", nil, e
	}
	now := s.Clock.Now().UTC()
	sessionID, e := s.id()
	if e != nil {
		return User{}, "", nil, e
	}
	if e = s.Repo.CreateSession(ctx, sessionID, u.ID, th, now, now.Add(s.TTL)); e != nil {
		return User{}, "", nil, e
	}
	return u, tok, nil, nil
}
func (s Service) CurrentUser(ctx context.Context, token string) (User, error) {
	hash, e := HashToken(token)
	if e != nil {
		return User{}, ErrNotFound
	}
	return s.Repo.CurrentUser(ctx, hash, s.Clock.Now().UTC(), s.LastSeen)
}
func (s Service) Logout(ctx context.Context, token string) error {
	hash, e := HashToken(token)
	if e != nil {
		return nil
	}
	return s.Repo.DeleteSession(ctx, hash)
}
