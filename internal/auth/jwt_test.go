package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenService(t *testing.T) {
	service := NewTokenService("test-secret", 15*time.Minute, 7*24*time.Hour)

	assert.NotNil(t, service)
	assert.Equal(t, []byte("test-secret"), service.secretKey)
	assert.Equal(t, 15*time.Minute, service.accessExpiry)
	assert.Equal(t, 7*24*time.Hour, service.refreshExpiry)
}

func TestGenerateAccessToken(t *testing.T) {
	service := NewTokenService("test-secret", 15*time.Minute, 7*24*time.Hour)

	tests := []struct {
		name     string
		userID   string
		email    string
		tenantID string
		role     string
	}{
		{
			name:     "with all fields",
			userID:   "user-123",
			email:    "test@example.com",
			tenantID: "tenant-456",
			role:     "admin",
		},
		{
			name:     "without tenant",
			userID:   "user-123",
			email:    "test@example.com",
			tenantID: "",
			role:     "",
		},
		{
			name:     "with only required fields",
			userID:   "user-789",
			email:    "another@example.com",
			tenantID: "",
			role:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := service.GenerateAccessToken(tt.userID, tt.email, tt.tenantID, tt.role)

			require.NoError(t, err)
			assert.NotEmpty(t, token)

			// Verify the token is valid
			claims, err := service.ValidateAccessToken(token)
			require.NoError(t, err)
			assert.Equal(t, tt.userID, claims.UserID)
			assert.Equal(t, tt.email, claims.Email)
			assert.Equal(t, tt.tenantID, claims.TenantID)
			assert.Equal(t, tt.role, claims.Role)
		})
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	service := NewTokenService("test-secret", 15*time.Minute, 7*24*time.Hour)

	userID := "user-123"
	token, err := service.GenerateRefreshToken(userID)

	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify the token is valid
	extractedUserID, err := service.ValidateRefreshToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, extractedUserID)
}

func TestValidateAccessToken(t *testing.T) {
	service := NewTokenService("test-secret", 15*time.Minute, 7*24*time.Hour)

	t.Run("valid token", func(t *testing.T) {
		token, _ := service.GenerateAccessToken("user-123", "test@example.com", "tenant-456", "admin")

		claims, err := service.ValidateAccessToken(token)

		require.NoError(t, err)
		assert.Equal(t, "user-123", claims.UserID)
		assert.Equal(t, "test@example.com", claims.Email)
		assert.Equal(t, "tenant-456", claims.TenantID)
		assert.Equal(t, "admin", claims.Role)
	})

	t.Run("invalid token format", func(t *testing.T) {
		_, err := service.ValidateAccessToken("not-a-valid-token")

		assert.Error(t, err)
	})

	t.Run("wrong secret", func(t *testing.T) {
		otherService := NewTokenService("other-secret", 15*time.Minute, 7*24*time.Hour)
		token, _ := otherService.GenerateAccessToken("user-123", "test@example.com", "", "")

		_, err := service.ValidateAccessToken(token)

		assert.Error(t, err)
	})

	t.Run("expired token", func(t *testing.T) {
		expiredService := NewTokenService("test-secret", -1*time.Hour, 7*24*time.Hour)
		token, _ := expiredService.GenerateAccessToken("user-123", "test@example.com", "", "")

		_, err := service.ValidateAccessToken(token)

		assert.Error(t, err)
	})
}

func TestValidateRefreshToken(t *testing.T) {
	service := NewTokenService("test-secret", 15*time.Minute, 7*24*time.Hour)

	t.Run("valid token", func(t *testing.T) {
		token, _ := service.GenerateRefreshToken("user-123")

		userID, err := service.ValidateRefreshToken(token)

		require.NoError(t, err)
		assert.Equal(t, "user-123", userID)
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := service.ValidateRefreshToken("invalid-token")

		assert.Error(t, err)
	})

	t.Run("wrong secret", func(t *testing.T) {
		otherService := NewTokenService("other-secret", 15*time.Minute, 7*24*time.Hour)
		token, _ := otherService.GenerateRefreshToken("user-123")

		_, err := service.ValidateRefreshToken(token)

		assert.Error(t, err)
	})
}

func TestGetClaims(t *testing.T) {
	t.Run("with claims in context", func(t *testing.T) {
		claims := &Claims{
			UserID:   "user-123",
			Email:    "test@example.com",
			TenantID: "tenant-456",
			Role:     "admin",
		}
		ctx := context.WithValue(context.Background(), ClaimsContextKey, claims)

		result, ok := GetClaims(ctx)

		assert.True(t, ok)
		assert.Equal(t, claims, result)
	})

	t.Run("without claims in context", func(t *testing.T) {
		ctx := context.Background()

		result, ok := GetClaims(ctx)

		assert.False(t, ok)
		assert.Nil(t, result)
	})
}

func TestMiddleware(t *testing.T) {
	service := NewTokenService("test-secret", 15*time.Minute, 7*24*time.Hour)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(claims.UserID))
	})

	middleware := service.Middleware(handler)

	t.Run("valid token", func(t *testing.T) {
		token, _ := service.GenerateAccessToken("user-123", "test@example.com", "", "")

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "user-123", w.Body.String())
	})

	t.Run("missing authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid authorization format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("case insensitive bearer", func(t *testing.T) {
		token, _ := service.GenerateAccessToken("user-123", "test@example.com", "", "")

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "bearer "+token)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestRequireTenant(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RequireTenant(handler)

	t.Run("with tenant", func(t *testing.T) {
		claims := &Claims{
			UserID:   "user-123",
			TenantID: "tenant-456",
		}
		ctx := context.WithValue(context.Background(), ClaimsContextKey, claims)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("without tenant", func(t *testing.T) {
		claims := &Claims{
			UserID:   "user-123",
			TenantID: "",
		}
		ctx := context.WithValue(context.Background(), ClaimsContextKey, claims)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("without claims", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestRequireRole(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("allowed role", func(t *testing.T) {
		middleware := RequireRole("admin", "owner")(handler)
		claims := &Claims{
			UserID: "user-123",
			Role:   "admin",
		}
		ctx := context.WithValue(context.Background(), ClaimsContextKey, claims)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("forbidden role", func(t *testing.T) {
		middleware := RequireRole("admin", "owner")(handler)
		claims := &Claims{
			UserID: "user-123",
			Role:   "member",
		}
		ctx := context.WithValue(context.Background(), ClaimsContextKey, claims)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("without claims", func(t *testing.T) {
		middleware := RequireRole("admin")(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
