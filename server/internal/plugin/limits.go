package plugin

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// routeLimits is one bounded sweep loop for every plugin route budget. A
// per-route ratelimit.Limiter would start one goroutine per route; this keeps
// the host at O(1) goroutines.
type routeLimits struct {
	mu   sync.Mutex
	hits map[string][]int64
	stop chan struct{}
	once sync.Once
	done chan struct{}
}

func newRouteLimits() *routeLimits {
	l := &routeLimits{hits: map[string][]int64{}, stop: make(chan struct{}), done: make(chan struct{})}
	go l.sweep()
	return l
}

func (l *routeLimits) close() {
	l.once.Do(func() { close(l.stop) })
	<-l.done
}

func (l *routeLimits) sweep() {
	defer close(l.done)
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-l.stop:
			return
		case now := <-t.C:
			cut := now.Add(-time.Hour).UnixMilli()
			l.mu.Lock()
			for k, hits := range l.hits {
				keep := hits[:0]
				for _, h := range hits {
					if h > cut {
						keep = append(keep, h)
					}
				}
				if len(keep) == 0 {
					delete(l.hits, k)
				} else {
					l.hits[k] = keep
				}
			}
			l.mu.Unlock()
		}
	}
}

func (l *routeLimits) allow(key string, max int, window time.Duration) bool {
	if l == nil || max <= 0 || window <= 0 {
		return true
	}
	now := time.Now().UnixMilli()
	cut := now - window.Milliseconds()
	l.mu.Lock()
	defer l.mu.Unlock()
	hits := l.hits[key]
	keep := hits[:0]
	for _, h := range hits {
		if h > cut {
			keep = append(keep, h)
		}
	}
	if len(keep) >= max {
		l.hits[key] = keep
		return false
	}
	l.hits[key] = append(keep, now)
	return true
}

// parseRateLimit parses "60/m", "10/s", "100/h" into max + window.
func parseRateLimit(s string) (int, time.Duration, bool) {
	parts := strings.SplitN(strings.TrimSpace(s), "/", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil || n <= 0 || n > 1_000_000 {
		return 0, 0, false
	}
	switch parts[1] {
	case "s":
		return n, time.Second, true
	case "m":
		return n, time.Minute, true
	case "h":
		return n, time.Hour, true
	}
	return 0, 0, false
}
