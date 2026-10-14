package auth

import (
	"bytes"
	"testing"
)

func TestValidationBoundaries(t *testing.T) {
	tests := []struct {
		login, password string
		ok              bool
	}{{"Ab3", "12345678", true}, {"A_b-c", "       x", true}, {"ab", "12345678", false}, {"_abc", "12345678", false}, {"абв", "12345678", false}, {"abc", "1234567", false}, {"abc", "1234567😀", true}, {"abc", "123456e\u0301", true}}
	for _, tt := range tests {
		if got := ValidateCredentials(tt.login, tt.password) == nil; got != tt.ok {
			t.Errorf("%q/%q got %v", tt.login, tt.password, got)
		}
	}
}
func TestPasswordHash(t *testing.T) {
	h := PasswordHasher{Memory: 19456, Iterations: 2, Parallelism: 1}
	p := "  pass😀word  "
	enc, e := h.Hash(p)
	if e != nil {
		t.Fatal(e)
	}
	if enc == p {
		t.Fatal("plaintext stored")
	}
	ok, e := h.Verify(p, enc)
	if e != nil || !ok {
		t.Fatalf("verify: %v %v", ok, e)
	}
	ok, e = h.Verify("wrong", enc)
	if e != nil || ok {
		t.Fatalf("wrong: %v %v", ok, e)
	}
	if _, e = h.Verify(p, "broken"); e == nil {
		t.Fatal("malformed accepted")
	}
}

func TestPasswordHashRejectsShortRandomRead(t *testing.T) {
	h := PasswordHasher{Memory: 19456, Iterations: 2, Parallelism: 1, Reader: bytes.NewReader([]byte{1})}
	if _, err := h.Hash("password"); err == nil {
		t.Fatal("short random read accepted")
	}
}
