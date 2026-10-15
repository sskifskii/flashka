package folders

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeName(t *testing.T) {
	name, normalized, message := NormalizeName("\u00a0TeMa\u2003")
	if message != "" || name != "TeMa" || normalized != "tema" {
		t.Fatalf("name=%q normalized=%q message=%q", name, normalized, message)
	}
	if _, _, message = NormalizeName(" \n "); message == "" {
		t.Fatal("whitespace-only name accepted")
	}
	if _, _, message = NormalizeName(strings.Repeat("🙂", 81)); message == "" {
		t.Fatal("81 code points accepted")
	}
}

func TestBuildTreeSortsAndTotals(t *testing.T) {
	root, a, b, nested := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	tree, err := BuildTree([]Record{
		{ID: b, ParentID: &root, Name: "beta", DirectCardCount: 2},
		{ID: root, Name: "Inbox", IsInbox: true, DirectCardCount: 1},
		{ID: nested, ParentID: &a, Name: "nested", DirectCardCount: 4},
		{ID: a, ParentID: &root, Name: "Alpha", DirectCardCount: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tree.TotalCardCount != 10 || len(tree.Children) != 2 || tree.Children[0].ID != a || tree.Children[0].TotalCardCount != 7 {
		t.Fatalf("tree=%#v", tree)
	}
}

func TestTreeRelationshipsAndDepth(t *testing.T) {
	root, parent, child, leaf := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	records := []Record{
		{ID: root, IsInbox: true},
		{ID: parent, ParentID: &root},
		{ID: child, ParentID: &parent},
		{ID: leaf, ParentID: &child},
	}
	if got, ok := depth(records, leaf); !ok || got != 3 {
		t.Fatalf("depth=%d ok=%v", got, ok)
	}
	if !isDescendant(records, parent, leaf) || isDescendant(records, leaf, parent) {
		t.Fatal("descendant relationship is incorrect")
	}
	if got := subtreeHeight(records, parent); got != 2 {
		t.Fatalf("height=%d", got)
	}
}
