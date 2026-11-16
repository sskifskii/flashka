package cards

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNormalizeSideUnicodeBoundaries(t *testing.T) {
	for _, tt := range []struct{ name, in, want, err string }{
		{"empty", "\u00a0\u2003", "", "Поле обязательно"},
		{"internal", "  a \n b  ", "a \n b", ""},
		{"astral", " 😀 ", "😀", ""},
		{"max", strings.Repeat("я", 2000), strings.Repeat("я", 2000), ""},
		{"too long", strings.Repeat("x", 2001), strings.Repeat("x", 2001), "Не больше 2000 символов"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, e := NormalizeSide(tt.in)
			if got != tt.want || e != tt.err {
				t.Fatalf("got len=%d err=%q", len([]rune(got)), e)
			}
		})
	}
}
func TestCursorRoundTripAndRejection(t *testing.T) {
	u := uuid.New()
	value, err := EncodeCursor(Cursor{CreatedAt: time.Now().UTC(), ID: uuid.New(), UserID: u})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeCursor(value, u); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct {
		value string
		user  uuid.UUID
	}{{"%%%", u}, {value, uuid.New()}, {value + base64.RawURLEncoding.EncodeToString([]byte("x")), u}} {
		if _, err = DecodeCursor(bad.value, bad.user); err != ErrInvalidCursor {
			t.Fatalf("error=%v", err)
		}
	}
}

func TestProgressStatus(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	one, thirty := 1, 30
	past, soon, later := now.Add(-time.Second), now.Add(time.Hour), now.Add(24*time.Hour)
	for _, tt := range []struct {
		name     string
		interval *int
		due      *time.Time
		want     string
	}{
		{"new", nil, nil, "new"},
		{"due wins", &thirty, &past, "due"},
		{"learning", &one, &soon, "learning"},
		{"mastered", &thirty, &later, "mastered"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProgressStatus(tt.interval, tt.due, now); got != tt.want {
				t.Fatalf("got=%q want=%q", got, tt.want)
			}
		})
	}
}
