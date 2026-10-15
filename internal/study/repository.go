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

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

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

func readState(ctx context.Context, q rowQuerier, userID, sessionID uuid.UUID) (SessionState, error) {
	var state SessionState
	err := q.QueryRow(ctx, `SELECT id,actual_count,status,
		(SELECT count(*)::int FROM study_session_cards WHERE session_id=study_sessions.id AND result IS NOT NULL)
		FROM study_sessions WHERE user_id=$1 AND id=$2`, userID, sessionID).
		Scan(&state.ID, &state.ActualCount, &state.Status, &state.AnsweredCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionState{}, ErrSessionNotFound
	}
	if err != nil {
		return SessionState{}, err
	}
	if state.Status != "active" {
		return state, nil
	}
	var current CurrentPosition
	err = q.QueryRow(ctx, `SELECT sc.position,c.id,c.front,
		CASE WHEN sc.revealed_at IS NOT NULL THEN c.back END,
		sc.revealed_at IS NOT NULL
		FROM study_session_cards sc
		JOIN cards c ON c.id=sc.card_id AND c.user_id=$1
		WHERE sc.session_id=$2 AND sc.result IS NULL
		ORDER BY sc.position LIMIT 1`, userID, sessionID).
		Scan(&current.Position, &current.CardID, &current.Front, &current.Back, &current.Revealed)
	if errors.Is(err, pgx.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return SessionState{}, err
	}
	state.Current = &current
	return state, nil
}

func (r Repository) Active(ctx context.Context, userID uuid.UUID) (SessionState, error) {
	var sessionID uuid.UUID
	err := r.Pool.QueryRow(ctx, `SELECT id FROM study_sessions
		WHERE user_id=$1 AND status='active' ORDER BY created_at DESC LIMIT 1`, userID).Scan(&sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionState{}, ErrNoActiveSession
	}
	if err != nil {
		return SessionState{}, err
	}
	return readState(ctx, r.Pool, userID, sessionID)
}

func (r Repository) Get(ctx context.Context, userID, sessionID uuid.UUID) (SessionState, error) {
	return readState(ctx, r.Pool, userID, sessionID)
}

