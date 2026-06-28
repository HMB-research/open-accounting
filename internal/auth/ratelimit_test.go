package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_AllowsBurst(t *testing.T) {
	rl := NewRateLimiter(10, 5) // 10 req/sec, burst 5

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Should allow 5 requests in burst
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Request %d: expected status 200, got %d", i+1, rr.Code)
		}
	}
}

func TestRateLimiter_BlocksExcessRequests(t *testing.T) {
	rl := NewRateLimiter(1, 2) // 1 req/sec, burst 2

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 2 requests should succeed (burst)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Burst request %d: expected status 200, got %d", i+1, rr.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Rate limited request: expected status 429, got %d", rr.Code)
	}

	// Check rate limit headers
	if rr.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("Missing X-RateLimit-Limit header")
	}
	if rr.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("Missing X-RateLimit-Remaining header")
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Error("Missing Retry-After header")
	}
}

func TestRateLimiterRejectsImpossibleReservation(t *testing.T) {
	rl := NewRateLimiter(1, 0)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_SeparatesClientsByIP(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 req/sec, burst 1

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First client - should succeed
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("Client 1 first request: expected status 200, got %d", rr1.Code)
	}

	// First client - second request should be rate limited
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Client 1 second request: expected status 429, got %d", rr2.Code)
	}

	// Second client - should succeed (different IP)
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.RemoteAddr = "192.168.1.2:12345"
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Errorf("Client 2 first request: expected status 200, got %d", rr3.Code)
	}
}

func TestRateLimiter_RespectsXForwardedFor(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 req/sec, burst 1

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with X-Forwarded-For
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "10.0.0.1:12345" // Proxy IP
	req1.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("First request: expected status 200, got %d", rr1.Code)
	}

	// Same client IP via X-Forwarded-For should be rate limited
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "10.0.0.1:12345"
	req2.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request: expected status 429, got %d", rr2.Code)
	}
}

func TestRateLimiter_RespectsXRealIP(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 req/sec, burst 1

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request with X-Real-IP
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "10.0.0.1:12345"
	req1.Header.Set("X-Real-IP", "203.0.113.2")
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("First request: expected status 200, got %d", rr1.Code)
	}

	// Same X-Real-IP should be rate limited
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "10.0.0.1:12345"
	req2.Header.Set("X-Real-IP", "203.0.113.2")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request: expected status 429, got %d", rr2.Code)
	}
}

func TestRateLimiter_RecoverAfterTime(t *testing.T) {
	rl := NewRateLimiter(10, 1) // 10 req/sec, burst 1

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should succeed
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("First request: expected status 200, got %d", rr1.Code)
	}

	// Immediate second request should fail
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request: expected status 429, got %d", rr2.Code)
	}

	// Wait for token to refill (100ms for 10 req/sec)
	time.Sleep(150 * time.Millisecond)

	// Third request should succeed after waiting
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Errorf("Third request after wait: expected status 200, got %d", rr3.Code)
	}
}

func TestDefaultRateLimiter(t *testing.T) {
	rl := DefaultRateLimiter()

	if rl == nil {
		t.Fatal("DefaultRateLimiter should not return nil")
	}

	if rl.b != defaultRateLimitBurst {
		t.Errorf("Expected burst of %d, got %d", defaultRateLimitBurst, rl.b)
	}

	expectedRate := defaultRateLimitRequestsPerMinute / 60.0
	if float64(rl.r) < expectedRate-0.01 || float64(rl.r) > expectedRate+0.01 {
		t.Errorf("Expected rate of ~%.2f, got %.2f", expectedRate, float64(rl.r))
	}
}

func TestDefaultRateLimiterAllowsWorkspaceLoadFanout(t *testing.T) {
	rl := DefaultRateLimiter()
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 60; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/tenant-1/dashboard", nil)
		req.RemoteAddr = "100.69.65.108:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("workspace fanout request %d: status = %d, want %d", i+1, rr.Code, http.StatusOK)
		}
	}
}

