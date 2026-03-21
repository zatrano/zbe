package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/zatrano/zbe/config"
	"github.com/zatrano/zbe/internal/domain"
	"github.com/zatrano/zbe/internal/repository"
	"github.com/zatrano/zbe/pkg/logger"
	"github.com/zatrano/zbe/pkg/mail"
	"github.com/zatrano/zbe/pkg/utils"
)

// AuthService handles all authentication operations.
type AuthService interface {
	Register(req *domain.RegisterRequest, ip string) (*domain.AuthResponse, error)
	Login(req *domain.LoginRequest, ip string) (*domain.AuthResponse, error)
	Logout(userID uuid.UUID, jti string, exp time.Time) error
	RefreshToken(refreshToken string) (*domain.AuthResponse, error)
	VerifyEmail(token string) error
	RequestPasswordReset(email string) error
	ConfirmPasswordReset(req *domain.PasswordResetConfirmDTO) error
	GetGoogleAuthURL(state string) string
	HandleGoogleCallback(code, state string, ip string) (*domain.AuthResponse, error)
}

type authService struct {
	userRepo   repository.UserRepository
	roleRepo   repository.RoleRepository
	tokenRepo  repository.TokenRepository
	oauthRepo  repository.OAuthRepository
	mailSvc    *mail.Service
	cfg        *config.Config
	googleCfg  *oauth2.Config
}

// NewAuthService constructs an AuthService with all dependencies.
func NewAuthService(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	tokenRepo repository.TokenRepository,
	oauthRepo repository.OAuthRepository,
	mailSvc *mail.Service,
	cfg *config.Config,
) AuthService {
	googleCfg := &oauth2.Config{
		ClientID:     cfg.OAuth.Google.ClientID,
		ClientSecret: cfg.OAuth.Google.ClientSecret,
		RedirectURL:  cfg.OAuth.Google.RedirectURL,
		Scopes:       cfg.OAuth.Google.Scopes,
		Endpoint:     google.Endpoint,
	}
	return &authService{
		userRepo:  userRepo,
		roleRepo:  roleRepo,
		tokenRepo: tokenRepo,
		oauthRepo: oauthRepo,
		mailSvc:   mailSvc,
		cfg:       cfg,
		googleCfg: googleCfg,
	}
}

// Register creates a new user account, assigns the default user role,
// and sends a verification email.
func (s *authService) Register(req *domain.RegisterRequest, ip string) (*domain.AuthResponse, error) {
	// Check for duplicate email
	exists, err := s.userRepo.ExistsByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	// Hash password
	hash, err := utils.HashPassword(req.Password, s.cfg.Security.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	// Generate verification token
	verifyToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate verify token: %w", err)
	}
	verifyExp := time.Now().Add(24 * time.Hour)

	// Find default user role
	userRole, err := s.roleRepo.FindByName(domain.RoleUser)
	if err != nil {
		logger.Warnf("default user role not found, proceeding without: %v", err)
	}

	user := &domain.User{
		Name:           req.Name,
		Email:          req.Email,
		PasswordHash:   hash,
		VerifyToken:    verifyToken,
		VerifyTokenExp: &verifyExp,
		IsActive:       true,
		LastLoginIP:    ip,
	}
	if userRole != nil {
		user.Roles = []domain.Role{*userRole}
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Send verification email asynchronously
	go func() {
		if err := s.mailSvc.SendVerificationEmail(user.Email, user.Name, verifyToken); err != nil {
			logger.Errorf("send verify email to %s: %v", user.Email, err)
		}
	}()

	return s.issueTokenPair(user)
}

// Login authenticates a user by email and password.
func (s *authService) Login(req *domain.LoginRequest, ip string) (*domain.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("find user: %w", err)
	}

	if !user.IsActive {
		return nil, ErrAccountDisabled
	}

	if err := utils.CheckPassword(req.Password, user.PasswordHash); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Update last login
	now := time.Now().UTC()
	_ = s.userRepo.UpdateFields(user.ID, map[string]interface{}{
		"last_login_at": now,
		"last_login_ip": ip,
	})
	user.LastLoginAt = &now
	user.LastLoginIP = ip

	return s.issueTokenPair(user)
}

