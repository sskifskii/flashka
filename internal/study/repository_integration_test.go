//go:build integration

package study

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryPreviewAndCreateUseOwnedRecursiveFixedScope(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, "TRUNCATE study_sessions,cards,folders,auth_sessions,users CASCADE"); err != nil {
		t.Fatalf("apply migrations first: %v", err)
	}
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	seedUser := func(login string) (uuid.UUID, uuid.UUID) {
		t.Helper()
		userID, inboxID := uuid.New(), uuid.New()
		_, e := pool.Exec(ctx, `INSERT INTO users(id,login,login_normalized,password_hash,created_at,updated_at) VALUES($1,$2,$2,'hash',$3,$3)`, userID, login, now)
		if e == nil {
			_, e = pool.Exec(ctx, `INSERT INTO folders(id,user_id,name,name_normalized,is_inbox,created_at,updated_at) VALUES($1,$2,'Inbox','inbox',true,$3,$3)`, inboxID, userID, now)
		}
		if e != nil {
			t.Fatal(e)
		}
		return userID, inboxID
	}
	userID, inboxID := seedUser("studyowner")
	otherUserID, otherInboxID := seedUser("studyother")
	childID := uuid.New()
	if _, err = pool.Exec(ctx, `INSERT INTO folders(id,user_id,parent_id,name,name_normalized,is_inbox,created_at,updated_at) VALUES($1,$2,$3,'Child','child',false,$4,$4)`, childID, userID, inboxID, now); err != nil {
		t.Fatal(err)
	}
	insertCard := func(folderID uuid.UUID, dueAt *time.Time, interval *int) uuid.UUID {
		t.Helper()
		cardID := uuid.New()
		if dueAt == nil {
			_, err = pool.Exec(ctx, `INSERT INTO cards(id,user_id,folder_id,front,back,created_at,updated_at) VALUES($1,$2,$3,$4,'back',$5,$5)`, cardID, userID, folderID, cardID.String(), now)
		} else {
			_, err = pool.Exec(ctx, `INSERT INTO cards(id,user_id,folder_id,front,back,correct_count,correct_streak,interval_days,last_reviewed_at,due_at,created_at,updated_at) VALUES($1,$2,$3,$4,'back',1,1,$5,$6,$7,$6,$6)`, cardID, userID, folderID, cardID.String(), *interval, now.Add(-24*time.Hour), *dueAt)
		}
		if err != nil {
			t.Fatal(err)
		}
		return cardID
	}
	one, thirty := 1, 30
	dueOld := now.Add(-2 * time.Hour)
	dueRecent := now.Add(-time.Hour)
	early := now.Add(24 * time.Hour)
	insertCard(inboxID, &dueRecent, &thirty)
	insertCard(childID, &dueOld, &one)
	insertCard(childID, nil, nil)
	insertCard(childID, &early, &one)

	repo := Repository{Pool: pool}
	preview, err := repo.Preview(ctx, userID, Scope{Type: "folder", FolderID: &inboxID}, now)
	if err != nil || preview != (Preview{Available: 4, Due: 2, New: 1, Mastered: 1}) {
		t.Fatalf("preview=%#v err=%v", preview, err)
	}
	session, err := repo.Create(ctx, uuid.New(), userID, Scope{Type: "folder", FolderID: &inboxID}, 3, now, 42)
	if err != nil || session.ActualCount != 3 {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	rows, err := pool.Query(ctx, `SELECT selection_bucket FROM study_session_cards WHERE session_id=$1 ORDER BY position`, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var buckets []string
	for rows.Next() {
		var bucket string
		if err = rows.Scan(&bucket); err != nil {
			t.Fatal(err)
		}
		buckets = append(buckets, bucket)
	}
	if len(buckets) != 3 || buckets[0] != "due" || buckets[1] != "due" || buckets[2] != "new" {
		t.Fatalf("buckets=%v", buckets)
	}
	if _, err = repo.Preview(ctx, userID, Scope{Type: "folder", FolderID: &otherInboxID}, now); !errors.Is(err, ErrFolderNotFound) {
		t.Fatalf("cross-tenant preview err=%v other=%s", err, otherUserID)
	}
	if _, err = repo.Create(ctx, uuid.New(), otherUserID, Scope{Type: "all"}, 5, now, 1); !errors.Is(err, ErrEmptyScope) {
		t.Fatalf("empty scope err=%v", err)
	}
}
