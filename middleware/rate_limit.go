package middleware

import (
	"net"
	"net/http"
	"sync"

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
	limiters map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

// NewRateLimiter creates a RateLimiter with the given rate and burst.
func NewRateLimiter(rps int, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rps, rl.burst)
		rl.limiters[ip] = limiter
	}
	return limiter
}

// Allow checks if a request from the given IP is allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	return rl.getLimiter(ip).Allow()
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
