package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

// TokenType distinguishes access from refresh tokens.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// ZatranoClaims extends jwt.RegisteredClaims with app-specific fields.
type ZatranoClaims struct {
	UserID    uuid.UUID `json:"uid"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	TokenType TokenType `json:"type"`
	jwt.RegisteredClaims
}

// GenerateAccessToken creates a signed JWT access token for the given user.
func GenerateAccessToken(
	userID uuid.UUID,
	email string,
	roles []string,
	secret string,
	expiry time.Duration,
	issuer string,
) (string, string, error) {
	jti, err := generateJTI()
	if err != nil {
		return "", "", fmt.Errorf("generate jti: %w", err)
	}

	now := time.Now().UTC()
	claims := ZatranoClaims{
		UserID:    userID,
		Email:     email,
		Roles:     roles,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   userID.String(),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, jti, nil
}

// GenerateRefreshToken creates a signed JWT refresh token.
func GenerateRefreshToken(
	userID uuid.UUID,
	email string,
	secret string,
	expiry time.Duration,
	issuer string,
) (string, string, error) {
	jti, err := generateJTI()
	if err != nil {
		return "", "", fmt.Errorf("generate jti: %w", err)
	}

	now := time.Now().UTC()
	claims := ZatranoClaims{
		UserID:    userID,
		Email:     email,
		TokenType: TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   userID.String(),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", "", fmt.Errorf("sign refresh token: %w", err)
	}
	return signed, jti, nil
}

// ParseToken parses and validates a JWT string, returning the claims.
func ParseToken(tokenString, secret string) (*ZatranoClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&ZatranoClaims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(secret), nil
		},
	)
	if err != nil {
		var ve *jwt.ValidationError
		if errors.As(err, &ve) {
			if ve.Errors&jwt.ValidationErrorExpired != 0 {
				return nil, ErrTokenExpired
			}
		}
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*ZatranoClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

// GenerateSecureToken returns a cryptographically random hex-encoded token.
func GenerateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secure token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// generateJTI returns a unique JWT ID.
func generateJTI() (string, error) {
	return GenerateSecureToken(16)
}

// Token errors.
var (
	ErrTokenExpired = errors.New("token has expired")
	ErrTokenInvalid = errors.New("token is invalid")
)
