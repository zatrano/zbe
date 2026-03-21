package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/zatrano/zbe/internal/domain"
)

// TokenRepository handles revoked JWT JTIs and password-reset tokens.
type TokenRepository interface {
	RevokeJTI(jti string, userID uuid.UUID, expiresAt time.Time) error
	IsJTIRevoked(jti string) (bool, error)
	CleanupExpiredRevocations() error

	CreatePasswordResetToken(token *domain.PasswordResetToken) error
	FindPasswordResetToken(rawToken string) (*domain.PasswordResetToken, error)
	MarkPasswordResetTokenUsed(id uuid.UUID) error
	InvalidateUserPasswordResetTokens(userID uuid.UUID) error
}

type tokenRepository struct {
	db *gorm.DB
}

// NewTokenRepository returns a new GORM-backed TokenRepository.
func NewTokenRepository(db *gorm.DB) TokenRepository {
	return &tokenRepository{db: db}
}

// ── Revoked JTIs ──────────────────────────────────────────────────────────────

func (r *tokenRepository) RevokeJTI(jti string, userID uuid.UUID, expiresAt time.Time) error {
	rev := &domain.RevokedToken{
		JTI:       jti,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}
	if err := r.db.Create(rev).Error; err != nil {
		return fmt.Errorf("revoke jti: %w", err)
	}
	return nil
}

func (r *tokenRepository) IsJTIRevoked(jti string) (bool, error) {
	var count int64
	err := r.db.Model(&domain.RevokedToken{}).
		Where("jti = ? AND expires_at > NOW()", jti).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check jti revocation: %w", err)
	}
	return count > 0, nil
}

func (r *tokenRepository) CleanupExpiredRevocations() error {
	if err := r.db.Where("expires_at <= NOW()").Delete(&domain.RevokedToken{}).Error; err != nil {
		return fmt.Errorf("cleanup revocations: %w", err)
	}
	return nil
}

// ── Password Reset Tokens ─────────────────────────────────────────────────────

func (r *tokenRepository) CreatePasswordResetToken(token *domain.PasswordResetToken) error {
	if err := r.db.Create(token).Error; err != nil {
		return fmt.Errorf("create password reset token: %w", err)
	}
	return nil
}

func (r *tokenRepository) FindPasswordResetToken(rawToken string) (*domain.PasswordResetToken, error) {
	var prt domain.PasswordResetToken
	err := r.db.
		Where("token = ? AND used_at IS NULL AND expires_at > NOW()", rawToken).
		First(&prt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find password reset token: %w", err)
	}
	return &prt, nil
}

func (r *tokenRepository) MarkPasswordResetTokenUsed(id uuid.UUID) error {
	now := time.Now().UTC()
	if err := r.db.Model(&domain.PasswordResetToken{}).
		Where("id = ?", id).
		Update("used_at", now).Error; err != nil {
		return fmt.Errorf("mark reset token used: %w", err)
	}
	return nil
}

func (r *tokenRepository) InvalidateUserPasswordResetTokens(userID uuid.UUID) error {
	now := time.Now().UTC()
	if err := r.db.Model(&domain.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", now).Error; err != nil {
		return fmt.Errorf("invalidate reset tokens: %w", err)
	}
	return nil
}
