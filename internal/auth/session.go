package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

type TokenSource struct{ Reader io.Reader }

func (s TokenSource) New() (string, []byte, error) {
	r := s.Reader
	if r == nil {
		r = rand.Reader
	}
	raw := make([]byte, 32)
	n, e := io.ReadFull(r, raw)
	if e != nil || n != len(raw) {
		return "", nil, errors.New("generate session token")
	}
	sum := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), sum[:], nil
}
func HashToken(encoded string) ([]byte, error) {
	raw, e := base64.RawURLEncoding.DecodeString(encoded)
	if e != nil || len(raw) != 32 {
		return nil, errors.New("invalid session token")
	}
	sum := sha256.Sum256(raw)
	return sum[:], nil
}
