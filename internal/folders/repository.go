package folders

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ Pool *pgxpool.Pool }

func siblingConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "folders_sibling_name_idx"
}

func (r Repository) lockRecords(ctx context.Context, tx pgx.Tx, userID uuid.UUID) ([]Record, error) {
	rows, err := tx.Query(ctx, `SELECT id,parent_id,name,is_inbox,created_at,updated_at FROM folders WHERE user_id=$1 ORDER BY id FOR UPDATE`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []Record{}
	for rows.Next() {
		var record Record
		if err = rows.Scan(&record.ID, &record.ParentID, &record.Name, &record.IsInbox, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func byID(records []Record, id uuid.UUID) *Record {
	for i := range records {
		if records[i].ID == id {
			return &records[i]
		}
	}
	return nil
}

func depth(records []Record, id uuid.UUID) (int, bool) {
	seen := map[uuid.UUID]bool{}
	d := 0
	for {
		if seen[id] {
			return 0, false
		}
		seen[id] = true
		record := byID(records, id)
		if record == nil {
			return 0, false
		}
		if record.ParentID == nil {
			return d, true
		}
		d++
		id = *record.ParentID
	}
}

func isDescendant(records []Record, ancestor, candidate uuid.UUID) bool {
	seen := map[uuid.UUID]bool{}
	for id := candidate; ; {
		if seen[id] {
			return false
		}
		seen[id] = true
		record := byID(records, id)
		if record == nil || record.ParentID == nil {
			return false
		}
		if *record.ParentID == ancestor {
			return true
		}
		id = *record.ParentID
	}
}

func subtreeHeight(records []Record, root uuid.UUID) int {
	max := 0
	seen := map[uuid.UUID]bool{}
	var visit func(uuid.UUID, int)
	visit = func(parent uuid.UUID, current int) {
		if seen[parent] {
			return
		}
		seen[parent] = true
		if current > max {
			max = current
		}
		for _, record := range records {
			if record.ParentID != nil && *record.ParentID == parent {
				visit(record.ID, current+1)
			}
		}
	}
	visit(root, 0)
	return max
}

func (r Repository) List(ctx context.Context, userID uuid.UUID) ([]Record, error) {
	rows, err := r.Pool.Query(ctx, `SELECT f.id,f.parent_id,f.name,f.is_inbox,f.created_at,f.updated_at,count(c.id)::int
		FROM folders f LEFT JOIN cards c ON c.user_id=f.user_id AND c.folder_id=f.id
		WHERE f.user_id=$1 GROUP BY f.id ORDER BY f.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []Record{}
	for rows.Next() {
		var record Record
		if err = rows.Scan(&record.ID, &record.ParentID, &record.Name, &record.IsInbox, &record.CreatedAt, &record.UpdatedAt, &record.DirectCardCount); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (r Repository) Create(ctx context.Context, id, userID, parentID uuid.UUID, name, normalized string, now time.Time) (Folder, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Folder{}, err
	}
	defer tx.Rollback(ctx)
	records, err := r.lockRecords(ctx, tx, userID)
	if err != nil {
		return Folder{}, err
	}
	if byID(records, parentID) == nil {
		return Folder{}, ErrNotFound
	}
	parentDepth, ok := depth(records, parentID)
	if !ok {
		return Folder{}, ErrNotFound
	}
	if parentDepth+1 > MaxDepth {
		return Folder{}, ErrDepthExceeded
	}
	folder := Folder{ID: id, ParentID: parentID, Name: name, CreatedAt: now, UpdatedAt: now}
	err = tx.QueryRow(ctx, `INSERT INTO folders(id,user_id,parent_id,name,name_normalized,is_inbox,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,false,$6,$6) RETURNING is_inbox`, id, userID, parentID, name, normalized, now).Scan(&folder.IsInbox)
	if siblingConflict(err) {
		return Folder{}, ErrNameTaken
	}
	if err != nil {
		return Folder{}, err
	}
	return folder, tx.Commit(ctx)
}

func (r Repository) Update(ctx context.Context, userID, folderID uuid.UUID, name, normalized *string, parentID *uuid.UUID, now time.Time) (Folder, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Folder{}, err
	}
	defer tx.Rollback(ctx)
	records, err := r.lockRecords(ctx, tx, userID)
	if err != nil {
		return Folder{}, err
	}
	current := byID(records, folderID)
	if current == nil {
		return Folder{}, ErrNotFound
	}
	if current.IsInbox {
		return Folder{}, ErrInboxImmutable
	}
	nextParent := *current.ParentID
	if parentID != nil {
		nextParent = *parentID
		if nextParent == folderID || isDescendant(records, folderID, nextParent) {
			return Folder{}, ErrCycle
		}
		if byID(records, nextParent) == nil {
			return Folder{}, ErrNotFound
		}
		parentDepth, ok := depth(records, nextParent)
		if !ok {
			return Folder{}, ErrNotFound
		}
		if parentDepth+1+subtreeHeight(records, folderID) > MaxDepth {
			return Folder{}, ErrDepthExceeded
		}
	}
	nextName := current.Name
	nextNormalized := ""
	if name != nil {
		nextName, nextNormalized = *name, *normalized
	} else if err = tx.QueryRow(ctx, `SELECT name_normalized FROM folders WHERE user_id=$1 AND id=$2`, userID, folderID).Scan(&nextNormalized); err != nil {
		return Folder{}, err
	}
	folder := Folder{ID: folderID, ParentID: nextParent, Name: nextName, IsInbox: false, CreatedAt: current.CreatedAt, UpdatedAt: now}
	_, err = tx.Exec(ctx, `UPDATE folders SET parent_id=$3,name=$4,name_normalized=$5,updated_at=$6 WHERE user_id=$1 AND id=$2`, userID, folderID, nextParent, nextName, nextNormalized, now)
	if siblingConflict(err) {
		return Folder{}, ErrNameTaken
	}
	if err != nil {
		return Folder{}, err
	}
	return folder, tx.Commit(ctx)
}

func (r Repository) Delete(ctx context.Context, userID, folderID uuid.UUID) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	records, err := r.lockRecords(ctx, tx, userID)
	if err != nil {
		return err
	}
	current := byID(records, folderID)
	if current == nil {
		return ErrNotFound
	}
	if current.IsInbox {
		return ErrInboxImmutable
	}
	var occupied bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM folders WHERE user_id=$1 AND parent_id=$2) OR EXISTS(SELECT 1 FROM cards WHERE user_id=$1 AND folder_id=$2)`, userID, folderID).Scan(&occupied)
	if err != nil {
		return err
	}
	if occupied {
		return ErrNotEmpty
	}
	if _, err = tx.Exec(ctx, `DELETE FROM folders WHERE user_id=$1 AND id=$2`, userID, folderID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
