//go:build integration

package auth

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func integrationRepo(t *testing.T) (Repository, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), "TRUNCATE folders,auth_sessions,users CASCADE"); err != nil {
		pool.Close()
		t.Fatalf("apply migrations first: %v", err)
	}
	t.Cleanup(pool.Close)
	return Repository{Pool: pool}, pool
}

func TestRegisterIsAtomicAndCaseInsensitive(t *testing.T) {
	repo, pool := integrationRepo(t)
	now := time.Now().UTC()
	register := func(login, normalized string) error {
		return repo.Register(context.Background(), User{ID: uuid.New(), Login: login}, normalized, "$argon2id$test", uuid.New(), make([]byte, 32), now, now.Add(time.Hour))
	}
	if err := register("Alex", "alex"); err != nil {
		t.Fatal(err)
	}
	if err := register("alex", "alex"); !errors.Is(err, ErrLoginTaken) {
		t.Fatalf("duplicate: %v", err)
	}
	var users, folders, sessions int
	if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM folders),(SELECT count(*) FROM auth_sessions)`).Scan(&users, &folders, &sessions); err != nil {
		t.Fatal(err)
	}
	if users != 1 || folders != 1 || sessions != 1 {
		t.Fatalf("partial data: %d/%d/%d", users, folders, sessions)
	}
	var name string
	var inbox bool
	if err := pool.QueryRow(context.Background(), `SELECT name,is_inbox FROM folders`).Scan(&name, &inbox); err != nil || name != "Inbox" || !inbox {
		t.Fatalf("inbox: %q %v %v", name, inbox, err)
	}
}

func TestConcurrentRegisterHasOneWinner(t *testing.T) {
	repo, pool := integrationRepo(t)
	now := time.Now().UTC()
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, login := range []string{"RaceUser", "raceuser"} {
		wg.Add(1)
		go func(login string) {
			defer wg.Done()
			<-start
			results <- repo.Register(context.Background(), User{ID: uuid.New(), Login: login}, "raceuser", "$argon2id$test", uuid.New(), make([]byte, 32), now, now.Add(time.Hour))
		}(login)
	}
	close(start)
	wg.Wait()
	close(results)
	success, taken := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrLoginTaken) {
			taken++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || taken != 1 {
		t.Fatalf("success/taken=%d/%d", success, taken)
	}
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM folders WHERE is_inbox`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("inboxes=%d err=%v", n, err)
	}
}

func TestRegisterRollsBackOnSessionConstraint(t *testing.T) {
	repo, pool := integrationRepo(t)
	now := time.Now().UTC()
	err := repo.Register(context.Background(), User{ID: uuid.New(), Login: "Rollback"}, "rollback", "hash", uuid.New(), []byte{1}, now, now.Add(time.Hour))
	if err == nil {
		t.Fatal("expected constraint error")
	}
	var n int
	if e := pool.QueryRow(context.Background(), `SELECT count(*) FROM users`).Scan(&n); e != nil || n != 0 {
		t.Fatalf("users=%d err=%v", n, e)
	}
}

