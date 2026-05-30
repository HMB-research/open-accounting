package apitoken

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"gorm.io/gorm"
)

var (
	ErrTokenNotFound = fmt.Errorf("api token not found")
)

// Repository defines API token storage operations.
type Repository interface {
	CreateToken(ctx context.Context, token *APIToken, tokenHash string) error
	ListTokens(ctx context.Context, userID, tenantID string) ([]APIToken, error)
	RevokeToken(ctx context.Context, userID, tenantID, tokenID string, revokedAt time.Time) error
	GetValidationRecord(ctx context.Context, tokenHash string, now time.Time) (*ValidationRecord, error)
	TouchToken(ctx context.Context, tokenID string, lastUsedAt time.Time) error
}

// GORMRepository stores API tokens through the shared ORM layer.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates an ORM-backed API token repository.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) CreateToken(ctx context.Context, token *APIToken, tokenHash string) error {
	tokenModel := apiTokenToModel(token, tokenHash)
	if err := r.db.WithContext(ctx).Create(tokenModel).Error; err != nil {
		return fmt.Errorf("create api token: %w", err)
	}
	return nil
}

func (r *GORMRepository) ListTokens(ctx context.Context, userID, tenantID string) ([]APIToken, error) {
	var tokenModels []models.APIToken
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Where("revoked_at IS NULL").
		Order("created_at DESC").
		Find(&tokenModels).Error; err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}

	tokens := make([]APIToken, len(tokenModels))
	for i := range tokenModels {
		tokens[i] = *modelToAPIToken(&tokenModels[i])
	}
	return tokens, nil
}

func (r *GORMRepository) RevokeToken(ctx context.Context, userID, tenantID, tokenID string, revokedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&models.APIToken{}).
		Where("id = ? AND user_id = ? AND tenant_id = ? AND revoked_at IS NULL", tokenID, userID, tenantID).
		Update("revoked_at", revokedAt)
	if result.Error != nil {
		return fmt.Errorf("revoke api token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTokenNotFound
	}
	return nil
}

func (r *GORMRepository) GetValidationRecord(ctx context.Context, tokenHash string, now time.Time) (*ValidationRecord, error) {
	var result struct {
		models.APIToken
		Email string
		Role  string
	}

	err := r.db.WithContext(ctx).
		Table("api_tokens AS at").
		Select("at.*, u.email, tu.role").
		Joins("JOIN users AS u ON u.id = at.user_id AND u.is_active = ?", true).
		Joins("JOIN tenants AS t ON t.id = at.tenant_id AND t.is_active = ?", true).
		Joins("JOIN tenant_users AS tu ON tu.tenant_id = at.tenant_id AND tu.user_id = at.user_id AND COALESCE(tu.is_active, true) = ?", true).
		Where("at.token_hash = ?", tokenHash).
		Where("at.revoked_at IS NULL").
		Where("at.expires_at IS NULL OR at.expires_at > ?", now).
		Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get api token validation record: %w", err)
	}

	record := &ValidationRecord{
		APIToken: *modelToAPIToken(&result.APIToken),
		Email:    result.Email,
		Role:     result.Role,
	}
	return record, nil
}

func (r *GORMRepository) TouchToken(ctx context.Context, tokenID string, lastUsedAt time.Time) error {
	if err := r.db.WithContext(ctx).Model(&models.APIToken{}).
		Where("id = ?", tokenID).
		Update("last_used_at", lastUsedAt).Error; err != nil {
		return fmt.Errorf("touch api token: %w", err)
	}
	return nil
}

func apiTokenToModel(token *APIToken, tokenHash string) *models.APIToken {
	return &models.APIToken{
		ID:          token.ID,
		TenantID:    token.TenantID,
		UserID:      token.UserID,
		Name:        token.Name,
		TokenHash:   tokenHash,
		TokenPrefix: token.TokenPrefix,
		LastUsedAt:  token.LastUsedAt,
		ExpiresAt:   token.ExpiresAt,
		RevokedAt:   token.RevokedAt,
		CreatedAt:   token.CreatedAt,
	}
}

func modelToAPIToken(token *models.APIToken) *APIToken {
	return &APIToken{
		ID:          token.ID,
		TenantID:    token.TenantID,
		UserID:      token.UserID,
		Name:        token.Name,
		TokenPrefix: token.TokenPrefix,
		LastUsedAt:  token.LastUsedAt,
		ExpiresAt:   token.ExpiresAt,
		RevokedAt:   token.RevokedAt,
		CreatedAt:   token.CreatedAt,
	}
}
