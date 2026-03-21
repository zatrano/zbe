package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/zatrano/zbe/config"
	"github.com/zatrano/zbe/internal/middleware"
	"github.com/zatrano/zbe/internal/service"
	"github.com/zatrano/zbe/internal/domain"
	"github.com/zatrano/zbe/pkg/utils"
)

// AuthHandler handles authentication-related HTTP endpoints.
type AuthHandler struct {
	authSvc service.AuthService
	cfg     *config.Config
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(authSvc service.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, cfg: cfg}
}

// Register handles POST /api/v1/auth/register
//
//	@Summary     Register a new user
//	@Description Creates an account, sends a verification email, and returns tokens.
//	@Tags        auth
//	@Accept      json
//	@Produce     json
//	@Param       body body domain.RegisterRequest true "Registration payload"
//	@Success     201 {object} domain.AuthResponse
//	@Failure     409 {object} map[string]interface{}
//	@Failure     422 {object} map[string]interface{}
//	@Router      /auth/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req domain.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.RespondBadRequest(c, "invalid request body")
	}

	// Sanitize
	req.Name  = utils.SanitizeString(req.Name)
	req.Email = utils.SanitizeString(req.Email)

	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.RespondValidationError(c, errs)
	}

	resp, err := h.authSvc.Register(&req, c.IP())
	if err != nil {
		if errors.Is(err, service.ErrEmailAlreadyExists) {
			return utils.RespondConflict(c, "email address is already registered")
		}
		return utils.RespondInternalError(c)
	}

	h.setRefreshCookie(c, resp.RefreshToken)
	return utils.RespondCreated(c, resp)
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req domain.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.RespondBadRequest(c, "invalid request body")
	}

	req.Email = utils.SanitizeString(req.Email)

	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.RespondValidationError(c, errs)
	}

	resp, err := h.authSvc.Login(&req, c.IP())
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			return utils.RespondError(c, fiber.StatusUnauthorized, "invalid email or password")
		case errors.Is(err, service.ErrAccountDisabled):
			return utils.RespondError(c, fiber.StatusForbidden, "your account has been disabled")
		default:
			return utils.RespondInternalError(c)
		}
	}

	h.setRefreshCookie(c, resp.RefreshToken)
	return utils.RespondOK(c, resp)
}

// Logout handles POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return utils.RespondUnauthorized(c)
	}

	claims, hasClaims := middleware.GetClaims(c)
	var jti string
	var exp time.Time
	if hasClaims {
		jti = claims.ID
		exp = claims.ExpiresAt.Time
	}

	if err := h.authSvc.Logout(userID, jti, exp); err != nil {
		return utils.RespondInternalError(c)
	}

	// Clear the refresh token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Expires:  time.Unix(0, 0),
		HTTPOnly: true,
		Secure:   h.cfg.Security.CookieSecure,
		SameSite: h.cfg.Security.CookieSameSite,
		Path:     "/",
	})

	return utils.RespondOK(c, fiber.Map{"message": "logged out successfully"})
}

// RefreshToken handles POST /api/v1/auth/refresh-token
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	// Accept token from body OR HttpOnly cookie
	refreshToken := c.Cookies("refresh_token")
	if refreshToken == "" {
		var req domain.RefreshTokenRequest
		if err := c.BodyParser(&req); err == nil && req.RefreshToken != "" {
			refreshToken = req.RefreshToken
		}
	}

	if refreshToken == "" {
		return utils.RespondUnauthorized(c, "refresh token required")
	}

	resp, err := h.authSvc.RefreshToken(refreshToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken),
			errors.Is(err, service.ErrTokenRevoked):
			return utils.RespondUnauthorized(c, "invalid or expired refresh token")
		case errors.Is(err, service.ErrAccountDisabled):
			return utils.RespondError(c, fiber.StatusForbidden, "account has been disabled")
		default:
			return utils.RespondInternalError(c)
		}
	}

	h.setRefreshCookie(c, resp.RefreshToken)
	return utils.RespondOK(c, resp)
}

