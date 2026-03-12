package apitoken

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HMB-research/open-accounting/internal/auth"
)

const (
	tokenPrefix = "oa_"
)

// Service manages tenant-scoped API tokens.
type Service struct {
	repo Repository
}

// NewService creates a new API token service.
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		repo: NewPostgresRepository(db),
	}
}

// NewServiceWithRepository creates a new API token service with a custom repository.
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// CreateToken creates and persists a new API token, returning the raw token once.
func (s *Service) CreateToken(ctx context.Context, userID, tenantID string, req *CreateRequest) (*CreateResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.ExpiresAt != nil && !req.ExpiresAt.After(time.Now()) {
		return nil, fmt.Errorf("expires_at must be in the future")
	}

	rawToken, prefix, tokenHash, err := generateTokenMaterial()
	if err != nil {
		return nil, err
	}

	token := &APIToken{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		UserID:      userID,
		Name:        name,
		TokenPrefix: prefix,
		ExpiresAt:   req.ExpiresAt,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.CreateToken(ctx, token, tokenHash); err != nil {
		return nil, err
	}

	return &CreateResult{
		Token:    rawToken,
		APIToken: token,
	}, nil
}

// ListTokens returns active API tokens for a user within a tenant.
func (s *Service) ListTokens(ctx context.Context, userID, tenantID string) ([]APIToken, error) {
	return s.repo.ListTokens(ctx, userID, tenantID)
}

// RevokeToken revokes an active API token owned by the current user.
func (s *Service) RevokeToken(ctx context.Context, userID, tenantID, tokenID string) error {
	if strings.TrimSpace(tokenID) == "" {
		return fmt.Errorf("token id is required")
	}
	if err := s.repo.RevokeToken(ctx, userID, tenantID, tokenID, time.Now()); err != nil {
		if err == ErrTokenNotFound {
			return fmt.Errorf("api token not found")
		}
		return err
	}
	return nil
}

// ValidateAPIToken validates a raw API token and returns auth claims for middleware use.
func (s *Service) ValidateAPIToken(ctx context.Context, rawToken string) (*auth.Claims, error) {
	tokenHash := hashToken(rawToken)
	record, err := s.repo.GetValidationRecord(ctx, tokenHash, time.Now())
	if err != nil {
		if err == ErrTokenNotFound {
			return nil, fmt.Errorf("api token not found")
		}
		return nil, err
	}

	if err := s.repo.TouchToken(ctx, record.ID, time.Now()); err != nil {
		return nil, err
	}

	return &auth.Claims{
		UserID:    record.UserID,
		Email:     record.Email,
		TenantID:  record.TenantID,
		Role:      record.Role,
		TokenKind: auth.TokenKindAPIToken,
	}, nil
}

func generateTokenMaterial() (rawToken string, prefix string, tokenHash string, err error) {
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return "", "", "", fmt.Errorf("generate api token: %w", err)
	}

	rawToken = tokenPrefix + hex.EncodeToString(secret)
	prefixLength := min(len(rawToken), 14)
	prefix = rawToken[:prefixLength]
	tokenHash = hashToken(rawToken)

	return rawToken, prefix, tokenHash, nil
}

func hashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}
