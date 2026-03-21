package service

import (
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/zatrano/zbe/internal/domain"
	"github.com/zatrano/zbe/internal/repository"
	"github.com/zatrano/zbe/pkg/utils"
)

// UserService handles user management operations.
type UserService interface {
	Create(req *domain.CreateUserRequest) (*domain.UserResponse, error)
	GetByID(id uuid.UUID) (*domain.UserResponse, error)
	Update(id uuid.UUID, req *domain.UpdateUserRequest) (*domain.UserResponse, error)
	UpdateProfile(id uuid.UUID, req *domain.UpdateProfileRequest) (*domain.UserResponse, error)
	ChangePassword(id uuid.UUID, req *domain.ChangePasswordDTO) error
	Delete(id uuid.UUID) error
	List(q *domain.PaginationQuery) (*domain.Paginated[domain.UserResponse], error)
	AssignRoles(userID uuid.UUID, req *domain.AssignRolesRequest) (*domain.UserResponse, error)
}

type userService struct {
	userRepo repository.UserRepository
	roleRepo repository.RoleRepository
	cfg      interface{ GetBcryptCost() int }
	bcrypt   int
}

// UserServiceCfg carries only the config fields UserService needs.
type UserServiceCfg struct {
	BcryptCost int
}

// NewUserService constructs a UserService.
func NewUserService(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	bcryptCost int,
) UserService {
	return &userService{
		userRepo: userRepo,
		roleRepo: roleRepo,
		bcrypt:   bcryptCost,
	}
}

func (s *userService) Create(req *domain.CreateUserRequest) (*domain.UserResponse, error) {
	exists, err := s.userRepo.ExistsByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	hash, err := utils.HashPassword(req.Password, s.bcrypt)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		Name:          req.Name,
		Email:         req.Email,
		PasswordHash:  hash,
		EmailVerified: true, // admin-created users are pre-verified
		IsActive:      req.IsActive,
	}

	if len(req.RoleIDs) > 0 {
		roles, err := s.roleRepo.FindByIDs(req.RoleIDs)
		if err != nil {
			return nil, fmt.Errorf("find roles: %w", err)
		}
		user.Roles = roles
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	resp := domain.ToUserResponse(user)
	return &resp, nil
}

func (s *userService) GetByID(id uuid.UUID) (*domain.UserResponse, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}
	resp := domain.ToUserResponse(user)
	return &resp, nil
}

func (s *userService) Update(id uuid.UUID, req *domain.UpdateUserRequest) (*domain.UserResponse, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}

	if req.Name != nil {
		user.Name = utils.SanitizeString(*req.Name)
	}
	if req.Email != nil && *req.Email != user.Email {
		exists, err := s.userRepo.ExistsByEmail(*req.Email)
		if err != nil {
			return nil, fmt.Errorf("check email: %w", err)
		}
		if exists {
			return nil, ErrEmailAlreadyExists
		}
		user.Email = *req.Email
		user.EmailVerified = false
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.AvatarURL != nil {
		user.AvatarURL = *req.AvatarURL
	}

	if err := s.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	if req.RoleIDs != nil {
		if err := s.userRepo.ReplaceRoles(user.ID, req.RoleIDs); err != nil {
			return nil, fmt.Errorf("replace roles: %w", err)
		}
	}

	// Reload to get updated roles
	updated, err := s.userRepo.FindByID(user.ID)
	if err != nil {
		return nil, fmt.Errorf("reload user: %w", err)
	}
	resp := domain.ToUserResponse(updated)
	return &resp, nil
}

func (s *userService) UpdateProfile(id uuid.UUID, req *domain.UpdateProfileRequest) (*domain.UserResponse, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	fields := map[string]interface{}{}
	if req.Name != nil {
		user.Name = utils.SanitizeString(*req.Name)
		fields["name"] = user.Name
	}
	if req.AvatarURL != nil {
		user.AvatarURL = *req.AvatarURL
		fields["avatar_url"] = user.AvatarURL
	}

	if len(fields) > 0 {
		if err := s.userRepo.UpdateFields(id, fields); err != nil {
			return nil, fmt.Errorf("update profile: %w", err)
		}
	}

	updated, _ := s.userRepo.FindByID(id)
	resp := domain.ToUserResponse(updated)
	return &resp, nil
}

func (s *userService) ChangePassword(id uuid.UUID, req *domain.ChangePasswordDTO) error {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return ErrUserNotFound
	}

	if err := utils.CheckPassword(req.CurrentPassword, user.PasswordHash); err != nil {
		return ErrInvalidCredentials
	}

	hash, err := utils.HashPassword(req.NewPassword, s.bcrypt)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	return s.userRepo.UpdateFields(id, map[string]interface{}{"password_hash": hash})
}

func (s *userService) Delete(id uuid.UUID) error {
	_, err := s.userRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("find user: %w", err)
	}
	return s.userRepo.Delete(id)
}

func (s *userService) List(q *domain.PaginationQuery) (*domain.Paginated[domain.UserResponse], error) {
	users, total, err := s.userRepo.List(q)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	responses := make([]domain.UserResponse, len(users))
	for i := range users {
		responses[i] = domain.ToUserResponse(&users[i])
	}

	result := domain.NewPaginated(responses, total, q)
	return &result, nil
}

func (s *userService) AssignRoles(userID uuid.UUID, req *domain.AssignRolesRequest) (*domain.UserResponse, error) {
	if err := s.userRepo.ReplaceRoles(userID, req.RoleIDs); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("assign roles: %w", err)
	}
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("reload user: %w", err)
	}
	resp := domain.ToUserResponse(user)
	return &resp, nil
}

// Service-level errors
var ErrUserNotFound = errors.New("user not found")
