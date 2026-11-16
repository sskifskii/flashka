package folders

import (
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxDepth = 10

var (
	ErrNotFound       = errors.New("folder not found")
	ErrInboxImmutable = errors.New("inbox is immutable")
	ErrNameTaken      = errors.New("folder name taken")
	ErrCycle          = errors.New("folder cycle")
	ErrDepthExceeded  = errors.New("folder depth exceeded")
	ErrNotEmpty       = errors.New("folder not empty")
)

type FieldErrors map[string]string

type Folder struct {
	ID        uuid.UUID `json:"id"`
	ParentID  uuid.UUID `json:"parentId"`
	Name      string    `json:"name"`
	IsInbox   bool      `json:"isInbox"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Record struct {
	ID              uuid.UUID
	ParentID        *uuid.UUID
	Name            string
	IsInbox         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DirectCardCount int
}

type Node struct {
	ID              uuid.UUID  `json:"id"`
	ParentID        *uuid.UUID `json:"parentId"`
	Name            string     `json:"name"`
	IsInbox         bool       `json:"isInbox"`
	DirectCardCount int        `json:"directCardCount"`
	TotalCardCount  int        `json:"totalCardCount"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	Children        []*Node    `json:"children"`
}

func NormalizeName(value string) (string, string, string) {
	value = strings.TrimFunc(value, unicode.IsSpace)
	switch n := utf8.RuneCountInString(value); {
	case n == 0:
		return value, "", "Поле обязательно"
	case n > 80:
		return value, "", "Не больше 80 символов"
	default:
		return value, strings.ToLower(value), ""
	}
}

func BuildTree(records []Record) (*Node, error) {
	nodes := make(map[uuid.UUID]*Node, len(records))
	for _, record := range records {
		nodes[record.ID] = &Node{
			ID: record.ID, ParentID: record.ParentID, Name: record.Name,
			IsInbox: record.IsInbox, DirectCardCount: record.DirectCardCount,
			CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
			Children: make([]*Node, 0),
		}
	}
	var root *Node
	for _, node := range nodes {
		if node.ParentID == nil {
			if root != nil || !node.IsInbox {
				return nil, errors.New("invalid folder roots")
			}
			root = node
			continue
		}
		parent := nodes[*node.ParentID]
		if parent == nil {
			return nil, errors.New("folder parent missing")
		}
		parent.Children = append(parent.Children, node)
	}
	if root == nil {
		return nil, errors.New("inbox missing")
	}
	var total func(*Node) int
	total = func(node *Node) int {
		sort.Slice(node.Children, func(i, j int) bool {
			left, right := strings.ToLower(node.Children[i].Name), strings.ToLower(node.Children[j].Name)
			if left == right {
				return node.Children[i].ID.String() < node.Children[j].ID.String()
			}
			return left < right
		})
		node.TotalCardCount = node.DirectCardCount
		for _, child := range node.Children {
			node.TotalCardCount += total(child)
		}
		return node.TotalCardCount
	}
	total(root)
	return root, nil
}
