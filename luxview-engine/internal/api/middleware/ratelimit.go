package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const maxVisitors = 10000

// RateLimiter implements per-IP rate limiting with a cap on tracked IPs.
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rps      rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a rate limiter with the given requests per second and burst.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rps:      rate.Limit(rps),
		burst:    burst,
	}

	// Cleanup old visitors every 3 minutes
	go func() {
		for {
			time.Sleep(3 * time.Minute)
			rl.cleanup()
		}
	}()

	return rl
}

// Middleware returns the rate limiting middleware.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		limiter := rl.getLimiter(ip)

		if !limiter.Allow() {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		// Evict oldest entries if at capacity to prevent memory exhaustion (CWE-770)
		if len(rl.visitors) >= maxVisitors {
			rl.evictOldest()
		}
		limiter := rate.NewLimiter(rl.rps, rl.burst)
		rl.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// evictOldest removes the oldest 10% of visitors. Must be called with mu held.
func (rl *RateLimiter) evictOldest() {
	oldest := time.Now()
	toEvict := len(rl.visitors) / 10
	if toEvict < 1 {
		toEvict = 1
	}

	evicted := 0
	for ip, v := range rl.visitors {
		if v.lastSeen.Before(oldest) || evicted < toEvict {
			delete(rl.visitors, ip)
			evicted++
			if evicted >= toEvict {
				break
			}
		}
	}
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for ip, v := range rl.visitors {
		if time.Since(v.lastSeen) > 5*time.Minute {
			delete(rl.visitors, ip)
		}
	}
}

// getIP extracts the client IP from the request.
// Uses the first IP from X-Forwarded-For (Traefik sets real client IP first).
func getIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Traefik puts the real client IP first, proxies append
		if i := strings.IndexByte(xff, ','); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}