func TestLoginAttemptLimiterRecordsCredentialAwareFailures(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	limiter := &LoginAttemptLimiter{
		attempts:    make(map[string]*loginAttempt),
		maxFailures: 2,
		window:      time.Minute,
		lockout:     5 * time.Minute,
		now:         func() time.Time { return now },
	}

	result := limiter.RecordFailure("User@Example.com", "192.0.2.10")
	if result.Limited {
		t.Fatal("first failed attempt should not be limited")
	}
	if result.Remaining != 1 {
		t.Fatalf("first attempt remaining = %d, want 1", result.Remaining)
	}

	result = limiter.RecordFailure("user@example.com", "192.0.2.10")
	if result.Limited {
		t.Fatal("second failed attempt should not be limited")
	}
	if result.Remaining != 0 {
		t.Fatalf("second attempt remaining = %d, want 0", result.Remaining)
	}

	result = limiter.RecordFailure(" USER@example.com ", "192.0.2.10")
	if !result.Limited {
		t.Fatal("third failed attempt should be limited")
	}
	if result.RetryAfter != 5*time.Minute {
		t.Fatalf("retry after = %s, want 5m", result.RetryAfter)
	}

	result = limiter.Check("user@example.com", "198.51.100.2")
	if result.Limited {
		t.Fatal("same credential from a different IP should have an independent budget")
	}

	limiter.Reset("user@example.com", "192.0.2.10")
	result = limiter.Check("user@example.com", "192.0.2.10")
	if result.Limited {
		t.Fatal("reset credential/IP pair should not be limited")
	}
	if result.Remaining != 2 {
		t.Fatalf("remaining after reset = %d, want 2", result.Remaining)
	}
}

func TestLoginAttemptLimiterNilAndBlockedRecordFailureBranches(t *testing.T) {
	var nilLimiter *LoginAttemptLimiter
	if result := nilLimiter.RecordFailure("user@example.com", "192.0.2.1"); result.Limited || result.Remaining != 0 {
		t.Fatalf("nil limiter RecordFailure = %#v, want zero result", result)
	}
	nilLimiter.Reset("user@example.com", "192.0.2.1")

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	limiter := &LoginAttemptLimiter{
		attempts:    make(map[string]*loginAttempt),
		maxFailures: 2,
		window:      time.Minute,
		lockout:     5 * time.Minute,
		now:         func() time.Time { return now },
	}
	key := loginAttemptKey("blocked@example.com", "192.0.2.1")
	limiter.attempts[key] = &loginAttempt{
		firstFailure: now.Add(-30 * time.Second),
		lastSeen:     now.Add(-30 * time.Second),
		failures:     3,
		blockedUntil: now.Add(2 * time.Minute),
	}

	result := limiter.RecordFailure("blocked@example.com", "192.0.2.1")
	if !result.Limited || result.RetryAfter != 2*time.Minute || result.Remaining != 0 {
		t.Fatalf("blocked RecordFailure = %#v, want limited with 2m retry", result)
	}
}

func TestLoginAttemptLimiterConstructors(t *testing.T) {
	custom := NewLoginAttemptLimiter(3, 2*time.Minute, 4*time.Minute)
	if custom == nil {
		t.Fatal("NewLoginAttemptLimiter returned nil")
	}
	if custom.maxFailures != 3 {
		t.Fatalf("maxFailures = %d, want 3", custom.maxFailures)
	}
	if custom.window != 2*time.Minute {
		t.Fatalf("window = %s, want 2m", custom.window)
	}
	if custom.lockout != 4*time.Minute {
		t.Fatalf("lockout = %s, want 4m", custom.lockout)
	}
	if custom.cleanup != 6*time.Minute {
		t.Fatalf("cleanup = %s, want 6m", custom.cleanup)
	}

	defaulted := NewLoginAttemptLimiter(0, 0, 0)
	if defaulted.maxFailures != 5 {
		t.Fatalf("default maxFailures = %d, want 5", defaulted.maxFailures)
	}
	if defaulted.window != 15*time.Minute {
		t.Fatalf("default window = %s, want 15m", defaulted.window)
	}
	if defaulted.lockout != 15*time.Minute {
		t.Fatalf("default lockout = %s, want 15m", defaulted.lockout)
	}

	minCleanup := NewLoginAttemptLimiter(1, time.Nanosecond, time.Nanosecond)
	if minCleanup.cleanup != time.Second {
		t.Fatalf("minimum cleanup = %s, want 1s", minCleanup.cleanup)
	}

	production := DefaultLoginAttemptLimiter()
	if production == nil || production.maxFailures != 5 || production.window != 15*time.Minute || production.lockout != 15*time.Minute {
		t.Fatalf("DefaultLoginAttemptLimiter returned unexpected config: %#v", production)
	}
}

