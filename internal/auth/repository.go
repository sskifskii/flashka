package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrLoginTaken = errors.New("login taken")
var ErrNotFound = errors.New("not found")

type Repository struct{ Pool *pgxpool.Pool }

func (r Repository) Register(ctx context.Context, u User, normalized, passwordHash string, sessionID uuid.UUID, tokenHash []byte, now, expires time.Time) error {
	tx, e := r.Pool.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, `INSERT INTO users(id,login,login_normalized,password_hash,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$5)`, u.ID, u.Login, normalized, passwordHash, now)
	if unique(e) {
		return ErrLoginTaken
	}
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO folders(user_id,name,name_normalized,is_inbox,created_at,updated_at) VALUES($1,'Inbox','inbox',true,$2,$2)`, u.ID, now)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO auth_sessions(id,token_hash,user_id,expires_at,created_at,last_seen_at) VALUES($1,$2,$3,$4,$5,$5)`, sessionID, tokenHash, u.ID, expires, now)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (r Repository) Credentials(ctx context.Context, normalized string) (User, string, error) {
	var u User
	var hash string
	e := r.Pool.QueryRow(ctx, `SELECT id,login,password_hash FROM users WHERE login_normalized=$1`, normalized).Scan(&u.ID, &u.Login, &hash)
	if errors.Is(e, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	return u, hash, e
}
func (r Repository) CreateSession(ctx context.Context, id, userID uuid.UUID, hash []byte, now, expires time.Time) error {
	_, e := r.Pool.Exec(ctx, `INSERT INTO auth_sessions(id,token_hash,user_id,expires_at,created_at,last_seen_at) VALUES($1,$2,$3,$4,$5,$5)`, id, hash, userID, expires, now)
	return e
}
func (r Repository) CurrentUser(ctx context.Context, hash []byte, now time.Time, lastSeen time.Duration) (User, error) {
	var u User
	e := r.Pool.QueryRow(ctx, `UPDATE auth_sessions s SET last_seen_at=CASE WHEN s.last_seen_at <= $2-make_interval(secs => $3) THEN $2 ELSE s.last_seen_at END FROM users u WHERE s.token_hash=$1 AND s.expires_at>$2 AND u.id=s.user_id RETURNING u.id,u.login`, hash, now, lastSeen.Seconds()).Scan(&u.ID, &u.Login)
	if errors.Is(e, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, e
}
func (r Repository) DeleteSession(ctx context.Context, hash []byte) error {
	_, e := r.Pool.Exec(ctx, `DELETE FROM auth_sessions WHERE token_hash=$1`, hash)
	return e
}
func unique(e error) bool { var pe *pgconn.PgError; return errors.As(e, &pe) && pe.Code == "23505" }
