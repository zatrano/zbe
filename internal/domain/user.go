package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RoleName defines the available role types.
type RoleName string

const (
	RoleAdmin     RoleName = "admin"
	RoleUser      RoleName = "user"
	RoleModerator RoleName = "moderator"
)

// Permission defines granular access permissions.
type Permission string

const (
	PermUsersRead    Permission = "users:read"
	PermUsersWrite   Permission = "users:write"
	PermUsersDelete  Permission = "users:delete"
	PermRolesRead    Permission = "roles:read"
	PermRolesWrite   Permission = "roles:write"
	PermFormsRead    Permission = "forms:read"
	PermFormsWrite   Permission = "forms:write"
	PermFormsDelete  Permission = "forms:delete"
	PermAdminAccess  Permission = "admin:access"
)

// Role represents a user role with associated permissions.
type Role struct {
	ID          uuid.UUID    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        RoleName     `gorm:"type:varchar(50);uniqueIndex;not null"          json:"name"`
	DisplayName string       `gorm:"type:varchar(100);not null"                     json:"display_name"`
	Description string       `gorm:"type:text"                                      json:"description"`
	Permissions Permissions  `gorm:"type:jsonb;default:'[]'"                        json:"permissions"`
	IsSystem    bool         `gorm:"default:false"                                  json:"is_system"`
	CreatedAt   time.Time    `                                                      json:"created_at"`
	UpdatedAt   time.Time    `                                                      json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index"                                        json:"deleted_at,omitempty"`
}

// User represents an application user.
type User struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name             string         `gorm:"type:varchar(255);not null"                     json:"name"`
	Email            string         `gorm:"type:varchar(255);uniqueIndex;not null"         json:"email"`
	PasswordHash     string         `gorm:"type:varchar(255)"                              json:"-"`
	EmailVerified    bool           `gorm:"default:false"                                  json:"email_verified"`
	VerifyToken      string         `gorm:"type:varchar(255);index"                        json:"-"`
	VerifyTokenExp   *time.Time     `                                                      json:"-"`
	ResetToken       string         `gorm:"type:varchar(255);index"                        json:"-"`
	ResetTokenExp    *time.Time     `                                                      json:"-"`
	RefreshToken     string         `gorm:"type:text"                                      json:"-"`
	RefreshTokenExp  *time.Time     `                                                      json:"-"`
	LastLoginAt      *time.Time     `                                                      json:"last_login_at,omitempty"`
	LastLoginIP      string         `gorm:"type:varchar(45)"                               json:"last_login_ip,omitempty"`
	IsActive         bool           `gorm:"default:true"                                   json:"is_active"`
	AvatarURL        string         `gorm:"type:text"                                      json:"avatar_url,omitempty"`
	Roles            []Role         `gorm:"many2many:user_roles"                           json:"roles,omitempty"`
	OAuthProviders   []OAuthProvider `gorm:"foreignKey:UserID"                             json:"oauth_providers,omitempty"`
	CreatedAt        time.Time      `                                                      json:"created_at"`
	UpdatedAt        time.Time      `                                                      json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index"                                          json:"deleted_at,omitempty"`
}

// OAuthProvider stores OAuth provider credentials for a user.
type OAuthProvider struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID       uuid.UUID  `gorm:"type:uuid;not null;index"                       json:"user_id"`
	Provider     string     `gorm:"type:varchar(50);not null"                      json:"provider"`
	ProviderID   string     `gorm:"type:varchar(255);not null"                     json:"provider_id"`
	AccessToken  string     `gorm:"type:text"                                      json:"-"`
	RefreshToken string     `gorm:"type:text"                                      json:"-"`
	TokenExpiry  *time.Time `                                                      json:"-"`
	Email        string     `gorm:"type:varchar(255)"                              json:"email"`
	Name         string     `gorm:"type:varchar(255)"                              json:"name"`
	AvatarURL    string     `gorm:"type:text"                                      json:"avatar_url"`
	RawData      JSONB      `gorm:"type:jsonb"                                     json:"-"`
	CreatedAt    time.Time  `                                                      json:"created_at"`
	UpdatedAt    time.Time  `                                                      json:"updated_at"`
}

// PasswordResetToken is a standalone record for password reset flow.
type PasswordResetToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index"                       json:"user_id"`
	Token     string     `gorm:"type:varchar(255);uniqueIndex;not null"         json:"-"`
	ExpiresAt time.Time  `gorm:"not null"                                       json:"expires_at"`
	UsedAt    *time.Time `                                                      json:"used_at,omitempty"`
	CreatedAt time.Time  `                                                      json:"created_at"`
}

// RevokedToken tracks revoked JWT tokens until they expire.
type RevokedToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	JTI       string    `gorm:"type:varchar(255);uniqueIndex;not null"         json:"jti"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"                       json:"user_id"`
	ExpiresAt time.Time `gorm:"not null;index"                                 json:"expires_at"`
	CreatedAt time.Time `                                                      json:"created_at"`
}

// TableName overrides for explicit table names.
func (Role) TableName() string              { return "roles" }
func (User) TableName() string              { return "users" }
func (OAuthProvider) TableName() string     { return "oauth_providers" }
func (PasswordResetToken) TableName() string { return "password_reset_tokens" }
func (RevokedToken) TableName() string       { return "revoked_tokens" }

// HasRole returns true if the user has the given role.
func (u *User) HasRole(role RoleName) bool {
	for _, r := range u.Roles {
		if r.Name == role {
			return true
		}
	}
	return false
}

// HasPermission returns true if any of the user's roles includes the given permission.
func (u *User) HasPermission(perm Permission) bool {
	for _, r := range u.Roles {
		for _, p := range r.Permissions {
			if p == perm {
				return true
			}
		}
	}
	return false
}

// IsAdmin returns true if the user has the admin role.
func (u *User) IsAdmin() bool { return u.HasRole(RoleAdmin) }

// IsModerator returns true if the user has the moderator role.
func (u *User) IsModerator() bool { return u.HasRole(RoleModerator) }

// GetRoleNames returns a slice of role name strings.
func (u *User) GetRoleNames() []string {
	names := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		names[i] = string(r.Name)
	}
	return names
}

// GetPermissions returns a deduplicated slice of all permissions across all roles.
func (u *User) GetPermissions() []Permission {
	seen := make(map[Permission]struct{})
	var perms []Permission
	for _, r := range u.Roles {
		for _, p := range r.Permissions {
			if _, ok := seen[p]; !ok {
				seen[p] = struct{}{}
				perms = append(perms, p)
			}
		}
	}
	return perms
}

// DefaultAdminPermissions returns the full permission set for admins.
func DefaultAdminPermissions() Permissions {
	return Permissions{
		PermUsersRead, PermUsersWrite, PermUsersDelete,
		PermRolesRead, PermRolesWrite,
		PermFormsRead, PermFormsWrite, PermFormsDelete,
		PermAdminAccess,
	}
}

// DefaultModeratorPermissions returns the permission set for moderators.
func DefaultModeratorPermissions() Permissions {
	return Permissions{
		PermUsersRead,
		PermFormsRead, PermFormsWrite,
	}
}

// DefaultUserPermissions returns the permission set for regular users.
func DefaultUserPermissions() Permissions {
	return Permissions{
		PermFormsRead, PermFormsWrite,
	}
}
