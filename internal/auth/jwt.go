package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

type Claims struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	AccountID int64  `json:"account_id"` // Account the user belongs to
	Role      string `json:"role"`       // 'owner' or 'member'
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret          []byte
	sessionDuration time.Duration
}

func NewJWTManager(secret string, sessionDuration time.Duration) *JWTManager {
	return &JWTManager{
		secret:          []byte(secret),
		sessionDuration: sessionDuration,
	}
}

// GenerateToken creates a new JWT token for a user
func (m *JWTManager) GenerateToken(userID int64, username string, accountID int64, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userID,
		Username:  username,
		AccountID: accountID,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.sessionDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ValidateToken validates a JWT token and returns the claims
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// RefreshToken generates a new token with extended expiration
func (m *JWTManager) RefreshToken(tokenString string) (string, error) {
	claims, err := m.ValidateToken(tokenString)
	if err != nil && !errors.Is(err, ErrExpiredToken) {
		return "", err
	}

	// If claims is nil (which can happen with expired tokens), we need to parse without validation
	if claims == nil {
		token, _, err := jwt.NewParser().ParseUnverified(tokenString, &Claims{})
		if err != nil {
			return "", ErrInvalidToken
		}
		var ok bool
		claims, ok = token.Claims.(*Claims)
		if !ok {
			return "", ErrInvalidToken
		}
	}

	// Generate new token with same claims but new expiration
	return m.GenerateToken(claims.UserID, claims.Username, claims.AccountID, claims.Role)
}

// SessionDuration returns the configured session duration
func (m *JWTManager) SessionDuration() time.Duration {
	return m.sessionDuration
}