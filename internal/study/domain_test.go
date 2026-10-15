package study

import (
	"testing"

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
