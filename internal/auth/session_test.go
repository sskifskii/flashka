package auth

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestToken(t *testing.T) {
	src := TokenSource{Reader: bytes.NewReader(make([]byte, 32))}
	tok, h, e := src.New()
	if e != nil {
		t.Fatal(e)
	}
	if len(tok) != 43 || len(h) != 32 {
		t.Fatalf("lengths %d %d", len(tok), len(h))
	}
	h2, e := HashToken(tok)
	if e != nil || !bytes.Equal(h, h2) {
		t.Fatal("hash mismatch")
	}
	raw, _ := base64.RawURLEncoding.DecodeString(tok)
	if bytes.Equal(raw, h) {
		t.Fatal("raw token stored")
	}
}
func TestTokenShortRead(t *testing.T) {
	if _, _, e := (TokenSource{Reader: bytes.NewReader([]byte{1})}).New(); e == nil {
		t.Fatal("short read accepted")
	}
}
