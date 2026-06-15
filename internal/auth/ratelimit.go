package auth

import (
	"net/http"
	"strconv"
	"strings"
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

// LoginAttemptLimiter tracks repeated failed login attempts per credential and client IP.
type LoginAttemptLimiter struct {
	attempts    map[string]*loginAttempt
	mu          sync.Mutex
	maxFailures int
	window      time.Duration
	lockout     time.Duration
	cleanup     time.Duration
	now         func() time.Time
}

type loginAttempt struct {
	failures     int
	firstFailure time.Time
	blockedUntil time.Time
	lastSeen     time.Time
}

// LoginAttemptResult describes the current state of a login-attempt key.
type LoginAttemptResult struct {
	Limited    bool
	RetryAfter time.Duration
	Remaining  int
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

// NewLoginAttemptLimiter creates a limiter for repeated failed login attempts.
func NewLoginAttemptLimiter(maxFailures int, window, lockout time.Duration) *LoginAttemptLimiter {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if window <= 0 {
		window = 15 * time.Minute
	}
	if lockout <= 0 {
		lockout = 15 * time.Minute
	}
	cleanup := window + lockout
	if cleanup < time.Second {
		cleanup = time.Second
	}

	limiter := &LoginAttemptLimiter{
		attempts:    make(map[string]*loginAttempt),
		maxFailures: maxFailures,
		window:      window,
		lockout:     lockout,
		cleanup:     cleanup,
		now:         time.Now,
	}
	go limiter.cleanupLoginAttempts()
	return limiter
}

// DefaultLoginAttemptLimiter returns the production failed-login limiter.
func DefaultLoginAttemptLimiter() *LoginAttemptLimiter {
	return NewLoginAttemptLimiter(5, 15*time.Minute, 15*time.Minute)
}

// Check reports whether a credential/IP pair is currently blocked.
func (l *LoginAttemptLimiter) Check(email, ip string) LoginAttemptResult {
	if l == nil {
		return LoginAttemptResult{}
	}

	key := loginAttemptKey(email, ip)
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, ok := l.attempts[key]
	if !ok {
		return LoginAttemptResult{Remaining: l.maxFailures}
	}
	attempt.lastSeen = now
	if !attempt.blockedUntil.IsZero() && attempt.blockedUntil.After(now) {
		return LoginAttemptResult{
			Limited:    true,
			RetryAfter: attempt.blockedUntil.Sub(now),
			Remaining:  0,
		}
	}
	if now.Sub(attempt.firstFailure) > l.window {
		delete(l.attempts, key)
		return LoginAttemptResult{Remaining: l.maxFailures}
	}
	remaining := l.maxFailures - attempt.failures
	if remaining < 0 {
		remaining = 0
	}
	return LoginAttemptResult{Remaining: remaining}
}

// RecordFailure records one failed login attempt and reports whether the key is now blocked.
func (l *LoginAttemptLimiter) RecordFailure(email, ip string) LoginAttemptResult {
	if l == nil {
		return LoginAttemptResult{}
	}

	key := loginAttemptKey(email, ip)
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, ok := l.attempts[key]
	if !ok || now.Sub(attempt.firstFailure) > l.window {
		attempt = &loginAttempt{firstFailure: now}
		l.attempts[key] = attempt
	}
	attempt.lastSeen = now
	if !attempt.blockedUntil.IsZero() && attempt.blockedUntil.After(now) {
		return LoginAttemptResult{
			Limited:    true,
			RetryAfter: attempt.blockedUntil.Sub(now),
			Remaining:  0,
		}
	}

	attempt.failures++
	remaining := l.maxFailures - attempt.failures
	if remaining < 0 {
		remaining = 0
	}
	if attempt.failures > l.maxFailures {
		attempt.blockedUntil = now.Add(l.lockout)
		return LoginAttemptResult{
			Limited:    true,
			RetryAfter: l.lockout,
			Remaining:  0,
		}
	}
	return LoginAttemptResult{Remaining: remaining}
}

// Reset clears failed attempts for a credential/IP pair after a successful login.
func (l *LoginAttemptLimiter) Reset(email, ip string) {
	if l == nil {
		return
	}

	l.mu.Lock()
	delete(l.attempts, loginAttemptKey(email, ip))
	l.mu.Unlock()
}

func (l *LoginAttemptLimiter) cleanupLoginAttempts() {
	for {
		time.Sleep(l.cleanup)
		now := l.now()

		l.mu.Lock()
		for key, attempt := range l.attempts {
			if now.Sub(attempt.lastSeen) > l.window+l.lockout {
				delete(l.attempts, key)
			}
		}
		l.mu.Unlock()
	}
}

func loginAttemptKey(email, ip string) string {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		normalizedEmail = "<blank>"
	}
	normalizedIP := strings.TrimSpace(ip)
	if normalizedIP == "" {
		normalizedIP = "<unknown>"
	}
	return normalizedEmail + "|" + normalizedIP
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

// ClientIP extracts the client IP from the request.
func ClientIP(r *http.Request) string {
	return getClientIP(r)
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
