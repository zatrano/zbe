package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/zatrano/zbe/internal/domain"
)

// OAuthRepository manages OAuth provider records.
type OAuthRepository interface {
	FindByProviderID(provider, providerID string) (*domain.OAuthProvider, error)
	FindByUserID(userID uuid.UUID) ([]domain.OAuthProvider, error)
	Upsert(op *domain.OAuthProvider) error
	Delete(id uuid.UUID) error
}

type oauthRepository struct {
	db *gorm.DB
}

// NewOAuthRepository returns a new GORM-backed OAuthRepository.
func NewOAuthRepository(db *gorm.DB) OAuthRepository {
	return &oauthRepository{db: db}
}

func (r *oauthRepository) FindByProviderID(provider, providerID string) (*domain.OAuthProvider, error) {
	var op domain.OAuthProvider
	err := r.db.Where("provider = ? AND provider_id = ?", provider, providerID).First(&op).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("oauth find by provider: %w", err)
	}
	return &op, nil
}

func (r *oauthRepository) FindByUserID(userID uuid.UUID) ([]domain.OAuthProvider, error) {
	var ops []domain.OAuthProvider
	if err := r.db.Where("user_id = ?", userID).Find(&ops).Error; err != nil {
		return nil, fmt.Errorf("oauth find by user: %w", err)
	}
	return ops, nil
}

func (r *oauthRepository) Upsert(op *domain.OAuthProvider) error {
	result := r.db.Where("provider = ? AND provider_id = ?", op.Provider, op.ProviderID).
		Assign(domain.OAuthProvider{
			UserID:       op.UserID,
			AccessToken:  op.AccessToken,
			RefreshToken: op.RefreshToken,
			TokenExpiry:  op.TokenExpiry,
			Email:        op.Email,
			Name:         op.Name,
			AvatarURL:    op.AvatarURL,
			RawData:      op.RawData,
		}).
		FirstOrCreate(op)
	if result.Error != nil {
		return fmt.Errorf("oauth upsert: %w", result.Error)
	}
	return nil
}

func (r *oauthRepository) Delete(id uuid.UUID) error {
	if err := r.db.Delete(&domain.OAuthProvider{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("oauth delete: %w", err)
	}
	return nil
}
