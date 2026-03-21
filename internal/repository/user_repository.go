package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/zatrano/zbe/internal/domain"
)

// UserRepository defines the interface for user persistence.
type UserRepository interface {
	Create(user *domain.User) error
	FindByID(id uuid.UUID) (*domain.User, error)
	FindByEmail(email string) (*domain.User, error)
	FindByVerifyToken(token string) (*domain.User, error)
	FindByResetToken(token string) (*domain.User, error)
	FindByRefreshToken(token string) (*domain.User, error)
	Update(user *domain.User) error
	UpdateFields(id uuid.UUID, fields map[string]interface{}) error
	Delete(id uuid.UUID) error
	List(q *domain.PaginationQuery) ([]domain.User, int64, error)
	AssignRoles(userID uuid.UUID, roleIDs []uuid.UUID) error
	RemoveRoles(userID uuid.UUID, roleIDs []uuid.UUID) error
	ReplaceRoles(userID uuid.UUID, roleIDs []uuid.UUID) error
	ExistsByEmail(email string) (bool, error)
	Count() (int64, error)
}

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository returns a new GORM-backed UserRepository.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *domain.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("user create: %w", err)
	}
	return nil
}

func (r *userRepository) FindByID(id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.
		Preload("Roles").
		Preload("OAuthProviders").
		First(&user, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("user find by id: %w", err)
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(email string) (*domain.User, error) {
	var user domain.User
	err := r.db.
		Preload("Roles").
		First(&user, "email = ?", email).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("user find by email: %w", err)
	}
	return &user, nil
}

func (r *userRepository) FindByVerifyToken(token string) (*domain.User, error) {
	var user domain.User
	err := r.db.
		Preload("Roles").
		First(&user, "verify_token = ?", token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("user find by verify token: %w", err)
	}
	return &user, nil
}

func (r *userRepository) FindByResetToken(token string) (*domain.User, error) {
	var prt domain.PasswordResetToken
	err := r.db.
		Where("token = ? AND used_at IS NULL AND expires_at > NOW()", token).
		First(&prt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find reset token: %w", err)
	}
	return r.FindByID(prt.UserID)
}

func (r *userRepository) FindByRefreshToken(token string) (*domain.User, error) {
	var user domain.User
	err := r.db.
		Preload("Roles").
		Where("refresh_token = ? AND refresh_token_exp > NOW()", token).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("user find by refresh token: %w", err)
	}
	return &user, nil
}

func (r *userRepository) Update(user *domain.User) error {
	if err := r.db.Save(user).Error; err != nil {
		return fmt.Errorf("user update: %w", err)
	}
	return nil
}

func (r *userRepository) UpdateFields(id uuid.UUID, fields map[string]interface{}) error {
	if err := r.db.Model(&domain.User{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return fmt.Errorf("user update fields: %w", err)
	}
	return nil
}

func (r *userRepository) Delete(id uuid.UUID) error {
	if err := r.db.Delete(&domain.User{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("user delete: %w", err)
	}
	return nil
}

func (r *userRepository) List(q *domain.PaginationQuery) ([]domain.User, int64, error) {
	q.Normalize()
	var users []domain.User
	var total int64

	query := r.db.Model(&domain.User{}).Preload("Roles")

	if q.Search != "" {
		search := "%" + q.Search + "%"
		query = query.Where("name ILIKE ? OR email ILIKE ?", search, search)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("user list count: %w", err)
	}

	sortBy := "created_at"
	if q.SortBy != "" {
		// Allowlist to prevent SQL injection
		allowed := map[string]bool{"name": true, "email": true, "created_at": true, "updated_at": true}
		if allowed[q.SortBy] {
			sortBy = q.SortBy
		}
	}
	order := sortBy + " " + q.SortDir

	if err := query.
		Order(order).
		Limit(q.PerPage).
		Offset(q.Offset()).
		Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("user list: %w", err)
	}

	return users, total, nil
}

func (r *userRepository) AssignRoles(userID uuid.UUID, roleIDs []uuid.UUID) error {
	user, err := r.FindByID(userID)
	if err != nil {
		return err
	}
	var roles []domain.Role
	if err := r.db.Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
		return fmt.Errorf("find roles: %w", err)
	}
	if err := r.db.Model(user).Association("Roles").Append(&roles); err != nil {
		return fmt.Errorf("assign roles: %w", err)
	}
	return nil
}

func (r *userRepository) RemoveRoles(userID uuid.UUID, roleIDs []uuid.UUID) error {
	user, err := r.FindByID(userID)
	if err != nil {
		return err
	}
	var roles []domain.Role
	if err := r.db.Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
		return fmt.Errorf("find roles: %w", err)
	}
	if err := r.db.Model(user).Association("Roles").Delete(&roles); err != nil {
		return fmt.Errorf("remove roles: %w", err)
	}
	return nil
}

func (r *userRepository) ReplaceRoles(userID uuid.UUID, roleIDs []uuid.UUID) error {
	user, err := r.FindByID(userID)
	if err != nil {
		return err
	}
	var roles []domain.Role
	if len(roleIDs) > 0 {
		if err := r.db.Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
			return fmt.Errorf("find roles for replace: %w", err)
		}
	}
	if err := r.db.Model(user).Association("Roles").Replace(&roles); err != nil {
		return fmt.Errorf("replace roles: %w", err)
	}
	return nil
}

func (r *userRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	if err := r.db.Model(&domain.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return false, fmt.Errorf("user exists by email: %w", err)
	}
	return count > 0, nil
}

func (r *userRepository) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&domain.User{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("user count: %w", err)
	}
	return count, nil
}
