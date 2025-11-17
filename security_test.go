package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"injection-tracker/internal/auth"
	"injection-tracker/internal/database"
	"injection-tracker/internal/handlers"
	"injection-tracker/internal/middleware"
	"injection-tracker/internal/models"
	"injection-tracker/internal/repository"
)

// setupSecurityTestDB creates a test database with required schema
func setupSecurityTestDB(t *testing.T) *database.DB {
	tmpDir := t.TempDir()
	db, err := database.Open(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create minimal schema for security tests
	schema := `
		CREATE TABLE accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		INSERT INTO accounts (id, name) VALUES (1, 'Test Account');

		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			email TEXT,
			account_id INTEGER NOT NULL DEFAULT 1,
			role TEXT DEFAULT 'member',
			is_active BOOLEAN DEFAULT 1,
			failed_login_attempts INTEGER DEFAULT 0,
			locked_until TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP,
			FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
		);

		CREATE TABLE account_members (
			account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('owner', 'member')),
			joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			invited_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
			PRIMARY KEY (account_id, user_id),
			CONSTRAINT chk_unique_user UNIQUE(user_id)
		);

		CREATE TABLE account_invitations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			email TEXT NOT NULL COLLATE NOCASE,
			token_hash TEXT UNIQUE NOT NULL,
			invited_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('owner', 'member')),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP NOT NULL,
			accepted_at TIMESTAMP,
			accepted_by INTEGER REFERENCES users(id) ON DELETE SET NULL
		);

		CREATE TABLE audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			action TEXT NOT NULL,
			entity_type TEXT,
			entity_id INTEGER,
			details TEXT,
			ip_address TEXT,
			user_agent TEXT,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}

// TestSecurity_SQLInjectionPrevention tests SQL injection attempts
func TestSecurity_SQLInjectionPrevention(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()

	jwtManager := auth.NewJWTManager("test-secret", 1*time.Hour)

	// Test SQL injection attempts in login
	maliciousInputs := []string{
		"admin' OR '1'='1",
		"admin'--",
		"admin'; DROP TABLE users;--",
		"admin' /*",
		"' OR 1=1--",
		"admin' UNION SELECT * FROM users--",
		"' OR 'x'='x",
		"1' OR '1' = '1')) /*",
	}

	handler := handlers.HandleLogin(db, jwtManager)

	for _, maliciousInput := range maliciousInputs {
		t.Run("SQL Injection: "+maliciousInput, func(t *testing.T) {
			payload := map[string]string{
				"username": maliciousInput,
				"password": "password",
			}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Should return 401 Unauthorized, not expose SQL errors
			if w.Code != http.StatusUnauthorized && w.Code != http.StatusBadRequest {
				t.Errorf("Expected 401 or 400, got %d", w.Code)
			}

			// Response should not contain SQL error messages
			respBody := w.Body.String()
			sqlKeywords := []string{"SQL", "syntax", "database", "sqlite", "query"}
			for _, keyword := range sqlKeywords {
				if strings.Contains(strings.ToLower(respBody), strings.ToLower(keyword)) {
					t.Errorf("Response contains SQL keyword '%s': %s", keyword, respBody)
				}
			}
		})
	}

	// Verify database integrity - users table should still exist
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Errorf("Database integrity compromised: %v", err)
	}
}

// TestSecurity_XSSPrevention tests XSS attack prevention
func TestSecurity_XSSPrevention(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()

	// Test XSS attempts in registration
	xssPayloads := []string{
		"<script>alert('XSS')</script>",
		"<img src=x onerror=alert('XSS')>",
		"<svg/onload=alert('XSS')>",
		"javascript:alert('XSS')",
		"<iframe src='javascript:alert(1)'>",
		"\"><script>alert(String.fromCharCode(88,83,83))</script>",
	}

	handler := handlers.HandleRegister(db)

	for _, xssPayload := range xssPayloads {
		t.Run("XSS: "+xssPayload, func(t *testing.T) {
			payload := map[string]string{
				"username": xssPayload,
				"password": "ValidPassword123",
			}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Registration might succeed or fail, but response should be safe
			responseBody := w.Body.String()

			// Check that script tags are not executed in response
			if strings.Contains(responseBody, "<script>") && !strings.Contains(responseBody, "&lt;script&gt;") {
				t.Error("Response contains unescaped script tags")
			}
		})
	}
}

// TestSecurity_CSRFProtection tests CSRF token validation
func TestSecurity_CSRFProtection(t *testing.T) {
	csrf := middleware.NewCSRFProtection("test-secret")

	handler := csrf.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	t.Run("POST without CSRF token fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", w.Code)
		}
	})

	t.Run("POST with valid CSRF token succeeds", func(t *testing.T) {
		token := csrf.GenerateToken()

		req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
		req.Header.Set("X-CSRF-Token", token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})

	t.Run("CSRF token cannot be reused", func(t *testing.T) {
		token := csrf.GenerateToken()

		// First use
		req1 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
		req1.Header.Set("X-CSRF-Token", token)
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)

		if w1.Code != http.StatusOK {
			t.Errorf("First use: Expected 200, got %d", w1.Code)
		}

		// Second use should fail
		req2 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
		req2.Header.Set("X-CSRF-Token", token)
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)

		if w2.Code != http.StatusForbidden {
			t.Errorf("Second use: Expected 403, got %d", w2.Code)
		}
	})
}

// TestSecurity_RateLimiting tests rate limiting enforcement
func TestSecurity_RateLimiting(t *testing.T) {
	// Create rate limiter: 5 requests per second
	limiter := middleware.NewRateLimiter(5, 1*time.Second)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("Rate limit enforced after threshold", func(t *testing.T) {
		successCount := 0
		rateLimitedCount := 0

		// Make 10 requests rapidly
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.168.1.100:12345"
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				successCount++
			} else if w.Code == http.StatusTooManyRequests {
				rateLimitedCount++
			}
		}

		if successCount != 5 {
			t.Errorf("Expected 5 successful requests, got %d", successCount)
		}

		if rateLimitedCount != 5 {
			t.Errorf("Expected 5 rate-limited requests, got %d", rateLimitedCount)
		}
	})
}

// TestSecurity_AccountLockout tests account lockout after failed login attempts
func TestSecurity_AccountLockout(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	jwtManager := auth.NewJWTManager("test-secret", 1*time.Hour)

	// Create test user
	hashedPassword, _ := auth.HashPassword("correctpassword")
	userModel := &models.User{
		Username:     "testuser",
		PasswordHash: hashedPassword,
		IsActive:     true,
	}
	if err := userRepo.Create(userModel); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	handler := handlers.HandleLogin(db, jwtManager)

	t.Run("Account locked after 5 failed attempts", func(t *testing.T) {
		// Make 5 failed login attempts
		for i := 0; i < 5; i++ {
			payload := map[string]string{
				"username": "testuser",
				"password": "wrongpassword",
			}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
				t.Logf("Attempt %d: Got status %d", i+1, w.Code)
			}
		}

		// 6th attempt should be locked
		payload := map[string]string{
			"username": "testuser",
			"password": "correctpassword", // Even with correct password
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 (account locked), got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "locked") {
			t.Error("Response should indicate account is locked")
		}
	})
}

// TestSecurity_PasswordStrength tests password strength requirements
func TestSecurity_PasswordStrength(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		shouldFail  bool
	}{
		{"Too short", "pass", true},
		{"7 characters", "1234567", true},
		{"8 characters minimum", "12345678", false},
		{"Long password", "ThisIsAVeryLongPassword123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := auth.ValidatePasswordStrength(tt.password)

			if tt.shouldFail && err == nil {
				t.Error("Expected password validation to fail")
			}

			if !tt.shouldFail && err != nil {
				t.Errorf("Expected password validation to pass: %v", err)
			}
		})
	}
}

// TestSecurity_JWTValidation tests JWT token security
func TestSecurity_JWTValidation(t *testing.T) {
	jwtManager := auth.NewJWTManager("secret-key", 1*time.Hour)

	t.Run("Expired token rejected", func(t *testing.T) {
		shortManager := auth.NewJWTManager("secret-key", 1*time.Millisecond)
		token, _ := shortManager.GenerateToken(1, "testuser", 1, "owner")

		time.Sleep(10 * time.Millisecond)

		_, err := jwtManager.ValidateToken(token)
		if err != auth.ErrExpiredToken {
			t.Errorf("Expected ErrExpiredToken, got %v", err)
		}
	})

	t.Run("Tampered token rejected", func(t *testing.T) {
		token, _ := jwtManager.GenerateToken(1, "testuser", 1, "owner")

		// Tamper with token
		tamperedToken := token[:len(token)-5] + "XXXXX"

		_, err := jwtManager.ValidateToken(tamperedToken)
		if err != auth.ErrInvalidToken {
			t.Errorf("Expected ErrInvalidToken for tampered token, got %v", err)
		}
	})

	t.Run("Wrong secret rejected", func(t *testing.T) {
		manager1 := auth.NewJWTManager("secret1", 1*time.Hour)
		manager2 := auth.NewJWTManager("secret2", 1*time.Hour)

		token, _ := manager1.GenerateToken(1, "testuser", 1, "owner")

		_, err := manager2.ValidateToken(token)
		if err != auth.ErrInvalidToken {
			t.Errorf("Expected ErrInvalidToken for wrong secret, got %v", err)
		}
	})
}

// TestSecurity_InputValidation tests input validation and sanitization
func TestSecurity_InputValidation(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()

	handler := handlers.HandleRegister(db)

	tests := []struct {
		name           string
		username       string
		password       string
		expectedStatus int
	}{
		{"Empty username", "", "password123", http.StatusBadRequest},
		{"Empty password", "validuser", "", http.StatusBadRequest},
		{"Short username", "ab", "password123", http.StatusBadRequest},
		{"Long username", strings.Repeat("a", 100), "password123", http.StatusBadRequest},
		{"Valid input", "validuser", "password123", http.StatusCreated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]string{
				"username": tt.username,
				"password": tt.password,
			}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestSecurity_SecureHeaders tests security headers are set
func TestSecurity_SecureHeaders(t *testing.T) {
	handler := middleware.SecurityHeaders(true, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	requiredHeaders := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"X-XSS-Protection":       "1; mode=block",
		"Content-Security-Policy": "default-src 'self'",
	}

	for header, expectedValue := range requiredHeaders {
		value := w.Header().Get(header)
		if value == "" {
			t.Errorf("Header %s not set", header)
		}
		if expectedValue != "" && !strings.Contains(value, expectedValue) {
			t.Errorf("Header %s: expected to contain '%s', got '%s'", header, expectedValue, value)
		}
	}
}

// TestSecurity_SessionManagement tests session handling
func TestSecurity_SessionManagement(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()

	jwtManager := auth.NewJWTManager("test-secret", 2*time.Hour)

	// Create test user
	hashedPassword, _ := auth.HashPassword("password123")
	userRepo := repository.NewUserRepository(db)
	user := &models.User{
		Username:     "testuser",
		PasswordHash: hashedPassword,
		IsActive:     true,
		AccountID:    1,
		Role:         "owner",
	}
	userRepo.Create(user)

	// Add user to account_members table (required for login)
	_, err := db.Exec(`
		INSERT INTO account_members (account_id, user_id, role, joined_at)
		VALUES (1, ?, 'owner', CURRENT_TIMESTAMP)
	`, user.ID)
	if err != nil {
		t.Fatalf("Failed to add user to account_members: %v", err)
	}

	handler := handlers.HandleLogin(db, jwtManager)

	t.Run("Successful login sets secure cookie", func(t *testing.T) {
		payload := map[string]string{
			"username": "testuser",
			"password": "password123",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Login failed: %d", w.Code)
		}

		// Check cookie is set
		cookies := w.Result().Cookies()
		var authCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "auth_token" {
				authCookie = cookie
				break
			}
		}

		if authCookie == nil {
			t.Fatal("Auth cookie not set")
		}

		// Verify cookie security attributes
		if !authCookie.HttpOnly {
			t.Error("Cookie should be HttpOnly")
		}

		if !authCookie.Secure {
			t.Error("Cookie should be Secure")
		}

		if authCookie.SameSite != http.SameSiteStrictMode {
			t.Error("Cookie should have SameSite=Strict")
		}
	})
}

// TestSecurity_BcryptCost tests bcrypt cost factor
func TestSecurity_BcryptCost(t *testing.T) {
	password := "testpassword123"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Bcrypt hash format: $2a$cost$salt+hash
	// Extract cost
	parts := strings.Split(hash, "$")
	if len(parts) < 4 {
		t.Fatal("Invalid bcrypt hash format")
	}

	cost := parts[2]
	if cost != "12" {
		t.Errorf("Expected bcrypt cost 12, got %s", cost)
	}
}

// TestSecurity_NoInformationLeakage tests that errors don't leak sensitive info
func TestSecurity_NoInformationLeakage(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()

	jwtManager := auth.NewJWTManager("test-secret", 1*time.Hour)
	handler := handlers.HandleLogin(db, jwtManager)

	t.Run("Login errors don't reveal user existence", func(t *testing.T) {
		// Try non-existent user
		payload1 := map[string]string{
			"username": "nonexistentuser",
			"password": "password123",
		}
		body1, _ := json.Marshal(payload1)

		req1 := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(string(body1)))
		req1.Header.Set("Content-Type", "application/json")
		w1 := httptest.NewRecorder()

		handler.ServeHTTP(w1, req1)
		response1 := w1.Body.String()

		// Create user and try wrong password
		hashedPassword, _ := auth.HashPassword("correctpassword")
		userRepo := repository.NewUserRepository(db)
		userRepo.Create(&models.User{
			Username:     "existinguser",
			PasswordHash: hashedPassword,
			IsActive:     true,
		})

		payload2 := map[string]string{
			"username": "existinguser",
			"password": "wrongpassword",
		}
		body2, _ := json.Marshal(payload2)

		req2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(string(body2)))
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()

		handler.ServeHTTP(w2, req2)
		response2 := w2.Body.String()

		// Both should return same generic error
		if w1.Code != w2.Code {
			t.Errorf("Status codes differ: %d vs %d", w1.Code, w2.Code)
		}

		// Responses should be similar (not revealing which case)
		if !strings.Contains(response1, "Invalid username or password") ||
			!strings.Contains(response2, "Invalid username or password") {
			t.Error("Error messages should be generic and identical")
		}
	})
}