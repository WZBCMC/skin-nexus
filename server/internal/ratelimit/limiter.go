// Package ratelimit provides the auth-endpoint protection layer (P1
// ygg_rate_limit, chosen design 2026-10-23: sliding-window per IP +
// consecutive-failure lockout per identifier — the Laravel/Cloudflare-style
// mainstream, in-memory with bounded sweep; single-instance deployment model,
// no external dependency).
//
// Two independent key spaces (callers prefix keys):
//
//	ip:1.2.3.4 — request sliding window (Allow): catches resource abuse;
//	id:bob     — consecutive-failure lockout (Fail/Succeed/Locked): catches
//	             targeted password guessing even from rotating IPs.
//
// Failures do NOT lock IPs (shared NAT would punish everyone); window hits
// do NOT lock identifiers. Thresholds come from the settings panel at boot.
package ratelimit

import (
	"sync"
	"time"
)

type failRec struct {
	count       int
	windowStart int64
	lockedUntil int64
}

// Limiter is a sliding-window + failure-lockout rate limiter. Timestamps
// are milliseconds (UnixMilli) — Seconds() truncation silently zeroed
// sub-second windows in tests.
type Limiter struct {
	mu       sync.Mutex
	requests map[string][]int64 // key → allowed hit timestamps (unix millis)
	fails    map[string]failRec
	maxReqs  int
	window   time.Duration
	failMax  int
	failWin  time.Duration
	stop     chan struct{}
}

// New builds a limiter and starts its sweeper. Non-positive maxReqs or
// failMax disables that layer (zero = off).
func New(maxReqs int, window time.Duration, failMax int, failWin time.Duration) *Limiter {
	l := &Limiter{
		requests: make(map[string][]int64),
		fails:    make(map[string]failRec),
		maxReqs:  maxReqs,
		window:   window,
		failMax:  failMax,
		failWin:  failWin,
		stop:     make(chan struct{}),
	}
	go l.sweepLoop()
	return l
}

// Allow reports whether a request may proceed for key (sliding window of
// maxReqs per window) and records the hit when allowed.
func (l *Limiter) Allow(key string) bool {
	if l.maxReqs <= 0 {
		return true
	}
	now := time.Now().UnixMilli()
	cut := now - l.window.Milliseconds()
	l.mu.Lock()
	defer l.mu.Unlock()
	hits := l.requests[key]
	keep := hits[:0]
	for _, h := range hits {
		if h > cut {
			keep = append(keep, h)
		}
	}
	if len(keep) >= l.maxReqs {
		l.requests[key] = keep
		return false
	}
	l.requests[key] = append(keep, now)
	return true
}

// Fail records one failed credential attempt for key (identifier space).
// failMax failures inside failWin lock the key until the window elapses.
func (l *Limiter) Fail(key string) {
	if l.failMax <= 0 {
		return
	}
	now := time.Now().UnixMilli()
	win := l.failWin.Milliseconds()
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.fails[key]
	if now > f.windowStart+win {
		f = failRec{windowStart: now}
	}
	f.count++
	if f.count >= l.failMax {
		f.lockedUntil = now + win
	}
	l.fails[key] = f
}

// Locked reports whether key is currently in failure lockout.
func (l *Limiter) Locked(key string) bool {
	now := time.Now().UnixMilli()
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.fails[key]
	return ok && now < f.lockedUntil
}

// Succeed clears failure state for key after a valid credential check.
func (l *Limiter) Succeed(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}

// RetryAfter returns the remaining lockout seconds (0 = not locked) —
// feeds the 429 Retry-After header.
func (l *Limiter) RetryAfter(key string) time.Duration {
	now := time.Now().UnixMilli()
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.fails[key]
	if !ok || now >= f.lockedUntil {
		return 0
	}
	return time.Duration(f.lockedUntil-now) * time.Millisecond
}

// sweepLoop drops stale request timestamps and expired failure records so
// memory stays bounded by active keys.
func (l *Limiter) sweepLoop() {
	t := time.NewTicker(l.window)
	defer t.Stop()
	for {
		select {
		case <-l.stop:
			return
		case now := <-t.C:
			l.sweep(now.UnixMilli())
		}
	}
}

func (l *Limiter) sweep(now int64) {
	cutReq := now - l.window.Milliseconds()
	cutFail := now - 2*l.failWin.Milliseconds()
	win := l.failWin.Milliseconds()
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, hits := range l.requests {
		keep := hits[:0]
		for _, h := range hits {
			if h > cutReq {
				keep = append(keep, h)
			}
		}
		if len(keep) == 0 {
			delete(l.requests, k)
		} else {
			l.requests[k] = keep
		}
	}
	for k, f := range l.fails {
		if f.lockedUntil < cutFail && now > f.windowStart+win {
			delete(l.fails, k)
		}
	}
}

// Stop terminates the sweeper (tests).
func (l *Limiter) Stop() { close(l.stop) }
