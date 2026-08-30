package importdelivery

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var (
	ErrAuthenticationFailed = errors.New("bridge delivery authentication failed")
	ErrNonceReplayed        = errors.New("bridge delivery nonce was already used")
)

const requestLifetime = 5 * time.Minute

// NonceStore must durably reject a previously accepted nonce for a tenant.
// In production GORMRepository persists this public, non-secret metadata.
type NonceStore interface {
	ConsumeNonce(ctx context.Context, tenantID, nonce string, expiresAt time.Time) error
}

// SignedRequest contains only HTTP metadata and bounded raw bytes. It is
// purposefully not a browser authorization format.
type SignedRequest struct {
	Method        string
	Path          string
	TenantID      string
	Timestamp     string
	Nonce         string
	ContentSHA256 string
	Signature     string
	Body          []byte
}

type Authenticator interface {
	Authenticate(ctx context.Context, request SignedRequest) error
}

type HMACAuthenticator struct {
	secret []byte
	nonces NonceStore
	now    func() time.Time
}

func NewHMACAuthenticator(secret string, nonces NonceStore) (*HMACAuthenticator, error) {
	if len(strings.TrimSpace(secret)) < 16 || nonces == nil {
		return nil, ErrAuthenticationFailed
	}
	return &HMACAuthenticator{secret: []byte(secret), nonces: nonces, now: time.Now}, nil
}

func (a *HMACAuthenticator) Authenticate(ctx context.Context, request SignedRequest) error {
	if a == nil || len(a.secret) < 16 || a.nonces == nil || !safeID(request.TenantID) || !safeID(request.Nonce) || !isSHA256(request.ContentSHA256) || len(request.Signature) != 64 || !strings.HasSuffix(request.Timestamp, "Z") {
		return ErrAuthenticationFailed
	}
	timestamp, err := time.Parse(time.RFC3339, request.Timestamp)
	if err != nil || timestamp.Location() != time.UTC || timestamp.Before(a.currentTime().Add(-requestLifetime)) || timestamp.After(a.currentTime().Add(requestLifetime)) || sha256Hex(request.Body) != request.ContentSHA256 {
		return ErrAuthenticationFailed
	}
	canonical := strings.Join([]string{"v1", request.Method, request.Path, request.TenantID, request.Timestamp, request.Nonce, request.ContentSHA256}, "\n")
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(canonical))
	expected, err := hex.DecodeString(hex.EncodeToString(mac.Sum(nil)))
	provided, decodeErr := hex.DecodeString(request.Signature)
	if err != nil || decodeErr != nil || !hmac.Equal(expected, provided) {
		return ErrAuthenticationFailed
	}
	if err := a.nonces.ConsumeNonce(ctx, request.TenantID, request.Nonce, timestamp.Add(requestLifetime)); err != nil {
		if errors.Is(err, ErrChunkConflict) || errors.Is(err, ErrNonceReplayed) {
			return ErrNonceReplayed
		}
		return ErrAuthenticationFailed
	}
	return nil
}

func (a *HMACAuthenticator) currentTime() time.Time {
	if a != nil && a.now != nil {
		return a.now().UTC()
	}
	return time.Now().UTC()
}
