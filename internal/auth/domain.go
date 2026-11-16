package auth

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

var loginPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{2,31}$`)

type User struct {
	ID    uuid.UUID `json:"id"`
	Login string    `json:"login"`
}
type FieldErrors map[string]string

func ValidateCredentials(login, password string) FieldErrors {
	err := FieldErrors{}
	if !loginPattern.MatchString(login) {
		err["login"] = "Логин должен содержать 3–32 латинских символа, цифры, _ или -"
	}
	n := utf8.RuneCountInString(password)
	if !utf8.ValidString(password) || n < 8 || n > 128 {
		err["password"] = "Пароль должен содержать 8–128 символов"
	}
	if len(err) == 0 {
		return nil
	}
	return err
}
func NormalizeLogin(login string) string { return strings.ToLower(login) }