func TestLoginAttemptLimiterCheckBranches(t *testing.T) {
	var nilLimiter *LoginAttemptLimiter
	if result := nilLimiter.Check("user@example.com", "192.0.2.1"); result.Limited || result.Remaining != 0 {
		t.Fatalf("nil limiter check = %#v, want zero result", result)
	}

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	limiter := &LoginAttemptLimiter{
		attempts:    make(map[string]*loginAttempt),
		maxFailures: 2,
		window:      time.Minute,
		lockout:     5 * time.Minute,
		now:         func() time.Time { return now },
	}

	blockedKey := loginAttemptKey("blocked@example.com", "192.0.2.1")
	limiter.attempts[blockedKey] = &loginAttempt{
		firstFailure: now.Add(-30 * time.Second),
		lastSeen:     now.Add(-30 * time.Second),
		failures:     3,
		blockedUntil: now.Add(time.Minute),
	}
	result := limiter.Check("blocked@example.com", "192.0.2.1")
	if !result.Limited || result.Remaining != 0 || result.RetryAfter != time.Minute {
		t.Fatalf("blocked check = %#v, want limited with 1m retry", result)
	}

	expiredKey := loginAttemptKey("expired@example.com", "192.0.2.2")
	limiter.attempts[expiredKey] = &loginAttempt{
		firstFailure: now.Add(-2 * time.Minute),
		lastSeen:     now.Add(-2 * time.Minute),
		failures:     1,
	}
	result = limiter.Check("expired@example.com", "192.0.2.2")
	if result.Limited || result.Remaining != 2 {
		t.Fatalf("expired-window check = %#v, want reset budget", result)
	}
	if _, ok := limiter.attempts[expiredKey]; ok {
		t.Fatal("expired attempt was not deleted")
	}

	overBudgetKey := loginAttemptKey("over@example.com", "192.0.2.3")
	limiter.attempts[overBudgetKey] = &loginAttempt{
		firstFailure: now,
		lastSeen:     now,
		failures:     5,
	}
	result = limiter.Check("over@example.com", "192.0.2.3")
	if result.Limited || result.Remaining != 0 {
		t.Fatalf("over-budget check = %#v, want remaining 0 without lockout", result)
	}
}

func TestLoginAttemptKeyDefaults(t *testing.T) {
	if got := loginAttemptKey(" User@Example.COM ", " 192.0.2.20 "); got != "user@example.com|192.0.2.20" {
		t.Fatalf("loginAttemptKey normalized = %q", got)
	}
	if got := loginAttemptKey(" ", " "); got != "<blank>|<unknown>" {
		t.Fatalf("loginAttemptKey blank defaults = %q", got)
	}
}

