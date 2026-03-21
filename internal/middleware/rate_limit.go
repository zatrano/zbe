package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/zatrano/zbe/pkg/utils"
)

// ipEntry tracks request count and window start for a single IP.
type ipEntry struct {
	count     int
	windowEnd time.Time
	mu        sync.Mutex
}

// ipStore is an in-memory store of IP rate-limit entries.
type ipStore struct {
	entries map[string]*ipEntry
	mu      sync.RWMutex
}

func newIPStore() *ipStore {
	s := &ipStore{entries: make(map[string]*ipEntry)}
	go s.cleanup()
	return s
}

func (s *ipStore) allow(ip string, limit int, window time.Duration) bool {
	s.mu.RLock()
	e, ok := s.entries[ip]
	s.mu.RUnlock()

	now := time.Now()

	if !ok {
		s.mu.Lock()
		e = &ipEntry{count: 0, windowEnd: now.Add(window)}
		s.entries[ip] = e
		s.mu.Unlock()
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if now.After(e.windowEnd) {
		e.count = 0
		e.windowEnd = now.Add(window)
	}

	e.count++
	return e.count <= limit
}

// cleanup removes expired entries every minute to prevent memory leaks.
func (s *ipStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for ip, e := range s.entries {
			e.mu.Lock()
			expired := now.After(e.windowEnd)
			e.mu.Unlock()
			if expired {
				delete(s.entries, ip)
			}
		}
		s.mu.Unlock()
	}
}

var globalStore = newIPStore()

// RateLimit returns a middleware that limits requests per IP.
// limit: max requests; window: time window duration.
func RateLimit(limit int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()
		if !globalStore.allow(ip, limit, window) {
			c.Set("Retry-After", fmt.Sprintf("%.0f", window.Seconds()))
			return utils.RespondError(c, fiber.StatusTooManyRequests,
				"too many requests, please slow down")
		}
		return c.Next()
	}
}

// StrictRateLimit is a tighter limit for sensitive endpoints (auth, password reset).
func StrictRateLimit() fiber.Handler {
	return RateLimit(10, time.Minute)
}

// AuthRateLimit applies a 5-per-minute limit for login/register endpoints.
func AuthRateLimit() fiber.Handler {
	return RateLimit(5, time.Minute)
}
