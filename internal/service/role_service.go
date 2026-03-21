package service

import (
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/zatrano/zbe/internal/domain"
	"github.com/zatrano/zbe/internal/repository"
)

// RoleService handles role management operations.
type RoleService interface {
	Create(req *domain.CreateRoleRequest) (*domain.RoleResponse, error)
	GetByID(id uuid.UUID) (*domain.RoleResponse, error)
	Update(id uuid.UUID, req *domain.UpdateRoleRequest) (*domain.RoleResponse, error)
	Delete(id uuid.UUID) error
	List(q *domain.PaginationQuery) (*domain.Paginated[domain.RoleResponse], error)
}

type roleService struct {
	roleRepo repository.RoleRepository
}

// NewRoleService constructs a RoleService.
func NewRoleService(roleRepo repository.RoleRepository) RoleService {
	return &roleService{roleRepo: roleRepo}
}

func (s *roleService) Create(req *domain.CreateRoleRequest) (*domain.RoleResponse, error) {
	exists, err := s.roleRepo.ExistsByName(req.Name)
	if err != nil {
		return nil, fmt.Errorf("check role name: %w", err)
	}
	if exists {
		return nil, ErrRoleNameTaken
	}

	perms := req.Permissions
	if perms == nil {
		perms = domain.Permissions{}
	}

	role := &domain.Role{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Permissions: perms,
		IsSystem:    false,
	}

	if err := s.roleRepo.Create(role); err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}

	resp := domain.ToRoleResponse(role)
	return &resp, nil
}

func (s *roleService) GetByID(id uuid.UUID) (*domain.RoleResponse, error) {
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("find role: %w", err)
	}
	resp := domain.ToRoleResponse(role)
	return &resp, nil
}

func (s *roleService) Update(id uuid.UUID, req *domain.UpdateRoleRequest) (*domain.RoleResponse, error) {
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("find role: %w", err)
	}

	if role.IsSystem {
		// Allow partial updates to system roles but never rename or delete them
		if req.Permissions != nil {
			role.Permissions = *req.Permissions
		}
		if req.Description != nil {
			role.Description = *req.Description
		}
		if req.DisplayName != nil {
			role.DisplayName = *req.DisplayName
		}
	} else {
		if req.DisplayName != nil {
			role.DisplayName = *req.DisplayName
		}
		if req.Description != nil {
			role.Description = *req.Description
		}
		if req.Permissions != nil {
			role.Permissions = *req.Permissions
		}
	}

	if err := s.roleRepo.Update(role); err != nil {
		return nil, fmt.Errorf("update role: %w", err)
	}

	resp := domain.ToRoleResponse(role)
	return &resp, nil
}

func (s *roleService) Delete(id uuid.UUID) error {
	err := s.roleRepo.Delete(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}

func (s *roleService) List(q *domain.PaginationQuery) (*domain.Paginated[domain.RoleResponse], error) {
	roles, total, err := s.roleRepo.List(q)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}

	responses := make([]domain.RoleResponse, len(roles))
	for i := range roles {
		responses[i] = domain.ToRoleResponse(&roles[i])
	}

	result := domain.NewPaginated(responses, total, q)
	return &result, nil
}

// Service-level errors
var (
	ErrRoleNotFound = errors.New("role not found")
	ErrRoleNameTaken = errors.New("role name is already taken")
)
