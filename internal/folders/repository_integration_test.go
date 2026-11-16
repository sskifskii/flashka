//go:build integration

package folders

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func foldersPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), "TRUNCATE cards,folders,auth_sessions,users CASCADE"); err != nil {
		pool.Close()
		t.Fatalf("apply migrations first: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func folderUser(t *testing.T, pool *pgxpool.Pool, login string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	userID, inboxID := uuid.New(), uuid.New()
	now := time.Now().UTC()
	_, err := pool.Exec(context.Background(), `INSERT INTO users(id,login,login_normalized,password_hash,created_at,updated_at) VALUES($1,$2,$2,'hash',$3,$3)`, userID, login, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO folders(id,user_id,name,name_normalized,is_inbox,created_at,updated_at) VALUES($1,$2,'Inbox','inbox',true,$3,$3)`, inboxID, userID, now)
	if err != nil {
		t.Fatal(err)
	}
	return userID, inboxID
}

func TestRepositoryTreeMutationsAndTenantIsolation(t *testing.T) {
	pool := foldersPool(t)
	repo := Repository{Pool: pool}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID, inboxID := folderUser(t, pool, "folderone")
	otherID, otherInbox := folderUser(t, pool, "foldertwo")

	topic, err := repo.Create(ctx, uuid.New(), userID, inboxID, "Topic", "topic", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, uuid.New(), userID, inboxID, "TOPIC", "topic", now); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("duplicate=%v", err)
	}
	child, err := repo.Create(ctx, uuid.New(), userID, topic.ID, "Child", "child", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Update(ctx, userID, topic.ID, nil, nil, &child.ID, now.Add(time.Second)); !errors.Is(err, ErrCycle) {
		t.Fatalf("cycle=%v", err)
	}
	if _, err = repo.Update(ctx, otherID, topic.ID, nil, nil, &otherInbox, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant update=%v", err)
	}
	if err = repo.Delete(ctx, otherID, topic.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant delete=%v", err)
	}
	if err = repo.Delete(ctx, userID, inboxID); !errors.Is(err, ErrInboxImmutable) {
		t.Fatalf("inbox delete=%v", err)
	}
	if err = repo.Delete(ctx, userID, topic.ID); !errors.Is(err, ErrNotEmpty) {
		t.Fatalf("folder with child delete=%v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO cards(user_id,folder_id,front,back,created_at,updated_at) VALUES($1,$2,'front','back',$3,$3)`, userID, child.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.Delete(ctx, userID, child.ID); !errors.Is(err, ErrNotEmpty) {
		t.Fatalf("folder with card delete=%v", err)
	}
	records, err := repo.List(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := BuildTree(records)
	if err != nil || tree.TotalCardCount != 1 || tree.Children[0].TotalCardCount != 1 || tree.Children[0].Children[0].DirectCardCount != 1 {
		t.Fatalf("tree=%#v err=%v", tree, err)
	}
}

func TestRepositoryRejectsDepthElevenAndSchemaInvalidNames(t *testing.T) {
	pool := foldersPool(t)
	repo := Repository{Pool: pool}
	ctx := context.Background()
	now := time.Now().UTC()
	userID, parentID := folderUser(t, pool, "folderdepth")
	rootID := parentID
	var levelNine uuid.UUID
	for level := 1; level <= MaxDepth; level++ {
		folder, err := repo.Create(ctx, uuid.New(), userID, parentID, fmt.Sprintf("Level %d", level), fmt.Sprintf("level %d", level), now)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		parentID = folder.ID
		if level == 9 {
			levelNine = folder.ID
		}
	}
	if _, err := repo.Create(ctx, uuid.New(), userID, parentID, "Too deep", "too deep", now); !errors.Is(err, ErrDepthExceeded) {
		t.Fatalf("depth=%v", err)
	}
	branch, err := repo.Create(ctx, uuid.New(), userID, rootID, "Branch", "branch", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, uuid.New(), userID, branch.ID, "Leaf", "leaf", now); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Update(ctx, userID, branch.ID, nil, nil, &levelNine, now); !errors.Is(err, ErrDepthExceeded) {
		t.Fatalf("moved subtree depth=%v", err)
	}
	_, inboxID := folderUser(t, pool, "folderschema")
	_, err = pool.Exec(ctx, `INSERT INTO folders(user_id,parent_id,name,name_normalized,created_at,updated_at)
		SELECT user_id,$1,'','empty',$2,$2 FROM folders WHERE id=$1`, inboxID, now)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "folders_name_length" {
		t.Fatalf("expected folders_name_length, got %v", err)
	}
}
