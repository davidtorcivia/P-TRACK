package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewJWTManager(t *testing.T) {
	secret := "test-secret-key"
	duration := 24 * time.Hour

	manager := NewJWTManager(secret, duration)

	if manager == nil {
		t.Fatal("Expected non-nil JWTManager")
	}

	if string(manager.secret) != secret {
		t.Errorf("Expected secret %s, got %s", secret, string(manager.secret))
	}

	if manager.sessionDuration != duration {
		t.Errorf("Expected duration %v, got %v", duration, manager.sessionDuration)
	}
}

func TestGenerateToken(t *testing.T) {
	manager := NewJWTManager("test-secret", 2*time.Hour)

	tests := []struct {
		name     string
		userID   int64
		username string
	}{
		{
			name:     "Valid user",
			userID:   1,
			username: "testuser",
		},
		{
			name:     "Another valid user",
			userID:   999,
			username: "anotheruser",
		},
		{
			name:     "User with special characters",
			userID:   42,
			username: "user@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := manager.GenerateToken(tt.userID, tt.username, 1, "owner")
			if err != nil {
				t.Fatalf("Failed to generate token: %v", err)
			}

			if token == "" {
				t.Error("Generated empty token")
			}

			// Token should have 3 parts (header.payload.signature)
			parts, err := jwt.NewParser().Parse(token, func(token *jwt.Token) (interface{}, error) {
				return manager.secret, nil
			})
			if err != nil {
				t.Errorf("Token parsing failed: %v", err)
			}
			if parts == nil {
				t.Error("Token parsing returned nil")
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	secret := "test-secret-key"
	manager := NewJWTManager(secret, 2*time.Hour)

	userID := int64(123)
	username := "testuser"

	// Generate a valid token
	token, err := manager.GenerateToken(userID, username, 1, "owner")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	tests := []struct {
		name        string
		token       string
		expectError bool
		errorType   error
	}{
		{
			name:        "Valid token",
			token:       token,
			expectError: false,
		},
		{
			name:        "Empty token",
			token:       "",
			expectError: true,
			errorType:   ErrInvalidToken,
		},
		{
			name:        "Invalid token format",
			token:       "not.a.valid.token",
			expectError: true,
			errorType:   ErrInvalidToken,
		},
		{
			name:        "Malformed token",
			token:       "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid",
			expectError: true,
			errorType:   ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := manager.ValidateToken(tt.token)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if claims != nil {
					t.Error("Expected nil claims with error")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if claims == nil {
				t.Fatal("Expected non-nil claims")
			}

			if claims.UserID != userID {
				t.Errorf("Expected UserID %d, got %d", userID, claims.UserID)
			}

			if claims.Username != username {
				t.Errorf("Expected Username %s, got %s", username, claims.Username)
			}
		})
	}
}

func TestValidateTokenExpired(t *testing.T) {
	// Create manager with very short duration
	manager := NewJWTManager("test-secret", 1*time.Millisecond)

	token, err := manager.GenerateToken(1, "testuser", 1, "owner")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	claims, err := manager.ValidateToken(token)
	if err != ErrExpiredToken {
		t.Errorf("Expected ErrExpiredToken, got %v", err)
	}

	if claims != nil {
		t.Error("Expected nil claims for expired token")
	}
}

func TestValidateTokenWrongSecret(t *testing.T) {
	manager1 := NewJWTManager("secret1", 1*time.Hour)
	manager2 := NewJWTManager("secret2", 1*time.Hour)

	// Generate token with manager1
	token, err := manager1.GenerateToken(1, "testuser", 1, "owner")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Try to validate with manager2 (different secret)
	claims, err := manager2.ValidateToken(token)
	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken, got %v", err)
	}

	if claims != nil {
		t.Error("Expected nil claims for token with wrong secret")
	}
}

func TestRefreshToken(t *testing.T) {
	manager := NewJWTManager("test-secret", 2*time.Hour)

	userID := int64(123)
	username := "testuser"

	// Generate original token
	originalToken, err := manager.GenerateToken(userID, username, 1, "owner")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Get original claims
	originalClaims, err := manager.ValidateToken(originalToken)
	if err != nil {
		t.Fatalf("Failed to validate original token: %v", err)
	}

	// Wait to ensure new token has different timestamp (JWT uses second precision)
	time.Sleep(1100 * time.Millisecond)

	// Refresh token
	newToken, err := manager.RefreshToken(originalToken)
	if err != nil {
		t.Fatalf("Failed to refresh token: %v", err)
	}

	if newToken == "" {
		t.Error("Expected non-empty refreshed token")
	}

	// Tokens should be different if enough time has passed (>1 second for JWT precision)
	if newToken == originalToken {
		t.Error("Refreshed token should be different from original after waiting >1 second")
	}

	// Validate new token
	newClaims, err := manager.ValidateToken(newToken)
	if err != nil {
		t.Fatalf("Failed to validate refreshed token: %v", err)
	}

	if newClaims.UserID != userID {
		t.Errorf("Expected UserID %d, got %d", userID, newClaims.UserID)
	}

	if newClaims.Username != username {
		t.Errorf("Expected Username %s, got %s", username, newClaims.Username)
	}

	// Verify the new token has a later expiration time (purpose of refresh)
	if !newClaims.ExpiresAt.Time.After(originalClaims.ExpiresAt.Time) {
		t.Error("Refreshed token should have later expiration time than original")
	}
}

func TestRefreshTokenExpired(t *testing.T) {
	// Create manager with very short duration
	manager := NewJWTManager("test-secret", 1*time.Millisecond)

	token, err := manager.GenerateToken(1, "testuser", 1, "owner")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// Refresh should still work for expired tokens (grace period)
	newToken, err := manager.RefreshToken(token)
	if err != nil {
		t.Errorf("Refresh should work for expired tokens: %v", err)
	}

	if newToken == "" {
		t.Error("Expected non-empty refreshed token")
	}
}

func TestRefreshTokenInvalid(t *testing.T) {
	manager := NewJWTManager("test-secret", 1*time.Hour)

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "Empty token",
			token: "",
		},
		{
			name:  "Invalid format",
			token: "not.a.valid.token",
		},
		{
			name:  "Malformed token",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newToken, err := manager.RefreshToken(tt.token)
			if err == nil {
				t.Error("Expected error for invalid token")
			}

			if newToken != "" {
				t.Error("Expected empty token on error")
			}
		})
	}
}

