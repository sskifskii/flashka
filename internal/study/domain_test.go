package study

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateScope(t *testing.T) {
	id := uuid.New()
	for _, tt := range []struct {
		name, kind string
		folder     *uuid.UUID
		field      string
	}{
		{"all", "all", nil, ""},
		{"folder", "folder", &id, ""},
		{"all with folder", "all", &id, "folderId"},
		{"folder missing id", "folder", nil, "folderId"},
		{"unknown", "other", nil, "scopeType"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, fields := ValidateScope(tt.kind, tt.folder)
			if tt.field == "" && fields != nil {
				t.Fatalf("fields=%v", fields)
			}
			if tt.field != "" && fields[tt.field] == "" {
				t.Fatalf("fields=%v", fields)
			}
		})
	}
}

func TestNextSchedule(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	three := 3
	max := 120
	tests := []struct {
		name                         string
		correct                      bool
		correctCount, incorrectCount int
		streak                       int
		interval                     *int
		want                         Schedule
	}{
		{"new correct", true, 0, 0, 0, nil, Schedule{1, 0, 1, 1, now.AddDate(0, 0, 1)}},
		{"new incorrect", false, 0, 0, 0, nil, Schedule{0, 1, 0, 1, now.AddDate(0, 0, 1)}},
		{"advance", true, 4, 2, 2, &three, Schedule{5, 2, 3, 7, now.AddDate(0, 0, 7)}},
		{"reset", false, 4, 2, 2, &three, Schedule{4, 3, 0, 1, now.AddDate(0, 0, 1)}},
		{"cap", true, 9, 1, 5, &max, Schedule{10, 1, 6, 120, now.AddDate(0, 0, 120)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextSchedule(tt.correct, tt.correctCount, tt.incorrectCount, tt.streak, tt.interval, now)
			if got != tt.want {
				t.Fatalf("got=%#v want=%#v", got, tt.want)
			}
		})
	}
}
