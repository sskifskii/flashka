package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

type PasswordHasher struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	Reader      io.Reader
}

func (h PasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, 16)
	reader := h.Reader
	if reader == nil {
		reader = rand.Reader
	}
	if _, err := io.ReadFull(reader, salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, h.Iterations, h.Memory, h.Parallelism, 32)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", h.Memory, h.Iterations, h.Parallelism, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

func (h PasswordHasher) Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, errors.New("malformed password hash")
	}
	var memory, iterations uint64
	var parallelism uint64
	for _, p := range strings.Split(parts[3], ",") {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			return false, errors.New("malformed parameters")
		}
		v, e := strconv.ParseUint(kv[1], 10, 32)
		if e != nil {
			return false, errors.New("malformed parameters")
		}
		switch kv[0] {
		case "m":
			memory = v
		case "t":
			iterations = v
		case "p":
			parallelism = v
		default:
			return false, errors.New("malformed parameters")
		}
	}
	if memory == 0 || iterations == 0 || parallelism == 0 || parallelism > 255 {
		return false, errors.New("malformed parameters")
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return false, errors.New("malformed salt")
	}
	expected, err := b64.DecodeString(parts[5])
	if err != nil || len(expected) != 32 {
		return false, errors.New("malformed key")
	}
	actual := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(memory), uint8(parallelism), uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}
