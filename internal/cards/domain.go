package cards

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("card not found")
var ErrInvalidCursor = errors.New("invalid cursor")
var ErrFolderNotFound = errors.New("folder not found")

type FieldErrors map[string]string

type Card struct {
	ID             uuid.UUID  `json:"id"`
	FolderID       uuid.UUID  `json:"folderId"`
	Front          string     `json:"front"`
	Back           string     `json:"back"`
	CorrectCount   int        `json:"correctCount"`
	IncorrectCount int        `json:"incorrectCount"`
	CorrectStreak  int        `json:"correctStreak"`
	IntervalDays   *int       `json:"intervalDays"`
	LastReviewedAt *time.Time `json:"lastReviewedAt"`
	DueAt          *time.Time `json:"dueAt"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type Summary struct {
	ID        uuid.UUID `json:"id"`
	Front     string    `json:"front"`
	Back      string    `json:"back"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func ProgressStatus(intervalDays *int, dueAt *time.Time, now time.Time) string {
	if dueAt == nil {
		return "new"
	}
	if !dueAt.After(now) {
		return "due"
	}
	if intervalDays != nil && *intervalDays >= 30 {
		return "mastered"
	}
	return "learning"
}

func NormalizeSide(value string) (string, string) {
	value = strings.TrimFunc(value, unicode.IsSpace)
	switch n := utf8.RuneCountInString(value); {
	case n == 0:
		return value, "Поле обязательно"
	case n > 2000:
		return value, "Не больше 2000 символов"
	default:
		return value, ""
	}
}

type Cursor struct {
	Version   int       `json:"v"`
	CreatedAt time.Time `json:"createdAt"`
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"userId"`
}

func EncodeCursor(c Cursor) (string, error) {
	c.Version = 1
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func DecodeCursor(value string, userID uuid.UUID) (*Cursor, error) {
	if value == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var c Cursor
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil || c.Version != 1 || c.ID == uuid.Nil || c.UserID != userID || c.CreatedAt.IsZero() {
		return nil, ErrInvalidCursor
	}
	if err = d.Decode(&struct{}{}); err != io.EOF {
		return nil, ErrInvalidCursor
	}
	return &c, nil
}