// Logout revokes the given JTI so the token can no longer be used.
func (s *authService) Logout(userID uuid.UUID, jti string, exp time.Time) error {
	if jti == "" {
		return nil
	}
	if err := s.tokenRepo.RevokeJTI(jti, userID, exp); err != nil {
		return fmt.Errorf("revoke jti: %w", err)
	}
	// Also clear the refresh token from the DB
	_ = s.userRepo.UpdateFields(userID, map[string]interface{}{
		"refresh_token":     nil,
		"refresh_token_exp": nil,
	})
	return nil
}

// RefreshToken validates the refresh token and issues new access + refresh tokens.
func (s *authService) RefreshToken(refreshToken string) (*domain.AuthResponse, error) {
	claims, err := utils.ParseToken(refreshToken, s.cfg.JWT.Secret)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if claims.TokenType != utils.TokenTypeRefresh {
		return nil, ErrInvalidToken
	}

	// Check revocation
	revoked, err := s.tokenRepo.IsJTIRevoked(claims.ID)
	if err != nil {
		return nil, fmt.Errorf("check revocation: %w", err)
	}
	if revoked {
		return nil, ErrTokenRevoked
	}

	user, err := s.userRepo.FindByID(claims.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if !user.IsActive {
		return nil, ErrAccountDisabled
	}

	// Rotate: revoke old refresh token
	if err := s.tokenRepo.RevokeJTI(claims.ID, user.ID, claims.ExpiresAt.Time); err != nil {
		logger.Warnf("could not revoke old refresh jti %s: %v", claims.ID, err)
	}

	return s.issueTokenPair(user)
}

// VerifyEmail confirms a user's email using the provided token.
func (s *authService) VerifyEmail(token string) error {
	user, err := s.userRepo.FindByVerifyToken(token)
	if err != nil {
		return ErrInvalidToken
	}
	if user.EmailVerified {
		return nil // idempotent
	}
	if user.VerifyTokenExp != nil && user.VerifyTokenExp.Before(time.Now()) {
		return ErrTokenExpired
	}

	if err := s.userRepo.UpdateFields(user.ID, map[string]interface{}{
		"email_verified":   true,
		"verify_token":     nil,
		"verify_token_exp": nil,
	}); err != nil {
		return fmt.Errorf("verify email update: %w", err)
	}

	go func() {
		if err := s.mailSvc.SendWelcomeEmail(user.Email, user.Name); err != nil {
			logger.Errorf("send welcome email: %v", err)
		}
	}()

	return nil
}

// RequestPasswordReset generates a reset token and sends it by email.
func (s *authService) RequestPasswordReset(email string) error {
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		// Don't reveal whether the email exists
		return nil
	}

	// Invalidate existing reset tokens for this user
	_ = s.tokenRepo.InvalidateUserPasswordResetTokens(user.ID)

	rawToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return fmt.Errorf("generate reset token: %w", err)
	}

	prt := &domain.PasswordResetToken{
		UserID:    user.ID,
		Token:     rawToken,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := s.tokenRepo.CreatePasswordResetToken(prt); err != nil {
		return fmt.Errorf("save reset token: %w", err)
	}

	go func() {
		if err := s.mailSvc.SendPasswordReset(user.Email, user.Name, rawToken); err != nil {
			logger.Errorf("send password reset email: %v", err)
		}
	}()

	return nil
}

// ConfirmPasswordReset validates the reset token and updates the password.
func (s *authService) ConfirmPasswordReset(req *domain.PasswordResetConfirmDTO) error {
	prt, err := s.tokenRepo.FindPasswordResetToken(req.Token)
	if err != nil {
		return ErrInvalidToken
	}

	user, err := s.userRepo.FindByID(prt.UserID)
	if err != nil {
		return ErrInvalidToken
	}

	hash, err := utils.HashPassword(req.NewPassword, s.cfg.Security.BcryptCost)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	if err := s.userRepo.UpdateFields(user.ID, map[string]interface{}{
		"password_hash": hash,
	}); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	if err := s.tokenRepo.MarkPasswordResetTokenUsed(prt.ID); err != nil {
		logger.Warnf("mark reset token used: %v", err)
	}

	return nil
}