func lockSession(ctx context.Context, tx pgx.Tx, userID, sessionID uuid.UUID) (string, error) {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM study_sessions WHERE user_id=$1 AND id=$2 FOR UPDATE`, userID, sessionID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrSessionNotFound
	}
	return status, err
}

func currentPosition(ctx context.Context, q rowQuerier, sessionID uuid.UUID) (int, error) {
	var position int
	err := q.QueryRow(ctx, `SELECT position FROM study_session_cards
		WHERE session_id=$1 AND result IS NULL ORDER BY position LIMIT 1`, sessionID).Scan(&position)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrPositionConflict
	}
	return position, err
}

func (r Repository) Reveal(ctx context.Context, userID, sessionID uuid.UUID, position int, now time.Time) (SessionState, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return SessionState{}, err
	}
	defer tx.Rollback(ctx)
	status, err := lockSession(ctx, tx, userID, sessionID)
	if err != nil {
		return SessionState{}, err
	}
	if status != "active" {
		return SessionState{}, ErrSessionInactive
	}
	current, err := currentPosition(ctx, tx, sessionID)
	if err != nil {
		return SessionState{}, err
	}
	if current != position {
		return SessionState{}, ErrPositionConflict
	}
	tag, err := tx.Exec(ctx, `UPDATE study_session_cards SET revealed_at=COALESCE(revealed_at,$1)
		WHERE session_id=$2 AND position=$3 AND result IS NULL`, now, sessionID, position)
	if err != nil {
		return SessionState{}, err
	}
	if tag.RowsAffected() != 1 {
		return SessionState{}, ErrPositionConflict
	}
	state, err := readState(ctx, tx, userID, sessionID)
	if err != nil {
		return SessionState{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionState{}, err
	}
	return state, nil
}

func (r Repository) Answer(ctx context.Context, userID, sessionID uuid.UUID, position int, rating string, now time.Time) (SessionState, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return SessionState{}, err
	}
	defer tx.Rollback(ctx)
	status, err := lockSession(ctx, tx, userID, sessionID)
	if err != nil {
		return SessionState{}, err
	}
	var cardID uuid.UUID
	var bucket string
	var revealedAt *time.Time
	var savedRating *string
	err = tx.QueryRow(ctx, `SELECT card_id,selection_bucket,revealed_at,result
		FROM study_session_cards WHERE session_id=$1 AND position=$2 FOR UPDATE`, sessionID, position).
		Scan(&cardID, &bucket, &revealedAt, &savedRating)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionState{}, ErrPositionConflict
	}
	if err != nil {
		return SessionState{}, err
	}
	if savedRating != nil {
		if *savedRating != rating {
			return SessionState{}, ErrAnswerConflict
		}
		return readState(ctx, tx, userID, sessionID)
	}
	if status != "active" {
		return SessionState{}, ErrSessionInactive
	}
	current, err := currentPosition(ctx, tx, sessionID)
	if err != nil || current != position {
		if err != nil {
			return SessionState{}, err
		}
		return SessionState{}, ErrPositionConflict
	}
	if revealedAt == nil {
		return SessionState{}, ErrNotRevealed
	}
	var correctCount, incorrectCount, streak int
	var interval *int
	var previousDue *time.Time
	err = tx.QueryRow(ctx, `SELECT correct_count,incorrect_count,correct_streak,interval_days,due_at
		FROM cards WHERE user_id=$1 AND id=$2 FOR UPDATE`, userID, cardID).
		Scan(&correctCount, &incorrectCount, &streak, &interval, &previousDue)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionState{}, ErrPositionConflict
	}
	if err != nil {
		return SessionState{}, err
	}
	next := NextSchedule(rating == "correct", correctCount, incorrectCount, streak, interval, now)
	_, err = tx.Exec(ctx, `INSERT INTO review_events(
		user_id,session_id,card_id,rating,reviewed_early,previous_interval_days,next_interval_days,previous_due_at,next_due_at,created_at
	) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, userID, sessionID, cardID, rating, bucket == "early", interval, next.IntervalDays, previousDue, next.DueAt, now)
	if err != nil {
		return SessionState{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE cards SET correct_count=$1,incorrect_count=$2,correct_streak=$3,
		interval_days=$4,last_reviewed_at=$5,due_at=$6,updated_at=$5 WHERE user_id=$7 AND id=$8`,
		next.CorrectCount, next.IncorrectCount, next.CorrectStreak, next.IntervalDays, now, next.DueAt, userID, cardID)
	if err != nil {
		return SessionState{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE study_session_cards SET result=$1,answered_at=$2
		WHERE session_id=$3 AND position=$4`, rating, now, sessionID, position)
	if err != nil {
		return SessionState{}, err
	}
	var remaining int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM study_session_cards WHERE session_id=$1 AND result IS NULL`, sessionID).Scan(&remaining); err != nil {
		return SessionState{}, err
	}
	if remaining == 0 {
		if _, err = tx.Exec(ctx, `UPDATE study_sessions SET status='completed',completed_at=$1 WHERE id=$2`, now, sessionID); err != nil {
			return SessionState{}, err
		}
	}
	state, err := readState(ctx, tx, userID, sessionID)
	if err != nil {
		return SessionState{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return SessionState{}, err
	}
	return state, nil
}

func (r Repository) Abandon(ctx context.Context, userID, sessionID uuid.UUID, now time.Time) error {
	tag, err := r.Pool.Exec(ctx, `UPDATE study_sessions SET status='abandoned',completed_at=$1
		WHERE user_id=$2 AND id=$3 AND status='active'`, now, userID, sessionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var status string
	err = r.Pool.QueryRow(ctx, `SELECT status FROM study_sessions WHERE user_id=$1 AND id=$2`, userID, sessionID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSessionNotFound
	}
	if err != nil {
		return err
	}
	return ErrSessionInactive
}