func TestLoginAttemptLimiterCleanupRemovesStaleEntries(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	limiter := &LoginAttemptLimiter{
		attempts:    make(map[string]*loginAttempt),
		maxFailures: 2,
		window:      time.Millisecond,
		lockout:     time.Millisecond,
		cleanup:     5 * time.Millisecond,
		now:         func() time.Time { return now },
	}
	staleKey := loginAttemptKey("stale@example.com", "192.0.2.10")
	freshKey := loginAttemptKey("fresh@example.com", "192.0.2.11")
	limiter.attempts[staleKey] = &loginAttempt{firstFailure: now.Add(-time.Hour), lastSeen: now.Add(-time.Hour)}
	limiter.attempts[freshKey] = &loginAttempt{firstFailure: now, lastSeen: now}

	go limiter.cleanupLoginAttempts()

	deadline := time.After(200 * time.Millisecond)
	for {
		limiter.mu.Lock()
		_, staleExists := limiter.attempts[staleKey]
		_, freshExists := limiter.attempts[freshKey]
		limiter.mu.Unlock()
		if !staleExists && freshExists {
			return
		}
		select {
		case <-deadline:
			t.Fatal("stale login attempt was not removed by cleanup")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestRateLimiter_TokensNegativeHandled(t *testing.T) {
	// Test that negative token count is handled correctly
	// This can happen when requests come in faster than tokens refill
	rl := NewRateLimiter(0.1, 2) // Very slow rate: 0.1 req/sec, burst 2

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Use all tokens
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}

	// Wait a tiny bit so tokens might be slightly negative
	time.Sleep(10 * time.Millisecond)

	// Make another request - should be rate limited but tokens should show as 0
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should be rate limited
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", rr.Code)
	}

	// X-RateLimit-Remaining should be "0" not a negative number
	remaining := rr.Header().Get("X-RateLimit-Remaining")
	if remaining != "" && remaining != "0" {
		// If header is set, it should be 0 (not negative)
		if remaining[0] == '-' {
			t.Errorf("X-RateLimit-Remaining should not be negative, got %s", remaining)
		}
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		expected   string
	}{
		{
			name:       "RemoteAddr only",
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1:12345",
		},
		{
			name:       "X-Forwarded-For single IP",
			remoteAddr: "10.0.0.1:12345",
			xff:        "203.0.113.1",
			expected:   "203.0.113.1",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			remoteAddr: "10.0.0.1:12345",
			xff:        "203.0.113.1, 10.0.0.1",
			expected:   "203.0.113.1",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xri:        "203.0.113.2",
			expected:   "203.0.113.2",
		},
		{
			name:       "X-Forwarded-For takes precedence over X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xff:        "203.0.113.1",
			xri:        "203.0.113.2",
			expected:   "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			got := getClientIP(req)
			if got != tt.expected {
				t.Errorf("getClientIP() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestClientIPWrapper(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Real-IP", "203.0.113.9")

	if got := ClientIP(req); got != "203.0.113.9" {
		t.Fatalf("ClientIP() = %q, want %q", got, "203.0.113.9")
	}
}

func TestCleanupVisitors(t *testing.T) {
	// Create a rate limiter with very short cleanup interval
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		r:        10,
		b:        5,
		cleanup:  20 * time.Millisecond,
	}

	// Add some visitors - stale visitor is older than cleanup interval
	rl.mu.Lock()
	rl.visitors["192.168.1.1"] = &visitor{
		limiter:  nil,                            // Not needed for cleanup test
		lastSeen: time.Now().Add(-1 * time.Hour), // Stale visitor (way older than 20ms)
	}
	rl.mu.Unlock()

	// Start cleanup in background
	go rl.cleanupVisitors()

	// Wait for cleanup to run at least once
	time.Sleep(60 * time.Millisecond)

	// Check that stale visitor was removed
	rl.mu.RLock()
	_, staleExists := rl.visitors["192.168.1.1"]
	rl.mu.RUnlock()

	if staleExists {
		t.Error("Stale visitor should have been cleaned up")
	}
}

func TestRateLimiter_NegativeTokensHandledInHeaders(t *testing.T) {
	// Create rate limiter with very slow refill to ensure tokens go negative
	rl := NewRateLimiter(0.001, 1) // 0.001 req/sec, burst 1

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request succeeds
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "10.10.10.1:12345"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("First request should succeed, got %d", rr1.Code)
	}

	// Get the limiter directly and drain tokens below zero
	limiter := rl.getVisitor("10.10.10.1:12345")
	// ReserveN will make tokens negative
	limiter.ReserveN(time.Now(), 10)

	// Now make another request - tokens are negative
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "10.10.10.1:12345"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	// Should be rate limited
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", rr2.Code)
	}

	// Remaining should be 0, not negative
	remaining := rr2.Header().Get("X-RateLimit-Remaining")
	if remaining == "" {
		t.Error("X-RateLimit-Remaining header should be set")
	} else if remaining[0] == '-' {
		t.Errorf("X-RateLimit-Remaining should not be negative, got %s", remaining)
	}
}

func TestRateLimiter_NegativeTokensOnSuccessPath(t *testing.T) {
	// Test the tokens < 0 check on the success path (line 136-137)
	// This tests when a successful request is made but tokens are still slightly negative
	rl := NewRateLimiter(1000, 10) // Fast refill rate, burst 10

	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make a successful request from a new IP
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.10.10.99:12345"
	rr := httptest.NewRecorder()

	// Get the limiter and drain tokens below zero before the middleware runs
	limiter := rl.getVisitor("10.10.10.99:12345")
	// Reserve many tokens to make it negative, but then let it refill just enough to allow request
	res := limiter.ReserveN(time.Now(), 20)
	res.CancelAt(time.Now()) // Cancel to give tokens back

	handler.ServeHTTP(rr, req)

	// Check that X-RateLimit-Remaining is set and not negative
	remaining := rr.Header().Get("X-RateLimit-Remaining")
	if remaining != "" && len(remaining) > 0 && remaining[0] == '-' {
		t.Errorf("X-RateLimit-Remaining should not be negative on success, got %s", remaining)
	}
}
