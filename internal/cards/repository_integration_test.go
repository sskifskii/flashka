//go:build integration

package cards

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func cardsPool(t *testing.T) *pgxpool.Pool {
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

func seedUser(t *testing.T, pool *pgxpool.Pool, login string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	u, folder := uuid.New(), uuid.New()
	now := time.Now().UTC()
	_, err := pool.Exec(context.Background(), `INSERT INTO users(id,login,login_normalized,password_hash,created_at,updated_at) VALUES($1,$2,$3,'hash',$4,$4)`, u, login, login, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO folders(id,user_id,name,name_normalized,is_inbox,created_at,updated_at) VALUES($1,$2,'Inbox','inbox',true,$3,$3)`, folder, u, now)
	if err != nil {
		t.Fatal(err)
	}
	return u, folder
}

func TestRepositoryCRUDPaginationAndTenantIsolation(t *testing.T) {
	pool := cardsPool(t)
	repo := Repository{Pool: pool}
	u1, f1 := seedUser(t, pool, "ownerone")
	u2, _ := seedUser(t, pool, "ownertwo")
	base := time.Now().UTC().Truncate(time.Microsecond)
	cards := make([]Card, 3)
	for i := range cards {
		var err error
		cards[i], err = repo.Create(context.Background(), uuid.New(), u1, f1, "front", "back", base.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
	}
	first, err := repo.List(context.Background(), u1, f1, nil, 2)
	if err != nil || len(first) != 2 || first[0].ID != cards[2].ID || first[1].ID != cards[1].ID {
		t.Fatalf("page=%#v err=%v", first, err)
	}
	second, err := repo.List(context.Background(), u1, f1, &Cursor{CreatedAt: first[1].CreatedAt, ID: first[1].ID}, 2)
	if err != nil || len(second) != 1 || second[0].ID != cards[0].ID {
		t.Fatalf("second=%#v err=%v", second, err)
	}
	if _, err = repo.Get(context.Background(), u2, cards[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross tenant get=%v", err)
	}
	if _, err = repo.Update(context.Background(), u2, cards[0].ID, f1, "x", "y", base); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross tenant update=%v", err)
	}
	if err = repo.Delete(context.Background(), u2, cards[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross tenant delete=%v", err)
	}
	updated, err := repo.Update(context.Background(), u1, cards[0].ID, f1, "new front", "new back", base.Add(time.Hour))
	if err != nil || updated.CreatedAt != cards[0].CreatedAt || updated.CorrectCount != 0 {
		t.Fatalf("updated=%#v err=%v", updated, err)
	}
	if err = repo.Delete(context.Background(), u1, cards[0].ID); err != nil {
		t.Fatal(err)
	}
	if err = repo.Delete(context.Background(), u1, cards[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("repeat delete=%v", err)
	}
}

func TestCardSchemaRejectsCrossTenantFolderAndInvalidStates(t *testing.T) {
	pool := cardsPool(t)
	u1, _ := seedUser(t, pool, "schemaone")
	_, f2 := seedUser(t, pool, "schematwo")
	now := time.Now().UTC()
	assertConstraint := func(name, sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(context.Background(), sql, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != name {
			t.Fatalf("expected %s got %v", name, err)
		}
	}
	base := `INSERT INTO cards(user_id,folder_id,front,back,created_at,updated_at) VALUES($1,$2,$3,'back',$4,$4)`
	assertConstraint("cards_folder_owner_fk", base, u1, f2, "front", now)
	var f1 uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT id FROM folders WHERE user_id=$1`, u1).Scan(&f1); err != nil {
		t.Fatal(err)
	}
	assertConstraint("cards_front_length", base, u1, f1, "", now)
	assertConstraint("cards_front_length", base, u1, f1, strings.Repeat("x", 2001), now)
	assertConstraint("cards_schedule_state", `INSERT INTO cards(user_id,folder_id,front,back,correct_count,created_at,updated_at) VALUES($1,$2,'front','back',1,$3,$3)`, u1, f1, now)
}
