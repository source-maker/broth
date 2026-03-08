package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitConfig configures the rate limiting middleware.
type RateLimitConfig struct {
	// RPS is the requests per second allowed per IP. Default: 100.
	RPS int

	// Burst is the maximum burst size. Default: RPS * 2.
	Burst int
}

// RateLimiter manages per-IP token bucket rate limiters.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*limiterEntry
	rps      rate.Limit
	burst    int
	now      func() time.Time

	cleanupAfter time.Duration
	lastCleanup  time.Time
	cleanupEvery int
	requests     int
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a RateLimiter with the given rate and burst.
func NewRateLimiter(rps int, burst int) *RateLimiter {
	return &RateLimiter{
		limiters:     make(map[string]*limiterEntry),
		rps:          rate.Limit(rps),
		burst:        burst,
		now:          time.Now,
		cleanupAfter: 10 * time.Minute,
		cleanupEvery: 256,
	}
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	rl.requests++
	if rl.cleanupEvery > 0 && rl.requests%rl.cleanupEvery == 0 {
		rl.cleanupExpiredLocked(now)
	}

	entry, exists := rl.limiters[ip]
	if !exists {
		entry = &limiterEntry{
			limiter:  rate.NewLimiter(rl.rps, rl.burst),
			lastSeen: now,
		}
		rl.limiters[ip] = entry
		return entry.limiter
	}
	entry.lastSeen = now
	return entry.limiter
}

// Allow checks if a request from the given IP is allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	return rl.getLimiter(ip).Allow()
}

func (rl *RateLimiter) cleanupExpiredLocked(now time.Time) {
	if rl.cleanupAfter <= 0 {
		return
	}
	if !rl.lastCleanup.IsZero() && now.Sub(rl.lastCleanup) < rl.cleanupAfter {
		return
	}
	for ip, entry := range rl.limiters {
		if now.Sub(entry.lastSeen) > rl.cleanupAfter {
			delete(rl.limiters, ip)
		}
	}
	rl.lastCleanup = now
}

// RateLimit returns middleware that enforces per-IP rate limiting.
func RateLimit(cfgs ...RateLimitConfig) func(http.Handler) http.Handler {
	cfg := RateLimitConfig{RPS: 100}
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	if cfg.RPS == 0 {
		cfg.RPS = 100
	}
	burst := cfg.Burst
	if burst == 0 {
		burst = cfg.RPS * 2
	}

	limiter := NewRateLimiter(cfg.RPS, burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)
			if !limiter.Allow(ip) {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
