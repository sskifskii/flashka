package study

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrFolderNotFound = errors.New("folder not found")
	ErrEmptyScope     = errors.New("study scope is empty")
)

type Scope struct {
	Type     string
	FolderID *uuid.UUID
}

type Preview struct {
	Available int `json:"available"`
	Due       int `json:"due"`
	New       int `json:"new"`
	Mastered  int `json:"mastered"`
}

type Position struct {
	Position int       `json:"position"`
	CardID   uuid.UUID `json:"cardId"`
	Front    string    `json:"front"`
}

type Session struct {
	ID             uuid.UUID  `json:"id"`
	ScopeType      string     `json:"scopeType"`
	ScopeFolderID  *uuid.UUID `json:"scopeFolderId"`
	RequestedCount int        `json:"requestedCount"`
	ActualCount    int        `json:"actualCount"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	First          Position   `json:"first"`
}

type FieldErrors map[string]string

func ValidateScope(scopeType string, folderID *uuid.UUID) (Scope, FieldErrors) {
	switch scopeType {
	case "all":
		if folderID != nil {
			return Scope{}, FieldErrors{"folderId": "Не указывайте папку для всех карточек"}
		}
	case "folder":
		if folderID == nil {
			return Scope{}, FieldErrors{"folderId": "Выберите папку"}
		}
	default:
		return Scope{}, FieldErrors{"scopeType": "Выберите все карточки или папку"}
	}
	return Scope{Type: scopeType, FolderID: folderID}, nil
}
