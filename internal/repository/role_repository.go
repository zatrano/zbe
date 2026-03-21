package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/zatrano/zbe/internal/domain"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = errors.New("record not found")

// ErrDuplicateKey is returned when a unique constraint is violated.
var ErrDuplicateKey = errors.New("duplicate key")

// RoleRepository defines the interface for role persistence.
type RoleRepository interface {
	Create(role *domain.Role) error
	FindByID(id uuid.UUID) (*domain.Role, error)
	FindByName(name domain.RoleName) (*domain.Role, error)
	FindByIDs(ids []uuid.UUID) ([]domain.Role, error)
	Update(role *domain.Role) error
	Delete(id uuid.UUID) error
	List(q *domain.PaginationQuery) ([]domain.Role, int64, error)
	ExistsByName(name domain.RoleName) (bool, error)
}

type roleRepository struct {
	db *gorm.DB
}

// NewRoleRepository returns a new GORM-backed RoleRepository.
func NewRoleRepository(db *gorm.DB) RoleRepository {
	return &roleRepository{db: db}
}

func (r *roleRepository) Create(role *domain.Role) error {
	if err := r.db.Create(role).Error; err != nil {
		return fmt.Errorf("role create: %w", err)
	}
	return nil
}

func (r *roleRepository) FindByID(id uuid.UUID) (*domain.Role, error) {
	var role domain.Role
	err := r.db.First(&role, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("role find by id: %w", err)
	}
	return &role, nil
}

func (r *roleRepository) FindByName(name domain.RoleName) (*domain.Role, error) {
	var role domain.Role
	err := r.db.First(&role, "name = ?", name).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("role find by name: %w", err)
	}
	return &role, nil
}

func (r *roleRepository) FindByIDs(ids []uuid.UUID) ([]domain.Role, error) {
	var roles []domain.Role
	if err := r.db.Where("id IN ?", ids).Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("role find by ids: %w", err)
	}
	return roles, nil
}

func (r *roleRepository) Update(role *domain.Role) error {
	if err := r.db.Save(role).Error; err != nil {
		return fmt.Errorf("role update: %w", err)
	}
	return nil
}

func (r *roleRepository) Delete(id uuid.UUID) error {
	var role domain.Role
	if err := r.db.First(&role, "id = ?", id).Error; err != nil {
		return ErrNotFound
	}
	if role.IsSystem {
		return fmt.Errorf("cannot delete system role")
	}
	if err := r.db.Delete(&role).Error; err != nil {
		return fmt.Errorf("role delete: %w", err)
	}
	return nil
}

func (r *roleRepository) List(q *domain.PaginationQuery) ([]domain.Role, int64, error) {
	q.Normalize()
	var roles []domain.Role
	var total int64

	query := r.db.Model(&domain.Role{})
	if q.Search != "" {
		search := "%" + q.Search + "%"
		query = query.Where("name ILIKE ? OR display_name ILIKE ?", search, search)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("role list count: %w", err)
	}

	if err := query.
		Order("created_at desc").
		Limit(q.PerPage).
		Offset(q.Offset()).
		Find(&roles).Error; err != nil {
		return nil, 0, fmt.Errorf("role list: %w", err)
	}

	return roles, total, nil
}

func (r *roleRepository) ExistsByName(name domain.RoleName) (bool, error) {
	var count int64
	if err := r.db.Model(&domain.Role{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return false, fmt.Errorf("role exists by name: %w", err)
	}
	return count > 0, nil
}
