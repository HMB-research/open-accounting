package auth

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter implements a token bucket rate limiter per client IP
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	r        rate.Limit // requests per second
	b        int        // burst size
	cleanup  time.Duration
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a new rate limiter
// rps: requests per second allowed
// burst: maximum burst size
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		r:        rate.Limit(rps),
		b:        burst,
		cleanup:  3 * time.Minute,
	}

	// Start background cleanup
	go rl.cleanupVisitors()

	return rl
}

// getVisitor returns the rate limiter for the given IP
func (rl *RateLimiter) getVisitor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.r, rl.b)
		rl.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupVisitors removes stale visitor entries
func (rl *RateLimiter) cleanupVisitors() {
	for {
		time.Sleep(rl.cleanup)

		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > rl.cleanup {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the chain
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// Middleware returns a rate limiting middleware handler
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		limiter := rl.getVisitor(ip)

		// Get current token state for headers
		now := time.Now()
		reservation := limiter.ReserveN(now, 1)
		if !reservation.OK() {
			// This shouldn't happen with a properly configured limiter
			http.Error(w, `{"error":"rate_limit_exceeded","message":"Too many requests. Please try again later."}`, http.StatusTooManyRequests)
			return
		}

		delay := reservation.DelayFrom(now)
		if delay > 0 {
			// We need to wait, which means we've exceeded the rate
			reservation.CancelAt(now)

			// Calculate when the limiter will have tokens again
			retryAfter := int(delay.Seconds()) + 1
			if retryAfter < 1 {
				retryAfter = 1
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.b))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(now.Add(delay).Unix(), 10))
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate_limit_exceeded","message":"Too many requests. Please try again later.","retry_after":` + strconv.Itoa(retryAfter) + `}`))
			return
		}

		// Add rate limit headers
		tokens := int(limiter.Tokens())
		if tokens < 0 {
			tokens = 0
		}
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.b))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(tokens))

		next.ServeHTTP(w, r)
	})
}

// DefaultRateLimiter returns a rate limiter with default settings
// 100 requests per minute with a burst of 10
func DefaultRateLimiter() *RateLimiter {
	return NewRateLimiter(100.0/60.0, 10) // ~1.67 requests/sec, burst 10
}
