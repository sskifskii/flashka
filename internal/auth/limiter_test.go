package auth

import (
	"fmt"
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time { return f.now }
func TestLimiter(t *testing.T) {
	c := &fakeClock{time.Unix(0, 0)}
	l := NewLoginLimiter(c)
	for i := 0; i < 5; i++ {
		ok, _ := l.Allow("k")
		if !ok {
			t.Fatal("blocked early")
		}
		l.Failure("k")
	}
	if ok, r := l.Allow("k"); ok || r != 900 {
		t.Fatalf("expected block: %v %d", ok, r)
	}
	c.now = c.now.Add(15 * time.Minute)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("not reset")
	}
}

func TestLimiterBoundsRandomFailedKeys(t *testing.T) {
	c := &fakeClock{time.Unix(0, 0)}
	l := NewLoginLimiter(c)
	l.MaxEntries = 10
	l.SweepEvery = 3
	for i := 0; i < 100; i++ {
		l.Failure(fmt.Sprintf("key-%d", i))
	}
	if len(l.entries) > 10 {
		t.Fatalf("entries=%d", len(l.entries))
	}
}
func TestLimiterSweepsExpiredEntries(t *testing.T) {
	c := &fakeClock{time.Unix(0, 0)}
	l := NewLoginLimiter(c)
	l.SweepEvery = 1
	l.Failure("old")
	c.now = c.now.Add(l.Window)
	l.Failure("new")
	if _, ok := l.entries["old"]; ok {
		t.Fatal("expired entry retained")
	}
}
