package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const (
	// Use cost factor 12 for bcrypt (as per security requirements)
	bcryptCost = 12
)

var (
	ErrInvalidPassword = errors.New("invalid password")
	ErrWeakPassword    = errors.New("password too weak")
)

// HashPassword generates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	// Validate password strength
	if err := ValidatePasswordStrength(password); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// VerifyPassword compares a password with its hash
func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// ValidatePasswordStrength checks if password meets minimum requirements
func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}

	// Additional password requirements can be added here:
	// - Uppercase letters
	// - Lowercase letters
	// - Numbers
	// - Special characters

	return nil
}

// GenerateResetToken generates a secure random token for password reset
func GenerateResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GenerateSessionToken generates a secure random token for session management
func GenerateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}