package seed

import (
	"errors"
	"fmt"
	"os"

	"gorm.io/gorm"

	"github.com/zatrano/zbe/internal/domain"
	"github.com/zatrano/zbe/internal/repository"
	"github.com/zatrano/zbe/pkg/logger"
	"github.com/zatrano/zbe/pkg/utils"
)

// Run seeds the database with default roles and the initial admin user.
// It is idempotent — existing records are skipped.
func Run(db *gorm.DB) error {
	logger.Info("running database seeds...")

	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)

	if err := seedRoles(roleRepo); err != nil {
		return fmt.Errorf("seed roles: %w", err)
	}

	if err := seedAdminUser(userRepo, roleRepo); err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}

	logger.Info("seeding completed successfully")
	return nil
}

// seedRoles creates the three default system roles.
func seedRoles(repo repository.RoleRepository) error {
	roles := []domain.Role{
		{
			Name:        domain.RoleAdmin,
			DisplayName: "Administrator",
			Description: "Full access to all resources and settings.",
			Permissions: domain.DefaultAdminPermissions(),
			IsSystem:    true,
		},
		{
			Name:        domain.RoleModerator,
			DisplayName: "Moderator",
			Description: "Can manage forms and read user data.",
			Permissions: domain.DefaultModeratorPermissions(),
			IsSystem:    true,
		},
		{
			Name:        domain.RoleUser,
			DisplayName: "User",
			Description: "Standard authenticated user.",
			Permissions: domain.DefaultUserPermissions(),
			IsSystem:    true,
		},
	}

	for _, r := range roles {
		exists, err := repo.ExistsByName(r.Name)
		if err != nil {
			return fmt.Errorf("check role %s: %w", r.Name, err)
		}
		if exists {
			logger.Infof("role %q already exists, skipping", r.Name)
			continue
		}

		role := r // copy to avoid loop-variable capture
		if err := repo.Create(&role); err != nil {
			return fmt.Errorf("create role %s: %w", r.Name, err)
		}
		logger.Infof("created role: %q", r.Name)
	}

	return nil
}

// seedAdminUser creates the initial admin user from environment variables
// (or falls back to safe defaults for development).
func seedAdminUser(userRepo repository.UserRepository, roleRepo repository.RoleRepository) error {
	adminEmail := getEnv("SEED_ADMIN_EMAIL", "admin@zatrano.com")

	exists, err := userRepo.ExistsByEmail(adminEmail)
	if err != nil {
		return fmt.Errorf("check admin user: %w", err)
	}
	if exists {
		logger.Infof("admin user %q already exists, skipping", adminEmail)
		return nil
	}

	adminPassword := getEnv("SEED_ADMIN_PASSWORD", "Admin@123456")
	bcryptCost := 12

	hash, err := utils.HashPassword(adminPassword, bcryptCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	adminRole, err := roleRepo.FindByName(domain.RoleAdmin)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("admin role not found — run role seeds first")
		}
		return fmt.Errorf("find admin role: %w", err)
	}

	admin := &domain.User{
		Name:          getEnv("SEED_ADMIN_NAME", "Administrator"),
		Email:         adminEmail,
		PasswordHash:  hash,
		EmailVerified: true,
		IsActive:      true,
		Roles:         []domain.Role{*adminRole},
	}

	if err := userRepo.Create(admin); err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	logger.Infof("created admin user: %q", adminEmail)
	logger.Infof("⚠️  default admin password set — change it immediately in production!")
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
