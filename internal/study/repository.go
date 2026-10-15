package study

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ Pool *pgxpool.Pool }

func validateFolder(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID uuid.UUID, scope Scope) error {
	if scope.Type != "folder" {
		return nil
	}
	var exists bool
	err := q.QueryRow(ctx, `SELECT true FROM folders WHERE user_id=$1 AND id=$2 FOR SHARE`, userID, *scope.FolderID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrFolderNotFound
	}
	return err
}

const scopedCards = `WITH RECURSIVE scoped_folders AS (
	SELECT id FROM folders WHERE user_id=$1 AND ($2::uuid IS NULL OR id=$2)
	UNION
	SELECT f.id FROM folders f JOIN scoped_folders p ON f.parent_id=p.id WHERE f.user_id=$1
)`

func (r Repository) Preview(ctx context.Context, userID uuid.UUID, scope Scope, now time.Time) (Preview, error) {
	if err := validateFolder(ctx, r.Pool, userID, scope); err != nil {
		return Preview{}, err
	}
	var p Preview
	err := r.Pool.QueryRow(ctx, scopedCards+`
	SELECT count(*)::int,
		count(*) FILTER (WHERE c.due_at <= $3)::int,
		count(*) FILTER (WHERE c.due_at IS NULL)::int,
		count(*) FILTER (WHERE c.interval_days >= 30)::int
	FROM cards c JOIN scoped_folders f ON f.id=c.folder_id WHERE c.user_id=$1`, userID, scope.FolderID, now).Scan(&p.Available, &p.Due, &p.New, &p.Mastered)
	return p, err
}

func (r Repository) Create(ctx context.Context, sessionID, userID uuid.UUID, scope Scope, count int, now time.Time, seed int64) (Session, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	if err = validateFolder(ctx, tx, userID, scope); err != nil {
		return Session{}, err
	}
	rows, err := tx.Query(ctx, scopedCards+`, candidates AS (
		SELECT c.id,c.front,c.due_at,
			CASE WHEN c.due_at <= $3 THEN 'due' WHEN c.due_at IS NULL THEN 'new' ELSE 'early' END AS bucket
		FROM cards c JOIN scoped_folders f ON f.id=c.folder_id WHERE c.user_id=$1
	)
	SELECT id,front,bucket FROM candidates
	ORDER BY CASE bucket WHEN 'due' THEN 1 WHEN 'new' THEN 2 ELSE 3 END,
		CASE WHEN bucket IN ('due','early') THEN due_at END,
		hashtextextended(id::text,$5)
	LIMIT $4`, userID, scope.FolderID, now, count, seed)
	if err != nil {
		return Session{}, err
	}
	type selected struct {
		id     uuid.UUID
		front  string
		bucket string
	}
	items := make([]selected, 0, count)
	for rows.Next() {
		var item selected
		if err = rows.Scan(&item.id, &item.front, &item.bucket); err != nil {
			rows.Close()
			return Session{}, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return Session{}, err
	}
	rows.Close()
	if len(items) == 0 {
		return Session{}, ErrEmptyScope
	}
	_, err = tx.Exec(ctx, `INSERT INTO study_sessions(id,user_id,scope_type,scope_folder_id,requested_count,actual_count,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, sessionID, userID, scope.Type, scope.FolderID, count, len(items), now)
	if err != nil {
		return Session{}, err
	}
	for i, item := range items {
		if _, err = tx.Exec(ctx, `INSERT INTO study_session_cards(session_id,card_id,position,selection_bucket) VALUES($1,$2,$3,$4)`, sessionID, item.id, i+1, item.bucket); err != nil {
			return Session{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return Session{ID: sessionID, ScopeType: scope.Type, ScopeFolderID: scope.FolderID, RequestedCount: count, ActualCount: len(items), Status: "active", CreatedAt: now, First: Position{Position: 1, CardID: items[0].id, Front: items[0].front}}, nil
}
