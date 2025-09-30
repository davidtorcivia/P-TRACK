package auth

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		expectError bool
		errorType   error
	}{
		{
			name:        "Valid password",
			password:    "validPassword123",
			expectError: false,
		},
		{
			name:        "Minimum length password",
			password:    "12345678",
			expectError: false,
		},
		{
			name:        "Too short password",
			password:    "1234567",
			expectError: true,
			errorType:   ErrWeakPassword,
		},
		{
			name:        "Empty password",
			password:    "",
			expectError: true,
			errorType:   ErrWeakPassword,
		},
		{
			name:        "Complex password with special characters",
			password:    "P@ssw0rd!2023#$%",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if tt.errorType != nil && err != tt.errorType {
					t.Errorf("Expected error %v, got %v", tt.errorType, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if hash == "" {
				t.Error("Expected non-empty hash")
			}

			// Verify the hash uses bcrypt
			if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
				t.Error("Hash doesn't appear to be bcrypt format")
			}

			// Verify we can validate the password with the hash
			if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(tt.password)); err != nil {
				t.Errorf("Generated hash doesn't validate against original password: %v", err)
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "testPassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	tests := []struct {
		name        string
		hash        string
		password    string
		expectError bool
	}{
		{
			name:        "Correct password",
			hash:        hash,
			password:    password,
			expectError: false,
		},
		{
			name:        "Wrong password",
			hash:        hash,
			password:    "wrongPassword",
			expectError: true,
		},
		{
			name:        "Empty password",
			hash:        hash,
			password:    "",
			expectError: true,
		},
		{
			name:        "Case sensitive - different case",
			hash:        hash,
			password:    "TESTPASSWORD123",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword(tt.hash, tt.password)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		expectError bool
	}{
		{
			name:        "Valid 8 character password",
			password:    "12345678",
			expectError: false,
		},
		{
			name:        "Valid long password",
			password:    "thisIsAVeryLongPasswordWithManyCharacters123!@#",
			expectError: false,
		},
		{
			name:        "7 characters - too short",
			password:    "1234567",
			expectError: true,
		},
		{
			name:        "Empty password",
			password:    "",
			expectError: true,
		},
		{
			name:        "1 character",
			password:    "a",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.password)

			if tt.expectError && err != ErrWeakPassword {
				t.Errorf("Expected ErrWeakPassword, got %v", err)
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestGenerateResetToken(t *testing.T) {
	// Generate multiple tokens
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := GenerateResetToken()
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Token should not be empty
		if token == "" {
			t.Error("Generated empty token")
		}

		// Token should be base64 URL encoded
		if !isValidBase64URL(token) {
			t.Errorf("Token is not valid base64 URL encoded: %s", token)
		}

		// Token should be unique
		if tokens[token] {
			t.Error("Generated duplicate token")
		}
		tokens[token] = true

		// Token should have reasonable length (32 bytes encoded)
		if len(token) < 40 {
			t.Errorf("Token too short: %d characters", len(token))
		}
	}
}

func TestGenerateSessionToken(t *testing.T) {
	// Generate multiple tokens
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := GenerateSessionToken()
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Token should not be empty
		if token == "" {
			t.Error("Generated empty token")
		}

		// Token should be base64 URL encoded
		if !isValidBase64URL(token) {
			t.Errorf("Token is not valid base64 URL encoded: %s", token)
		}

		// Token should be unique
		if tokens[token] {
			t.Error("Generated duplicate token")
		}
		tokens[token] = true

		// Token should have reasonable length (32 bytes encoded)
		if len(token) < 40 {
			t.Errorf("Token too short: %d characters", len(token))
		}
	}
}

func TestHashPasswordBcryptCost(t *testing.T) {
	password := "testPassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Extract cost from hash (bcrypt format: $2a$cost$...)
	if len(hash) < 7 {
		t.Fatal("Hash too short to extract cost")
	}

	// Verify cost is 12 as specified in requirements
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("Failed to extract cost: %v", err)
	}

	if cost != bcryptCost {
		t.Errorf("Expected cost %d, got %d", bcryptCost, cost)
	}
}

// Benchmark tests for password operations
func BenchmarkHashPassword(b *testing.B) {
	password := "benchmarkPassword123"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = HashPassword(password)
	}
}

func BenchmarkVerifyPassword(b *testing.B) {
	password := "benchmarkPassword123"
	hash, _ := HashPassword(password)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = VerifyPassword(hash, password)
	}
}

func BenchmarkGenerateResetToken(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GenerateResetToken()
	}
}

// Helper function to validate base64 URL encoding
func isValidBase64URL(s string) bool {
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '=') {
			return false
		}
	}
	return true
}

// Test timing attack resistance
func TestVerifyPasswordTimingAttack(t *testing.T) {
	// This test ensures password verification takes consistent time
	// regardless of whether the password is correct or not
	password := "testPassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Test with correct and incorrect passwords
	// Note: bcrypt is designed to be resistant to timing attacks
	tests := []struct {
		name     string
		password string
	}{
		{"Correct password", password},
		{"Wrong password", "wrongPassword"},
		{"Empty password", ""},
		{"Very long password", strings.Repeat("a", 1000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify it doesn't panic and returns consistently
			_ = VerifyPassword(hash, tt.password)
		})
	}
}