func TestSessionDuration(t *testing.T) {
	duration := 24 * time.Hour
	manager := NewJWTManager("test-secret", duration)

	if manager.SessionDuration() != duration {
		t.Errorf("Expected duration %v, got %v", duration, manager.SessionDuration())
	}
}

func TestTokenClaims(t *testing.T) {
	manager := NewJWTManager("test-secret", 2*time.Hour)

	userID := int64(42)
	username := "testuser"

	token, err := manager.GenerateToken(userID, username, 1, "owner")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	claims, err := manager.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	// Check custom claims
	if claims.UserID != userID {
		t.Errorf("Expected UserID %d, got %d", userID, claims.UserID)
	}

	if claims.Username != username {
		t.Errorf("Expected Username %s, got %s", username, claims.Username)
	}

	// Check standard claims
	if claims.ExpiresAt == nil {
		t.Error("Expected ExpiresAt to be set")
	}

	if claims.IssuedAt == nil {
		t.Error("Expected IssuedAt to be set")
	}

	if claims.NotBefore == nil {
		t.Error("Expected NotBefore to be set")
	}

	// Check expiration is in the future
	if time.Until(claims.ExpiresAt.Time) <= 0 {
		t.Error("Token should not be expired")
	}

	// Check expiration is approximately sessionDuration away
	expectedExpiry := time.Now().Add(manager.sessionDuration)
	actualExpiry := claims.ExpiresAt.Time
	diff := actualExpiry.Sub(expectedExpiry).Abs()
	if diff > 1*time.Second {
		t.Errorf("Expiration time difference too large: %v", diff)
	}
}

func TestTokenSigningMethod(t *testing.T) {
	manager := NewJWTManager("test-secret", 1*time.Hour)

	token, err := manager.GenerateToken(1, "testuser", 1, "owner")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Parse token without validation to check signing method
	parsedToken, _, err := jwt.NewParser().ParseUnverified(token, &Claims{})
	if err != nil {
		t.Fatalf("Failed to parse token: %v", err)
	}

	if parsedToken.Method != jwt.SigningMethodHS256 {
		t.Errorf("Expected signing method HS256, got %v", parsedToken.Method)
	}
}

func TestTokenNotBeforeClaim(t *testing.T) {
	manager := NewJWTManager("test-secret", 1*time.Hour)

	token, err := manager.GenerateToken(1, "testuser", 1, "owner")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	claims, err := manager.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	// NotBefore should be now or in the past
	if claims.NotBefore == nil {
		t.Error("Expected NotBefore to be set")
	}

	if time.Until(claims.NotBefore.Time) > 1*time.Second {
		t.Error("NotBefore should be current time or past")
	}
}

// Benchmark tests
func BenchmarkGenerateToken(b *testing.B) {
	manager := NewJWTManager("benchmark-secret", 2*time.Hour)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GenerateToken(int64(i), "benchuser", 1, "owner")
	}
}

func BenchmarkValidateToken(b *testing.B) {
	manager := NewJWTManager("benchmark-secret", 2*time.Hour)
	token, _ := manager.GenerateToken(1, "benchuser", 1, "owner")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.ValidateToken(token)
	}
}

func BenchmarkRefreshToken(b *testing.B) {
	manager := NewJWTManager("benchmark-secret", 2*time.Hour)
	token, _ := manager.GenerateToken(1, "benchuser", 1, "owner")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.RefreshToken(token)
	}
}

// Test concurrent token operations
func TestConcurrentTokenOperations(t *testing.T) {
	manager := NewJWTManager("test-secret", 1*time.Hour)
	const goroutines = 100

	// Test concurrent token generation
	t.Run("Concurrent generation", func(t *testing.T) {
		done := make(chan bool, goroutines)
		for i := 0; i < goroutines; i++ {
			go func(id int) {
				_, err := manager.GenerateToken(int64(id), "user", 1, "owner")
				if err != nil {
					t.Errorf("Failed to generate token: %v", err)
				}
				done <- true
			}(i)
		}

		for i := 0; i < goroutines; i++ {
			<-done
		}
	})

	// Test concurrent token validation
	t.Run("Concurrent validation", func(t *testing.T) {
		token, _ := manager.GenerateToken(1, "testuser", 1, "owner")
		done := make(chan bool, goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				_, err := manager.ValidateToken(token)
				if err != nil {
					t.Errorf("Failed to validate token: %v", err)
				}
				done <- true
			}()
		}

		for i := 0; i < goroutines; i++ {
			<-done
		}
	})
}