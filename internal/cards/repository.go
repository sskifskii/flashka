package cards

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ Pool *pgxpool.Pool }

const cardColumns = `id,folder_id,front,back,correct_count,incorrect_count,correct_streak,interval_days,last_reviewed_at,due_at,created_at,updated_at`

func scanCard(row pgx.Row) (Card, error) {
	var c Card
	err := row.Scan(&c.ID, &c.FolderID, &c.Front, &c.Back, &c.CorrectCount, &c.IncorrectCount, &c.CorrectStreak, &c.IntervalDays, &c.LastReviewedAt, &c.DueAt, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Card{}, ErrNotFound
	}
	return c, err
}

func (r Repository) Create(ctx context.Context, id, userID uuid.UUID, front, back string, now time.Time) (Card, error) {
	return scanCard(r.Pool.QueryRow(ctx, `INSERT INTO cards(id,user_id,folder_id,front,back,created_at,updated_at)
		SELECT $1,$2,id,$3,$4,$5,$5 FROM folders WHERE user_id=$2 AND is_inbox
		RETURNING `+cardColumns, id, userID, front, back, now))
}

func (r Repository) Get(ctx context.Context, userID, cardID uuid.UUID) (Card, error) {
	return scanCard(r.Pool.QueryRow(ctx, `SELECT `+cardColumns+` FROM cards WHERE user_id=$1 AND id=$2`, userID, cardID))
}

func (r Repository) Update(ctx context.Context, userID, cardID uuid.UUID, front, back string, now time.Time) (Card, error) {
	return scanCard(r.Pool.QueryRow(ctx, `UPDATE cards SET front=$3,back=$4,updated_at=$5 WHERE user_id=$1 AND id=$2 RETURNING `+cardColumns, userID, cardID, front, back, now))
}

func (r Repository) Delete(ctx context.Context, userID, cardID uuid.UUID) error {
	tag, err := r.Pool.Exec(ctx, `DELETE FROM cards WHERE user_id=$1 AND id=$2`, userID, cardID)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (r Repository) List(ctx context.Context, userID uuid.UUID, cursor *Cursor, limit int) ([]Summary, error) {
	var rows pgx.Rows
	var err error
	if cursor == nil {
		rows, err = r.Pool.Query(ctx, `SELECT c.id,c.front,c.back,c.created_at,c.updated_at FROM cards c JOIN folders f ON f.user_id=c.user_id AND f.id=c.folder_id WHERE c.user_id=$1 AND f.is_inbox ORDER BY c.created_at DESC,c.id DESC LIMIT $2`, userID, limit)
	} else {
		rows, err = r.Pool.Query(ctx, `SELECT c.id,c.front,c.back,c.created_at,c.updated_at FROM cards c JOIN folders f ON f.user_id=c.user_id AND f.id=c.folder_id WHERE c.user_id=$1 AND f.is_inbox AND (c.created_at,c.id)<($2,$3) ORDER BY c.created_at DESC,c.id DESC LIMIT $4`, userID, cursor.CreatedAt, cursor.ID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Summary, 0, limit)
	for rows.Next() {
		var item Summary
		if err = rows.Scan(&item.ID, &item.Front, &item.Back, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
