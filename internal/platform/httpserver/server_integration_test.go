//go:build integration

package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skif/flashka/internal/auth"
	"github.com/skif/flashka/internal/platform/config"
)

func TestAuthHTTPContract(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(context.Background(), "TRUNCATE folders,auth_sessions,users CASCADE"); err != nil {
		t.Fatalf("apply migrations first: %v", err)
	}
	hasher := auth.PasswordHasher{Memory: 19456, Iterations: 2, Parallelism: 1}
	dummy, err := hasher.Hash("dummy password")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Environment: "development", CookieName: "flashka_session", SessionTTL: time.Hour, LastSeenInterval: time.Minute, ArgonMemory: 19456, ArgonIterations: 2, ArgonParallelism: 1}
	service := auth.Service{Repo: auth.Repository{Pool: pool}, Hasher: hasher, Tokens: auth.TokenSource{}, Clock: auth.RealClock{}, TTL: cfg.SessionTTL, LastSeen: cfg.LastSeenInterval, DummyHash: dummy}
	server := httptest.NewUnstartedServer(nil)
	cfg.TrustedOrigin = "http://" + server.Listener.Addr().String()
	var logs bytes.Buffer
	server.Config.Handler = New(cfg, pool, service, slog.New(slog.NewJSONHandler(&logs, nil)))
	server.Start()
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	for _, path := range []string{"/health/live", "/health/ready"} {
		res, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("%s=%d", path, res.StatusCode)
		}
		assertSecurityHeaders(t, res.Header)
	}
	request := func(method, path string, body any) (int, map[string]any, http.Header) {
		t.Helper()
		var content io.Reader
		if body != nil {
			encoded, _ := json.Marshal(body)
			content = bytes.NewReader(encoded)
		}
		req, e := http.NewRequest(method, server.URL+path, content)
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Origin", cfg.TrustedOrigin)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		decoded := map[string]any{}
		if res.StatusCode != 204 {
			if e = json.NewDecoder(res.Body).Decode(&decoded); e != nil {
				t.Fatal(e)
			}
		}
		assertAuthHeaders(t, res.Header)
		return res.StatusCode, decoded, res.Header
	}
	if status, _, _ := request("POST", "/api/v1/auth/register", map[string]string{"login": "ContractUser", "password": "exact password"}); status != 201 {
		t.Fatalf("register=%d", status)
	}
	var passwordHash string
	if err := pool.QueryRow(context.Background(), `SELECT password_hash FROM users WHERE login_normalized='contractuser'`).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	cookies := jar.Cookies(mustURL(t, server.URL))
	if len(cookies) == 0 {
		t.Fatal("session cookie missing")
	}
	rawToken := cookies[0].Value
	if status, body, _ := request("GET", "/api/v1/auth/me", nil); status != 200 || body["user"] == nil {
		t.Fatalf("me=%d %#v", status, body)
	}
	if status, _, _ := request("POST", "/api/v1/auth/logout", nil); status != 204 {
		t.Fatalf("logout=%d", status)
	}
	if status, body, headers := request("GET", "/api/v1/auth/me", nil); status != 401 || errorCode(body) != "unauthenticated" || headers.Get("Set-Cookie") != "" {
		t.Fatalf("unauthenticated=%d %#v", status, body)
	}
	jar.SetCookies(mustURL(t, server.URL), []*http.Cookie{{Name: cfg.CookieName, Value: "broken", Path: "/"}})
	if status, _, headers := request("GET", "/api/v1/auth/me", nil); status != 401 || headers.Get("Set-Cookie") == "" {
		t.Fatalf("invalid cookie not cleared: %d %q", status, headers.Get("Set-Cookie"))
	}
	if status, _, _ := request("POST", "/api/v1/auth/login", map[string]string{"login": "contractuser", "password": "exact password"}); status != 200 {
		t.Fatalf("case-insensitive login=%d", status)
	}
	if status, body, _ := request("POST", "/api/v1/auth/login", map[string]string{"login": "ContractUser", "password": "wrong password"}); status != 401 || errorCode(body) != "invalid_credentials" {
		t.Fatalf("bad password=%d %#v", status, body)
	}
	if status, body, _ := request("POST", "/api/v1/auth/login", map[string]string{"login": "UnknownUser", "password": "wrong password"}); status != 401 || errorCode(body) != "invalid_credentials" {
		t.Fatalf("unknown=%d %#v", status, body)
	}
	if status, body, _ := request("POST", "/api/v1/cards", map[string]string{"front": "  private front  ", "back": "private\nback"}); status != 201 {
		t.Fatalf("create card=%d %#v", status, body)
	}
	status, body, _ := request("GET", "/api/v1/cards?limit=1", nil)
	if status != 200 {
		t.Fatalf("list cards=%d %#v", status, body)
	}
	items, _ := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items=%#v", body)
	}
	item, _ := items[0].(map[string]any)
	cardID, _ := item["id"].(string)
	if item["front"] != "private front" || cardID == "" {
		t.Fatalf("card=%#v", item)
	}
	if status, body, _ = request("PATCH", "/api/v1/cards/"+cardID, map[string]string{"front": "changed front"}); status != 200 {
		t.Fatalf("patch=%d %#v", status, body)
	}
	if status, body, _ = request("POST", "/api/v1/cards", map[string]string{"front": "", "back": "ok"}); status != 422 || errorCode(body) != "validation_failed" {
		t.Fatalf("validation=%d %#v", status, body)
	}
	if status, body, _ = request("POST", "/api/v1/cards", map[string]string{"front": "a", "back": "b", "extra": "no"}); status != 400 || errorCode(body) != "invalid_request" {
		t.Fatalf("strict=%d %#v", status, body)
	}
	if status, body, _ = request("GET", "/api/v1/cards?folderId=broken", nil); status != 422 {
		t.Fatalf("invalid folder=%d %#v", status, body)
	}
	if status, body, _ = request("GET", "/api/v1/cards?folderId="+uuid.NewString(), nil); status != 404 || errorCode(body) != "folder_not_found" {
		t.Fatalf("missing folder=%d %#v", status, body)
	}
	if status, body, _ = request("GET", "/api/v1/cards?cursor=broken", nil); status != 400 || errorCode(body) != "invalid_cursor" {
		t.Fatalf("cursor=%d %#v", status, body)
	}
	status, body, _ = request("GET", "/api/v1/folders/tree", nil)
	if status != 200 {
		t.Fatalf("tree=%d %#v", status, body)
	}
	root, _ := body["root"].(map[string]any)
	inboxID, _ := root["id"].(string)
	if inboxID == "" || root["name"] != "Inbox" {
		t.Fatalf("root=%#v", root)
	}
	status, body, _ = request("POST", "/api/v1/folders", map[string]string{"parentId": inboxID, "name": " Topic "})
	if status != 201 {
		t.Fatalf("create folder=%d %#v", status, body)
	}
	topic, _ := body["folder"].(map[string]any)
	topicID, _ := topic["id"].(string)
	status, body, _ = request("POST", "/api/v1/folders", map[string]string{"parentId": topicID, "name": "Child"})
	if status != 201 {
		t.Fatalf("create child=%d %#v", status, body)
	}
	child, _ := body["folder"].(map[string]any)
	childID, _ := child["id"].(string)
	if status, body, _ = request("POST", "/api/v1/folders", map[string]string{"parentId": inboxID, "name": "topic"}); status != 409 || errorCode(body) != "folder_name_taken" {
		t.Fatalf("duplicate folder=%d %#v", status, body)
	}
	if status, body, _ = request("PATCH", "/api/v1/folders/"+inboxID, map[string]string{"name": "Renamed"}); status != 403 || errorCode(body) != "inbox_immutable" {
		t.Fatalf("inbox mutation=%d %#v", status, body)
	}
	if status, body, _ = request("PATCH", "/api/v1/folders/"+topicID, map[string]string{"parentId": childID}); status != 409 || errorCode(body) != "folder_cycle" {
		t.Fatalf("folder cycle=%d %#v", status, body)
	}
	status, body, _ = request("POST", "/api/v1/cards", map[string]string{"front": "folder front", "back": "folder back", "folderId": childID})
	if status != 201 {
		t.Fatalf("folder card=%d %#v", status, body)
	}
	folderCard, _ := body["card"].(map[string]any)
	folderCardID, _ := folderCard["id"].(string)
	if status, body, _ = request("GET", "/api/v1/cards?folderId="+childID, nil); status != 200 || len(body["items"].([]any)) != 1 {
		t.Fatalf("folder list=%d %#v", status, body)
	}
	if status, body, _ = request("DELETE", "/api/v1/folders/"+childID, nil); status != 409 || errorCode(body) != "folder_not_empty" {
		t.Fatalf("nonempty folder=%d %#v", status, body)
	}
	if status, body, _ = request("PATCH", "/api/v1/cards/"+folderCardID, map[string]string{"folderId": inboxID}); status != 200 {
		t.Fatalf("move card=%d %#v", status, body)
	}
	if status, _, _ = request("DELETE", "/api/v1/folders/"+childID, nil); status != 204 {
		t.Fatalf("delete child=%d", status)
	}
	if status, _, _ = request("DELETE", "/api/v1/folders/"+topicID, nil); status != 204 {
		t.Fatalf("delete topic=%d", status)
	}
	status, body, _ = request("GET", "/api/v1/study/preview?scopeType=all", nil)
	preview, _ := body["preview"].(map[string]any)
	if status != 200 || preview["available"] != float64(2) || preview["new"] != float64(2) {
		t.Fatalf("study preview=%d %#v", status, body)
	}
	status, body, _ = request("POST", "/api/v1/study/sessions", map[string]any{"scopeType": "all", "count": 10})
	session, _ := body["session"].(map[string]any)
	first, _ := session["first"].(map[string]any)
	if status != 201 || session["actualCount"] != float64(2) || first["position"] != float64(1) || first["front"] == nil {
		t.Fatalf("create study session=%d %#v", status, body)
	}
	now := time.Now().UTC()
	otherUser, otherFolder, otherCard := uuid.New(), uuid.New(), uuid.New()
	if _, err = pool.Exec(context.Background(), `INSERT INTO users(id,login,login_normalized,password_hash,created_at,updated_at) VALUES($1,'OtherUser','otheruser','hash',$2,$2)`, otherUser, now); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `INSERT INTO folders(id,user_id,name,name_normalized,is_inbox,created_at,updated_at) VALUES($1,$2,'Inbox','inbox',true,$3,$3)`, otherFolder, otherUser, now); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), `INSERT INTO cards(id,user_id,folder_id,front,back,created_at,updated_at) VALUES($1,$2,$3,'other front','other back',$4,$4)`, otherCard, otherUser, otherFolder, now); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		method string
		path   string
		body   any
	}{
		{"GET", "/api/v1/cards?folderId=" + otherFolder.String(), nil},
		{"POST", "/api/v1/cards", map[string]string{"front": "foreign target", "back": "no", "folderId": otherFolder.String()}},
		{"PATCH", "/api/v1/cards/" + cardID, map[string]string{"folderId": otherFolder.String()}},
		{"POST", "/api/v1/folders", map[string]string{"parentId": otherFolder.String(), "name": "No"}},
		{"PATCH", "/api/v1/folders/" + otherFolder.String(), map[string]string{"name": "No"}},
		{"DELETE", "/api/v1/folders/" + otherFolder.String(), nil},
		{"GET", "/api/v1/study/preview?scopeType=folder&folderId=" + otherFolder.String(), nil},
	} {
		if status, body, _ = request(test.method, test.path, test.body); status != 404 || errorCode(body) != "folder_not_found" {
			t.Fatalf("foreign folder %s %s=%d %#v", test.method, test.path, status, body)
		}
	}
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		var payload any
		if method == "PATCH" {
			payload = map[string]string{"front": "stolen"}
		}
		if status, body, _ = request(method, "/api/v1/cards/"+otherCard.String(), payload); status != 404 || errorCode(body) != "card_not_found" {
			t.Fatalf("tenant %s=%d %#v", method, status, body)
		}
	}
	if status, _, _ = request("DELETE", "/api/v1/cards/"+cardID, nil); status != 204 {
		t.Fatalf("delete=%d", status)
	}
	if status, body, _ = request("GET", "/api/v1/cards/"+cardID, nil); status != 404 || errorCode(body) != "card_not_found" {
		t.Fatalf("deleted=%d %#v", status, body)
	}
	badOrigin, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/logout", nil)
	badOrigin.Header.Set("Origin", "https://evil.test")
	response, err := client.Do(badOrigin)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatalf("csrf=%d", response.StatusCode)
	}
	assertAuthHeaders(t, response.Header)
	for attempt := 1; attempt <= 6; attempt++ {
		status, body, headers := request("POST", "/api/v1/auth/login", map[string]string{"login": "LimitedUser", "password": "wrong password"})
		if attempt < 6 && (status != 401 || errorCode(body) != "invalid_credentials") {
			t.Fatalf("attempt %d=%d %#v", attempt, status, body)
		}
		if attempt == 6 && (status != 429 || errorCode(body) != "login_rate_limited" || headers.Get("Retry-After") == "") {
			t.Fatalf("limited=%d %#v retry=%q", status, body, headers.Get("Retry-After"))
		}
	}
	logText := logs.String()
	for _, secret := range []string{"exact password", "wrong password", passwordHash, rawToken, "\"password\"", "flashka_session=", "private front", "private\\nback", "changed front", "folder front", "folder back", "foreign target", "other front", "other back"} {
		if secret != "" && strings.Contains(logText, secret) {
			t.Fatalf("secret leaked in logs: %q", secret)
		}
	}
	assertAccessLogs(t, logText)
}