func TestFolderOwnerConstraint(t *testing.T) {
	repo, pool := integrationRepo(t)
	now := time.Now().UTC()
	u1, u2 := uuid.New(), uuid.New()
	for index, u := range []User{{ID: u1, Login: "OwnerOne"}, {ID: u2, Login: "OwnerTwo"}} {
		tokenHash := make([]byte, 32)
		tokenHash[0] = byte(index + 1)
		if err := repo.Register(context.Background(), u, NormalizeLogin(u.Login), "hash", uuid.New(), tokenHash, now, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	var parent uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT id FROM folders WHERE user_id=$1`, u1).Scan(&parent); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO folders(user_id,parent_id,name,name_normalized,is_inbox,created_at,updated_at) VALUES($1,$2,'Bad','bad',false,$3,$3)`, u2, parent, now); err == nil {
		t.Fatal("cross-user parent accepted")
	}
}

type countingHasher struct{ calls int }

func (*countingHasher) Hash(string) (string, error)           { return "hash", nil }
func (h *countingHasher) Verify(string, string) (bool, error) { h.calls++; return false, nil }

func TestEveryPermittedLoginPerformsOneVerify(t *testing.T) {
	repo, _ := integrationRepo(t)
	now := time.Now().UTC()
	if err := repo.Register(context.Background(), User{ID: uuid.New(), Login: "Existing"}, "existing", "stored-hash", uuid.New(), make([]byte, 32), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	for _, login := range []string{"Existing", "Unknown"} {
		h := &countingHasher{}
		service := Service{Repo: repo, Hasher: h, Clock: &fakeClock{now}, DummyHash: "dummy-hash", TTL: time.Hour}
		_, _, _, err := service.Login(context.Background(), login, "password")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s error %v", login, err)
		}
		if h.calls != 1 {
			t.Fatalf("%s verifies=%d", login, h.calls)
		}
	}
}

func TestSessionExpiryAndIsolation(t *testing.T) {
	repo, _ := integrationRepo(t)
	now := time.Now().UTC()
	user := User{ID: uuid.New(), Login: "Sessions"}
	h1, h2 := make([]byte, 32), make([]byte, 32)
	h1[0] = 1
	h2[0] = 2
	if err := repo.Register(context.Background(), user, "sessions", "hash", uuid.New(), h1, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateSession(context.Background(), uuid.New(), user.ID, h2, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteSession(context.Background(), h1); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CurrentUser(context.Background(), h1, now, time.Minute); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted session: %v", err)
	}
	if got, err := repo.CurrentUser(context.Background(), h2, now, time.Minute); err != nil || got.ID != user.ID {
		t.Fatalf("other session: %#v %v", got, err)
	}
	if _, err := repo.CurrentUser(context.Background(), h2, now.Add(2*time.Hour), time.Minute); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired session: %v", err)
	}
}

func TestFolderRootAndInboxConstraints(t *testing.T) {
	repo, pool := integrationRepo(t)
	now := time.Now().UTC()
	user := User{ID: uuid.New(), Login: "Constraints"}
	if err := repo.Register(context.Background(), user, "constraints", "hash", uuid.New(), make([]byte, 32), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	assertConstraint := func(name, sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(context.Background(), sql, args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != name {
			t.Fatalf("expected %s, got %v", name, err)
		}
	}
	assertConstraint("folders_inbox_canonical", `INSERT INTO folders(user_id,name,name_normalized,is_inbox,created_at,updated_at) VALUES($1,'Not Inbox','not inbox',true,$2,$2)`, user.ID, now)
	assertConstraint("folders_only_inbox_root", `INSERT INTO folders(user_id,name,name_normalized,is_inbox,created_at,updated_at) VALUES($1,'Root','root',false,$2,$2)`, user.ID, now)
	rows, err := pool.Query(context.Background(), `SELECT indexname,indexdef FROM pg_indexes WHERE schemaname=current_schema() AND tablename='folders' AND indexname IN ('folders_one_root_per_user_idx','folders_one_inbox_per_user_idx')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	indexes := map[string]string{}
	for rows.Next() {
		var name, definition string
		if err = rows.Scan(&name, &definition); err != nil {
			t.Fatal(err)
		}
		indexes[name] = definition
	}
	if len(indexes) != 2 || !strings.Contains(indexes["folders_one_root_per_user_idx"], "UNIQUE") || !strings.Contains(indexes["folders_one_root_per_user_idx"], "parent_id IS NULL") || !strings.Contains(indexes["folders_one_inbox_per_user_idx"], "UNIQUE") || !strings.Contains(indexes["folders_one_inbox_per_user_idx"], "is_inbox") {
		t.Fatalf("partial indexes: %#v", indexes)
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO folders(user_id,name,name_normalized,is_inbox,created_at,updated_at) VALUES($1,'Inbox','inbox',true,$2,$2)`, user.ID, now)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("duplicate root/inbox accepted: %v", err)
	}
}

func TestSessionSchemaConstraints(t *testing.T) {
	repo, pool := integrationRepo(t)
	now := time.Now().UTC()
	user := User{ID: uuid.New(), Login: "SessionRules"}
	hash := make([]byte, 32)
	hash[0] = 1
	if err := repo.Register(context.Background(), user, "sessionrules", "hash", uuid.New(), hash, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	assertConstraint := func(name string, token []byte, expires time.Time) {
		t.Helper()
		_, err := pool.Exec(context.Background(), `INSERT INTO auth_sessions(id,token_hash,user_id,expires_at,created_at,last_seen_at) VALUES($1,$2,$3,$4,$5,$5)`, uuid.New(), token, user.ID, expires, now)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != name {
			t.Fatalf("expected %s, got %v", name, err)
		}
	}
	assertConstraint("auth_sessions_token_hash_length", make([]byte, 31), now.Add(time.Hour))
	other := make([]byte, 32)
	other[0] = 2
	assertConstraint("auth_sessions_expiry", other, now)
	if err := repo.CreateSession(context.Background(), uuid.New(), user.ID, hash, now, now.Add(time.Hour)); err == nil {
		t.Fatal("duplicate token hash accepted")
	}
}
