package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/skif/flashka/internal/auth"
	"github.com/skif/flashka/internal/platform/config"
)

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

func testHandler() http.Handler {
	cfg := config.Config{Environment: "development", CookieName: "session", SessionTTL: time.Hour, TrustedOrigin: "https://flashka.test"}
	service := auth.Service{Clock: fixedClock{time.Unix(1_000, 0)}, TTL: time.Hour}
	return New(cfg, nil, service, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCSRFFullRejectionMatrix(t *testing.T) {
	tests := []struct {
		name, origin, site string
		status             int
	}{{"missing", "", "", 403}, {"matching origin", "https://flashka.test", "", 422}, {"mismatch", "https://evil.test", "same-origin", 403}, {"same origin", "", "same-origin", 422}, {"none without origin", "", "none", 422}, {"origin conflicts with none", "https://flashka.test", "none", 403}, {"same site", "https://flashka.test", "same-site", 403}, {"cross site", "https://flashka.test", "cross-site", 403}, {"unknown", "https://flashka.test", "future", 403}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.site != "" {
				req.Header.Set("Sec-Fetch-Site", tt.site)
			}
			rec := httptest.NewRecorder()
			testHandler().ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
			}
			if rec.Header().Get("X-Request-ID") == "" {
				t.Fatal("request id missing")
			}
			if rec.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("permissive CORS emitted")
			}
		})
	}
}

func TestSecurityAndHealthHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	testHandler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	for _, name := range []string{"X-Request-ID", "X-Content-Type-Options", "Referrer-Policy", "Permissions-Policy", "Content-Security-Policy"} {
		if rec.Header().Get(name) == "" {
			t.Errorf("missing %s", name)
		}
	}
}

func TestProductionEnablesHSTS(t *testing.T) {
	cfg := config.Config{Environment: "production", CookieName: "session", SessionTTL: time.Hour, TrustedOrigin: "https://flashka.test"}
	service := auth.Service{Clock: fixedClock{time.Unix(1_000, 0)}, TTL: time.Hour}
	handler := New(cfg, nil, service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("production response lacks HSTS")
	}
}

func TestPrivateShellIsNotCached(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	rec := httptest.NewRecorder()
	testHandler().ServeHTTP(rec, req)
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private shell may be cached")
	}
}

func TestCookieAttributesMatchOnSetAndClear(t *testing.T) {
	s := &Server{cfg: config.Config{Environment: "production", CookieName: "session", SessionTTL: time.Hour}, auth: auth.Service{Clock: fixedClock{time.Unix(1_000, 0)}}}
	set := httptest.NewRecorder()
	s.setCookie(set, "token")
	clear := httptest.NewRecorder()
	s.clearCookie(clear)
	a, b := set.Result().Cookies()[0], clear.Result().Cookies()[0]
	if !a.HttpOnly || !a.Secure || a.SameSite != http.SameSiteLaxMode || a.Path != "/" {
		t.Fatalf("set cookie %#v", a)
	}
	if a.Name != b.Name || a.Path != b.Path || a.Secure != b.Secure || a.HttpOnly != b.HttpOnly || a.SameSite != b.SameSite {
		t.Fatalf("cookie mismatch %#v %#v", a, b)
	}
	if b.MaxAge >= 0 {
		t.Fatal("clear max-age")
	}
}

func TestPanicProducesRecoveredResponseAndAccessLog(t *testing.T) {
	var output bytes.Buffer
	s := &Server{log: slog.New(slog.NewJSONHandler(&output, nil))}
	handler := s.requestContext(s.accessLog(s.recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if rec.Code != 500 {
		t.Fatalf("status=%d", rec.Code)
	}
	found := false
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		entry := map[string]any{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["msg"] == "http request" {
			found = true
			if entry["status"] != float64(500) {
				t.Fatalf("access status=%v", entry["status"])
			}
			for _, field := range []string{"requestId", "method", "route", "durationMs"} {
				if _, ok := entry[field]; !ok {
					t.Fatalf("missing %s", field)
				}
			}
		}
	}
	if !found {
		t.Fatalf("access log missing: %s", output.String())
	}
}