func errorCode(body map[string]any) string {
	envelope, _ := body["error"].(map[string]any)
	code, _ := envelope["code"].(string)
	return code
}

func assertAuthHeaders(t *testing.T, headers http.Header) {
	t.Helper()
	assertSecurityHeaders(t, headers)
	if headers.Get("Cache-Control") != "no-store" {
		t.Fatal("auth response may be cached")
	}
}

func assertSecurityHeaders(t *testing.T, headers http.Header) {
	t.Helper()
	for _, name := range []string{"X-Request-ID", "X-Content-Type-Options", "Referrer-Policy", "Content-Security-Policy", "Permissions-Policy"} {
		if headers.Get(name) == "" {
			t.Fatalf("missing %s", name)
		}
	}
}

func mustURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func assertAccessLogs(t *testing.T, text string) {
	t.Helper()
	foundSuccess, foundError := false, false
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		entry := map[string]any{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["msg"] != "http request" {
			continue
		}
		for _, field := range []string{"requestId", "method", "route", "status", "durationMs"} {
			if _, ok := entry[field]; !ok {
				t.Fatalf("log lacks %s: %s", field, line)
			}
		}
		status, _ := entry["status"].(float64)
		if status == 201 {
			if entry["userId"] == nil {
				t.Fatalf("successful auth log lacks userId: %s", line)
			}
			foundSuccess = true
		}
		if status == 401 {
			foundError = true
		}
	}
	if !foundSuccess || !foundError {
		t.Fatalf("missing success/error access logs: %s", text)
	}
}
