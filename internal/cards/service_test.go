package cards

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestUpdateRequiresAtLeastOneSide(t *testing.T) {
	_, fields, err := (Service{}).Update(context.Background(), uuid.New(), uuid.New(), nil, nil)
	if err != nil || fields["request"] == "" {
		t.Fatalf("fields=%#v err=%v", fields, err)
	}
}