// VerifyEmail handles GET /api/v1/auth/verify-email?token=...
func (h *AuthHandler) VerifyEmail(c *fiber.Ctx) error {
	token := c.Query("token")
	if token == "" {
		return utils.RespondBadRequest(c, "verification token is required")
	}

	if err := h.authSvc.VerifyEmail(token); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			return utils.RespondBadRequest(c, "invalid or expired verification token")
		case errors.Is(err, service.ErrTokenExpired):
			return utils.RespondBadRequest(c, "verification token has expired, please request a new one")
		default:
			return utils.RespondInternalError(c)
		}
	}

	return utils.RespondOK(c, fiber.Map{"message": "email verified successfully"})
}

// RequestPasswordReset handles POST /api/v1/password-reset/request
func (h *AuthHandler) RequestPasswordReset(c *fiber.Ctx) error {
	var req domain.PasswordResetRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return utils.RespondBadRequest(c, "invalid request body")
	}

	req.Email = utils.SanitizeString(req.Email)

	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.RespondValidationError(c, errs)
	}

	// Always respond with success to prevent email enumeration
	_ = h.authSvc.RequestPasswordReset(req.Email)

	return utils.RespondOK(c, fiber.Map{
		"message": "if that email is registered you will receive a password reset link shortly",
	})
}

// ConfirmPasswordReset handles POST /api/v1/password-reset/confirm
func (h *AuthHandler) ConfirmPasswordReset(c *fiber.Ctx) error {
	var req domain.PasswordResetConfirmDTO
	if err := c.BodyParser(&req); err != nil {
		return utils.RespondBadRequest(c, "invalid request body")
	}

	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.RespondValidationError(c, errs)
	}

	if err := h.authSvc.ConfirmPasswordReset(&req); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			return utils.RespondBadRequest(c, "invalid or expired reset token")
		default:
			return utils.RespondInternalError(c)
		}
	}

	return utils.RespondOK(c, fiber.Map{"message": "password has been reset successfully"})
}

// GoogleLogin handles GET /api/v1/auth/google/login
// Redirects the user to Google's OAuth2 consent page.
func (h *AuthHandler) GoogleLogin(c *fiber.Ctx) error {
	// Generate CSRF state token
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return utils.RespondInternalError(c)
	}
	state := hex.EncodeToString(stateBytes)

	// Store state in a short-lived cookie for CSRF validation in callback
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute),
		HTTPOnly: true,
		Secure:   h.cfg.Security.CookieSecure,
		SameSite: "Lax",
		Path:     "/",
	})

	url := h.authSvc.GetGoogleAuthURL(state)
	return c.Redirect(url, fiber.StatusTemporaryRedirect)
}

// GoogleCallback handles GET /api/v1/auth/google/callback
// Exchanges the code for tokens, creates/updates the user, and returns JWT tokens.
func (h *AuthHandler) GoogleCallback(c *fiber.Ctx) error {
	code  := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		return utils.RespondBadRequest(c, "missing code or state parameter")
	}

	// Validate CSRF state
	storedState := c.Cookies("oauth_state")
	if storedState == "" || storedState != state {
		return utils.RespondBadRequest(c, "invalid oauth state")
	}

	// Clear the state cookie
	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Expires:  time.Unix(0, 0),
		HTTPOnly: true,
		Secure:   h.cfg.Security.CookieSecure,
		SameSite: "Lax",
		Path:     "/",
	})

	resp, err := h.authSvc.HandleGoogleCallback(code, state, c.IP())
	if err != nil {
		return utils.RespondError(c, fiber.StatusBadGateway, "google authentication failed")
	}

	h.setRefreshCookie(c, resp.RefreshToken)

	// Redirect to frontend with tokens in fragment (SPA handles them)
	redirectURL := h.cfg.App.FrontendURL + "/auth/callback" +
		"?access_token=" + resp.AccessToken +
		"&expires_in=" + utils.Int64ToString(resp.ExpiresIn)
	return c.Redirect(redirectURL, fiber.StatusTemporaryRedirect)
}

// ── helpers ────────────────────────────────────────────────────────────────────

func (h *AuthHandler) setRefreshCookie(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Expires:  time.Now().Add(h.cfg.JWT.RefreshTokenExpiry),
		HTTPOnly: true,
		Secure:   h.cfg.Security.CookieSecure,
		SameSite: h.cfg.Security.CookieSameSite,
		Path:     "/api/v1/auth",
	})
}
