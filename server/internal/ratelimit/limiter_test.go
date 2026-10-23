package ratelimit

import (
	"testing"
	"time"
)

func TestSlidingWindow(t *testing.T) {
	l := New(3, 50*time.Millisecond, 0, time.Second)
	defer l.Stop()
	for i := 0; i < 3; i++ {
		if !l.Allow("ip:1") {
			t.Fatalf("hit %d unexpectedly denied", i)
		}
	}
	if l.Allow("ip:1") {
		t.Fatal("4th hit inside window must be denied")
	}
	if !l.Allow("ip:2") {
		t.Fatal("other key must be independent")
	}
	time.Sleep(60 * time.Millisecond)
	if !l.Allow("ip:1") {
		t.Fatal("window must reopen after expiry")
	}
}

func TestFailureLockout(t *testing.T) {
	l := New(0, time.Second, 3, time.Minute)
	defer l.Stop()
	key := "id:bob"
	if l.Locked(key) {
		t.Fatal("fresh key not locked")
	}
	l.Fail(key)
	l.Fail(key)
	if l.Locked(key) {
		t.Fatal("2 fails under threshold must not lock")
	}
	l.Fail(key)
	if !l.Locked(key) {
		t.Fatal("3rd fail must lock")
	}
	if l.RetryAfter(key) <= 0 {
		t.Fatal("RetryAfter must be positive while locked")
	}
	l.Succeed(key)
	if l.Locked(key) {
		t.Fatal("Succeed must clear the lock")
	}
}

func TestDisabledLayers(t *testing.T) {
	l := New(0, time.Second, 0, time.Second)
	defer l.Stop()
	for i := 0; i < 100; i++ {
		if !l.Allow("ip:x") {
			t.Fatal("disabled window must always allow")
		}
	}
	l.Fail("id:y")
	if l.Locked("id:y") {
		t.Fatal("disabled lockout must never lock")
	}
}

func TestSweep(t *testing.T) {
	l := New(2, 30*time.Millisecond, 2, 30*time.Millisecond)
	defer l.Stop()
	l.Allow("ip:a")
	l.Fail("id:b")
	l.Fail("id:b") // locks
	l.sweep(time.Now().UnixMilli() + 10_000)
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.requests) != 0 {
		t.Fatalf("stale requests not swept: %v", l.requests)
	}
	if len(l.fails) != 0 {
		t.Fatalf("expired fails not swept: %v", l.fails)
	}
}
