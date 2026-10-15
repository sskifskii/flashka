package study

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrFolderNotFound   = errors.New("folder not found")
	ErrEmptyScope       = errors.New("study scope is empty")
	ErrSessionNotFound  = errors.New("study session not found")
	ErrNoActiveSession  = errors.New("active study session not found")
	ErrPositionConflict = errors.New("study position is not current")
	ErrNotRevealed      = errors.New("study position is not revealed")
	ErrAnswerConflict   = errors.New("study position already has another answer")
	ErrSessionInactive  = errors.New("study session is not active")
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

type CurrentPosition struct {
	Position int       `json:"position"`
	CardID   uuid.UUID `json:"cardId"`
	Front    string    `json:"front"`
	Back     *string   `json:"back"`
	Revealed bool      `json:"revealed"`
}

type SessionState struct {
	ID            uuid.UUID        `json:"id"`
	ActualCount   int              `json:"actualCount"`
	AnsweredCount int              `json:"answeredCount"`
	Status        string           `json:"status"`
	Current       *CurrentPosition `json:"current"`
}

type Schedule struct {
	CorrectCount   int
	IncorrectCount int
	CorrectStreak  int
	IntervalDays   int
	DueAt          time.Time
}

var successIntervals = [...]int{1, 3, 7, 14, 30, 60, 120}

func NextSchedule(correct bool, correctCount, incorrectCount, streak int, interval *int, now time.Time) Schedule {
	next := 1
	if correct && interval != nil {
		next = successIntervals[len(successIntervals)-1]
		for _, days := range successIntervals {
			if days > *interval {
				next = days
				break
			}
		}
	}
	if correct {
		correctCount++
		streak++
	} else {
		incorrectCount++
		streak = 0
	}
	return Schedule{CorrectCount: correctCount, IncorrectCount: incorrectCount, CorrectStreak: streak, IntervalDays: next, DueAt: now.AddDate(0, 0, next)}
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
