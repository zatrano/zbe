package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/zatrano/zbe/config"
	"github.com/zatrano/zbe/internal/domain"
	"github.com/zatrano/zbe/internal/repository"
	"github.com/zatrano/zbe/pkg/utils"
)

// ContextKey constants for values stored in fiber.Ctx.Locals.
const (
	LocalUserID    = "user_id"
	LocalUserEmail = "user_email"
	LocalUserRoles = "user_roles"
	LocalJTI       = "jti"
	LocalClaims    = "claims"
)

// RequireAuth validates the JWT Bearer token and populates locals.
// Aborts with 401 if the token is missing, invalid, or revoked.
func RequireAuth(cfg *config.Config, tokenRepo repository.TokenRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := extractBearerToken(c)
		if raw == "" {
			return utils.RespondUnauthorized(c, "authentication required")
		}

		claims, err := utils.ParseToken(raw, cfg.JWT.Secret)
		if err != nil {
			return utils.RespondUnauthorized(c, "invalid or expired token")
		}

		if claims.TokenType != utils.TokenTypeAccess {
			return utils.RespondUnauthorized(c, "invalid token type")
		}

		// Check against revocation list
		revoked, err := tokenRepo.IsJTIRevoked(claims.ID)
		if err != nil {
			return utils.RespondInternalError(c)
		}
		if revoked {
			return utils.RespondUnauthorized(c, "token has been revoked")
		}

		// Populate locals for downstream handlers
		c.Locals(LocalUserID, claims.UserID)
		c.Locals(LocalUserEmail, claims.Email)
		c.Locals(LocalUserRoles, claims.Roles)
		c.Locals(LocalJTI, claims.ID)
		c.Locals(LocalClaims, claims)

		return c.Next()
	}
}

// OptionalAuth parses the JWT if present but does not abort if absent.
func OptionalAuth(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := extractBearerToken(c)
		if raw == "" {
			return c.Next()
		}

		claims, err := utils.ParseToken(raw, cfg.JWT.Secret)
		if err != nil {
			return c.Next() // invalid token — treat as unauthenticated
		}

		if claims.TokenType == utils.TokenTypeAccess {
			c.Locals(LocalUserID, claims.UserID)
			c.Locals(LocalUserEmail, claims.Email)
			c.Locals(LocalUserRoles, claims.Roles)
			c.Locals(LocalJTI, claims.ID)
			c.Locals(LocalClaims, claims)
		}

		return c.Next()
	}
}

// RequireRole returns a middleware that checks the authenticated user has at least one
// of the supplied roles.
func RequireRole(roles ...domain.RoleName) fiber.Handler {
	roleSet := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		roleSet[string(r)] = struct{}{}
	}

	return func(c *fiber.Ctx) error {
		userRoles, ok := c.Locals(LocalUserRoles).([]string)
		if !ok || len(userRoles) == 0 {
			return utils.RespondForbidden(c)
		}

		for _, r := range userRoles {
			if _, allowed := roleSet[r]; allowed {
				return c.Next()
			}
		}

		return utils.RespondForbidden(c)
	}
}

// RequireAdmin is a convenience wrapper for RequireRole(domain.RoleAdmin).
func RequireAdmin() fiber.Handler {
	return RequireRole(domain.RoleAdmin)
}

// RequireAdminOrModerator allows admin or moderator access.
func RequireAdminOrModerator() fiber.Handler {
	return RequireRole(domain.RoleAdmin, domain.RoleModerator)
}

// RequireEmailVerified aborts with 403 if the user's email is not verified.
// Needs the full user record; lightweight approach uses a claim extension.
func RequireEmailVerified(userRepo repository.UserRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals(LocalUserID).(uuid.UUID)
		if !ok {
			return utils.RespondUnauthorized(c)
		}

		user, err := userRepo.FindByID(userID)
		if err != nil {
			return utils.RespondUnauthorized(c)
		}

		if !user.EmailVerified {
			return utils.RespondForbidden(c, "email address is not verified")
		}

		return c.Next()
	}
}

// GetUserID extracts the authenticated user's UUID from fiber locals.
// Returns uuid.Nil and false if not present.
func GetUserID(c *fiber.Ctx) (uuid.UUID, bool) {
	id, ok := c.Locals(LocalUserID).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// GetClaims extracts the JWT claims from fiber locals.
func GetClaims(c *fiber.Ctx) (*utils.ZatranoClaims, bool) {
	cl, ok := c.Locals(LocalClaims).(*utils.ZatranoClaims)
	return cl, ok
}

// GetJTI extracts the JWT ID from fiber locals.
func GetJTI(c *fiber.Ctx) string {
	jti, _ := c.Locals(LocalJTI).(string)
	return jti
}

// ── helpers ────────────────────────────────────────────────────────────────────

// extractBearerToken reads the token from the Authorization header or
// the access_token query parameter (for SSE/WebSocket compatibility).
func extractBearerToken(c *fiber.Ctx) string {
	auth := c.Get(fiber.HeaderAuthorization)
	if auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	// Fallback: query param
	if token := c.Query("access_token"); token != "" {
		return token
	}
	return ""
}
