package domain

import (
	"time"

	"github.com/google/uuid"
)

// ── Auth DTOs ─────────────────────────────────────────────────────────────────

// RegisterRequest is the payload for /auth/register.
type RegisterRequest struct {
	Name     string `json:"name"     validate:"required,min=2,max=255"`
	Email    string `json:"email"    validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

// LoginRequest is the payload for /auth/login.
type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
	Remember bool   `json:"remember"`
}

// RefreshTokenRequest carries the refresh token.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// PasswordResetRequestDTO is the payload for requesting a password reset.
type PasswordResetRequestDTO struct {
	Email string `json:"email" validate:"required,email"`
}

// PasswordResetConfirmDTO is the payload for confirming a password reset.
type PasswordResetConfirmDTO struct {
	Token       string `json:"token"        validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=128"`
}

// ChangePasswordDTO is used by authenticated users to change their password.
type ChangePasswordDTO struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password"     validate:"required,min=8,max=128"`
}

// VerifyEmailDTO carries the email verification token.
type VerifyEmailDTO struct {
	Token string `json:"token" validate:"required"`
}

// ── Auth Responses ─────────────────────────────────────────────────────────────

// AuthResponse is returned on successful login or token refresh.
type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int64        `json:"expires_in"` // seconds
	User         UserResponse `json:"user"`
}

// ── User DTOs ─────────────────────────────────────────────────────────────────

// CreateUserRequest is used by admins to create a user directly.
type CreateUserRequest struct {
	Name     string     `json:"name"     validate:"required,min=2,max=255"`
	Email    string     `json:"email"    validate:"required,email,max=255"`
	Password string     `json:"password" validate:"required,min=8,max=128"`
	RoleIDs  []uuid.UUID `json:"role_ids"`
	IsActive bool       `json:"is_active"`
}

// UpdateUserRequest is used by admins to update any user.
type UpdateUserRequest struct {
	Name      *string     `json:"name"      validate:"omitempty,min=2,max=255"`
	Email     *string     `json:"email"     validate:"omitempty,email,max=255"`
	IsActive  *bool       `json:"is_active"`
	RoleIDs   []uuid.UUID `json:"role_ids"`
	AvatarURL *string     `json:"avatar_url" validate:"omitempty,url"`
}

// UpdateProfileRequest is used by the authenticated user to update their own profile.
type UpdateProfileRequest struct {
	Name      *string `json:"name"       validate:"omitempty,min=2,max=255"`
	AvatarURL *string `json:"avatar_url" validate:"omitempty,url"`
}

// ── Role DTOs ─────────────────────────────────────────────────────────────────

// CreateRoleRequest creates a new role.
type CreateRoleRequest struct {
	Name        RoleName    `json:"name"         validate:"required,min=2,max=50"`
	DisplayName string      `json:"display_name" validate:"required,min=2,max=100"`
	Description string      `json:"description"  validate:"max=500"`
	Permissions Permissions `json:"permissions"`
}

// UpdateRoleRequest updates an existing role.
type UpdateRoleRequest struct {
	DisplayName *string      `json:"display_name" validate:"omitempty,min=2,max=100"`
	Description *string      `json:"description"  validate:"omitempty,max=500"`
	Permissions *Permissions `json:"permissions"`
}

// AssignRolesRequest assigns roles to a user.
type AssignRolesRequest struct {
	RoleIDs []uuid.UUID `json:"role_ids" validate:"required,min=1"`
}

// ── Response DTOs ──────────────────────────────────────────────────────────────

// UserResponse is the safe public representation of a user.
type UserResponse struct {
	ID            uuid.UUID      `json:"id"`
	Name          string         `json:"name"`
	Email         string         `json:"email"`
	EmailVerified bool           `json:"email_verified"`
	IsActive      bool           `json:"is_active"`
	AvatarURL     string         `json:"avatar_url,omitempty"`
	Roles         []RoleResponse `json:"roles"`
	LastLoginAt   *time.Time     `json:"last_login_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// RoleResponse is the public representation of a role.
type RoleResponse struct {
	ID          uuid.UUID   `json:"id"`
	Name        RoleName    `json:"name"`
	DisplayName string      `json:"display_name"`
	Description string      `json:"description"`
	Permissions Permissions `json:"permissions"`
	IsSystem    bool        `json:"is_system"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// ── Pagination ─────────────────────────────────────────────────────────────────

// PaginationQuery holds pagination parameters parsed from query strings.
type PaginationQuery struct {
	Page    int    `query:"page"    validate:"min=1"`
	PerPage int    `query:"per_page" validate:"min=1,max=100"`
	Search  string `query:"search"`
	SortBy  string `query:"sort_by"`
	SortDir string `query:"sort_dir" validate:"omitempty,oneof=asc desc"`
}

// Paginated is a generic paginated response wrapper.
type Paginated[T any] struct {
	Items      []T   `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	TotalPages int   `json:"total_pages"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// ToUserResponse converts a domain User to a UserResponse DTO.
func ToUserResponse(u *User) UserResponse {
	roles := make([]RoleResponse, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = ToRoleResponse(&r)
	}
	return UserResponse{
		ID:            u.ID,
		Name:          u.Name,
		Email:         u.Email,
		EmailVerified: u.EmailVerified,
		IsActive:      u.IsActive,
		AvatarURL:     u.AvatarURL,
		Roles:         roles,
		LastLoginAt:   u.LastLoginAt,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

// ToRoleResponse converts a domain Role to a RoleResponse DTO.
func ToRoleResponse(r *Role) RoleResponse {
	return RoleResponse{
		ID:          r.ID,
		Name:        r.Name,
		DisplayName: r.DisplayName,
		Description: r.Description,
		Permissions: r.Permissions,
		IsSystem:    r.IsSystem,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// Normalize sets sane defaults for pagination parameters.
func (p *PaginationQuery) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 20
	}
	if p.PerPage > 100 {
		p.PerPage = 100
	}
	if p.SortDir == "" {
		p.SortDir = "desc"
	}
}

// Offset returns the SQL OFFSET value for the current page.
func (p *PaginationQuery) Offset() int {
	return (p.Page - 1) * p.PerPage
}

// NewPaginated creates a paginated response from a slice and total count.
func NewPaginated[T any](items []T, total int64, q *PaginationQuery) Paginated[T] {
	totalPages := int(total) / q.PerPage
	if int(total)%q.PerPage != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}
	return Paginated[T]{
		Items:      items,
		Total:      total,
		Page:       q.Page,
		PerPage:    q.PerPage,
		TotalPages: totalPages,
	}
}

// APIError is a structured API error response.
type APIError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// ValidationError represents a single field validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