// GetGoogleAuthURL returns the OAuth2 redirect URL for Google sign-in.
func (s *authService) GetGoogleAuthURL(state string) string {
	return s.googleCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// HandleGoogleCallback exchanges the code for tokens, fetches the Google user profile,
// and creates or updates the local user record.
func (s *authService) HandleGoogleCallback(code, state string, ip string) (*domain.AuthResponse, error) {
	token, err := s.googleCfg.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("google code exchange: %w", err)
	}

	// Fetch user info from Google
	googleUser, err := fetchGoogleUserInfo(s.googleCfg, token)
	if err != nil {
		return nil, fmt.Errorf("fetch google user info: %w", err)
	}

	// Look for existing OAuth provider record
	existing, err := s.oauthRepo.FindByProviderID("google", googleUser.ID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("find oauth provider: %w", err)
	}

	var user *domain.User

	if existing != nil {
		// Existing OAuth link — load the user
		user, err = s.userRepo.FindByID(existing.UserID)
		if err != nil {
			return nil, fmt.Errorf("find user by oauth: %w", err)
		}
	} else {
		// Check if user exists with this email
		user, err = s.userRepo.FindByEmail(googleUser.Email)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("find user by email: %w", err)
		}
		if errors.Is(err, repository.ErrNotFound) || user == nil {
			// Create new user
			userRole, _ := s.roleRepo.FindByName(domain.RoleUser)
			user = &domain.User{
				Name:          googleUser.Name,
				Email:         googleUser.Email,
				EmailVerified: googleUser.EmailVerified,
				AvatarURL:     googleUser.Picture,
				IsActive:      true,
				LastLoginIP:   ip,
			}
			if userRole != nil {
				user.Roles = []domain.Role{*userRole}
			}
			if err := s.userRepo.Create(user); err != nil {
				return nil, fmt.Errorf("create oauth user: %w", err)
			}
		}
	}

	// Upsert OAuth provider record
	tokenExpiry := token.Expiry
	op := &domain.OAuthProvider{
		UserID:       user.ID,
		Provider:     "google",
		ProviderID:   googleUser.ID,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenExpiry:  &tokenExpiry,
		Email:        googleUser.Email,
		Name:         googleUser.Name,
		AvatarURL:    googleUser.Picture,
		RawData: domain.JSONB{
			"id":             googleUser.ID,
			"email":          googleUser.Email,
			"name":           googleUser.Name,
			"picture":        googleUser.Picture,
			"email_verified": googleUser.EmailVerified,
		},
	}
	if err := s.oauthRepo.Upsert(op); err != nil {
		logger.Warnf("oauth upsert: %v", err)
	}

	// Update last login
	now := time.Now().UTC()
	_ = s.userRepo.UpdateFields(user.ID, map[string]interface{}{
		"last_login_at": now,
		"last_login_ip": ip,
	})

	return s.issueTokenPair(user)
}

// ── Internal helpers ───────────────────────────────────────────────────────────

// issueTokenPair generates an access + refresh token pair and persists the refresh token.
func (s *authService) issueTokenPair(user *domain.User) (*domain.AuthResponse, error) {
	roleNames := user.GetRoleNames()

	accessToken, _, err := utils.GenerateAccessToken(
		user.ID, user.Email, roleNames,
		s.cfg.JWT.Secret, s.cfg.JWT.AccessTokenExpiry, s.cfg.JWT.Issuer,
	)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, _, err := utils.GenerateRefreshToken(
		user.ID, user.Email,
		s.cfg.JWT.Secret, s.cfg.JWT.RefreshTokenExpiry, s.cfg.JWT.Issuer,
	)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	// Persist refresh token (hashed or as-is — using plain for simplicity;
	// production systems would hash this)
	refreshExp := time.Now().Add(s.cfg.JWT.RefreshTokenExpiry)
	_ = s.userRepo.UpdateFields(user.ID, map[string]interface{}{
		"refresh_token":     refreshToken,
		"refresh_token_exp": refreshExp,
	})

	return &domain.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.cfg.JWT.AccessTokenExpiry.Seconds()),
		User:         domain.ToUserResponse(user),
	}, nil
}

// Service-level errors
var (
	ErrEmailAlreadyExists = errors.New("email address is already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountDisabled    = errors.New("account has been disabled")
	ErrInvalidToken       = errors.New("token is invalid or has expired")
	ErrTokenExpired       = errors.New("token has expired")
	ErrTokenRevoked       = errors.New("token has been revoked")
)
