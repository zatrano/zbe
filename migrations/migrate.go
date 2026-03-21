package migrations

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/zatrano/zbe/internal/domain"
	"github.com/zatrano/zbe/pkg/logger"
)

// Run executes all GORM auto-migrations.
// It is idempotent — safe to call on every startup.
func Run(db *gorm.DB) error {
	logger.Info("running database migrations...")

	// Enable the pgcrypto extension for gen_random_uuid() support
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pgcrypto").Error; err != nil {
		logger.Warnf("pgcrypto extension: %v", err)
	}

	models := []interface{}{
		&domain.Role{},
		&domain.User{},
		&domain.OAuthProvider{},
		&domain.PasswordResetToken{},
		&domain.RevokedToken{},
	}

	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("migrate %T: %w", model, err)
		}
	}

	// Add indexes not expressible through struct tags
	addIndexes(db)

	logger.Info("migrations completed successfully")
	return nil
}

func addIndexes(db *gorm.DB) {
	// Composite index for OAuth lookup
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_provider_unique
		ON oauth_providers(provider, provider_id)`)

	// Partial index for active refresh tokens
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_refresh_token_active
		ON users(refresh_token)
		WHERE refresh_token IS NOT NULL AND deleted_at IS NULL`)

	// Index for fast password reset lookups on valid tokens
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_prt_token_valid
		ON password_reset_tokens(token)
		WHERE used_at IS NULL`)

	// Cleanup index for revoked tokens
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_revoked_tokens_expires
		ON revoked_tokens(expires_at)`)
}
