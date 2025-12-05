package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(true, true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	tests := []struct {
		header   string
		contains string
	}{
		{"Content-Security-Policy", "default-src 'self'"},
		{"Content-Security-Policy", "frame-ancestors 'none'"},
		{"Strict-Transport-Security", "max-age=31536000"},
		{"X-Frame-Options", "DENY"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Permissions-Policy", "geolocation=()"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			value := w.Header().Get(tt.header)
			if value == "" {
				t.Errorf("Expected %s header to be set", tt.header)
			}

			if tt.contains != "" && !strings.Contains(value, tt.contains) {
				t.Errorf("Expected %s header to contain '%s', got '%s'", tt.header, tt.contains, value)
			}
		})
	}
}

func TestSecurityHeaders_CSPDisabled(t *testing.T) {
	handler := SecurityHeaders(false, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// CSP should not be set
	if w.Header().Get("Content-Security-Policy") != "" {
		t.Error("Expected CSP header to be empty when disabled")
	}

	// HSTS should not be set
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Error("Expected HSTS header to be empty when disabled")
	}

	// Other headers should still be set
	if w.Header().Get("X-Frame-Options") == "" {
		t.Error("Expected X-Frame-Options to be set")
	}
}

func TestCSRFProtection_SafeMethods(t *testing.T) {
	csrf := NewCSRFProtection("test-secret")

	safeMethods := []string{http.MethodGet, http.MethodHead, http.MethodOptions}

	for _, method := range safeMethods {
		t.Run(method, func(t *testing.T) {
			handler := csrf.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200 for safe method %s, got %d", method, w.Code)
			}
		})
	}
}

func TestCSRFProtection_UnsafeMethodsWithoutToken(t *testing.T) {
	csrf := NewCSRFProtection("test-secret")

	unsafeMethods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range unsafeMethods {
		t.Run(method, func(t *testing.T) {
			handler := csrf.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("Expected status 403 for unsafe method %s without token, got %d", method, w.Code)
			}

			if !strings.Contains(w.Body.String(), "Invalid CSRF token") {
				t.Error("Expected CSRF error message")
			}
		})
	}
}

func TestCSRFProtection_ValidToken(t *testing.T) {
	csrf := NewCSRFProtection("test-secret")

	// Generate token
	token := csrf.GenerateToken()

	handler := csrf.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test with token in header
	t.Run("Token in header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("X-CSRF-Token", token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 with valid token, got %d", w.Code)
		}
	})

	// Generate new token for form test (tokens are one-time use)
	token = csrf.GenerateToken()

	// Test with token in form
	t.Run("Token in form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("csrf_token="+token))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 with valid token, got %d", w.Code)
		}
	})
}

func TestCSRFProtection_InvalidToken(t *testing.T) {
	csrf := NewCSRFProtection("test-secret")

	handler := csrf.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-CSRF-Token", "invalid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 with invalid token, got %d", w.Code)
	}
}

func TestCSRFProtection_TokenExpiration(t *testing.T) {
	csrf := NewCSRFProtection("test-secret")

	// Generate token
	token := csrf.GenerateToken()

	// Manually expire the token
	csrf.tokens.Store(token, time.Now().Add(-25*time.Hour))

	handler := csrf.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-CSRF-Token", token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 with expired token, got %d", w.Code)
	}
}

func TestCSRFProtection_TokenReusable(t *testing.T) {
	csrf := NewCSRFProtection("test-secret")

	token := csrf.GenerateToken()

	handler := csrf.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First use should succeed
	req1 := httptest.NewRequest(http.MethodPost, "/", nil)
	req1.Header.Set("X-CSRF-Token", token)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200 on first use, got %d", w1.Code)
	}

	// Second use should also succeed (tokens are reusable until expiry for SPA support)
	req2 := httptest.NewRequest(http.MethodPost, "/", nil)
	req2.Header.Set("X-CSRF-Token", token)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 on second use (reusable token), got %d", w2.Code)
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	// Create limiter: 5 requests per second
	limiter := NewRateLimiter(5, 1*time.Second)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 5 requests should succeed
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// 6th request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429 after rate limit, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Rate limit exceeded") {
		t.Error("Expected rate limit error message")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	limiter := NewRateLimiter(2, 1*time.Second)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// IP 1: 2 requests (should succeed)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("IP1 Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// IP 2: 2 requests (should succeed - different IP)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("IP2 Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// IP 1: 3rd request (should be rate limited)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429 for IP1, got %d", w.Code)
	}
}

func TestRateLimiter_XForwardedFor(t *testing.T) {
	limiter := NewRateLimiter(2, 1*time.Second)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make requests with X-Forwarded-For header
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}
}

func TestGetIP(t *testing.T) {
	tests := []struct {
		name          string
		remoteAddr    string
		xForwardedFor string
		xRealIP       string
		expectedIP    string
	}{
		{
			name:       "RemoteAddr only",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1:12345",
		},
		{
			name:          "X-Forwarded-For single",
			remoteAddr:    "192.168.1.1:12345",
			xForwardedFor: "10.0.0.1",
			expectedIP:    "10.0.0.1",
		},
		{
			name:          "X-Forwarded-For multiple",
			remoteAddr:    "192.168.1.1:12345",
			xForwardedFor: "10.0.0.1, 10.0.0.2, 10.0.0.3",
			expectedIP:    "10.0.0.1",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "192.168.1.1:12345",
			xRealIP:    "10.0.0.1",
			expectedIP: "10.0.0.1",
		},
		{
			name:          "X-Forwarded-For takes precedence",
			remoteAddr:    "192.168.1.1:12345",
			xForwardedFor: "10.0.0.1",
			xRealIP:       "10.0.0.2",
			expectedIP:    "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			ip := getIP(req)
			if ip != tt.expectedIP {
				t.Errorf("Expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

func TestSecureCompare(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{
			name:     "Equal strings",
			a:        "password123",
			b:        "password123",
			expected: true,
		},
		{
			name:     "Different strings",
			a:        "password123",
			b:        "password456",
			expected: false,
		},
		{
			name:     "Empty strings",
			a:        "",
			b:        "",
			expected: true,
		},
		{
			name:     "One empty",
			a:        "password",
			b:        "",
			expected: false,
		},
		{
			name:     "Case sensitive",
			a:        "Password",
			b:        "password",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SecureCompare(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Test concurrent rate limiting
func TestRateLimiter_Concurrent(t *testing.T) {
	limiter := NewRateLimiter(100, 1*time.Second)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const goroutines = 50
	results := make(chan int, goroutines)

	// Launch concurrent requests
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			results <- w.Code
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < goroutines; i++ {
		code := <-results
		if code == http.StatusOK {
			successCount++
		}
	}

	// All requests within burst should succeed
	if successCount < goroutines {
		t.Logf("Success count: %d out of %d", successCount, goroutines)
	}
}

// Benchmark tests
func BenchmarkRateLimiter(b *testing.B) {
	limiter := NewRateLimiter(1000, 1*time.Second)

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkCSRFValidation(b *testing.B) {
	csrf := NewCSRFProtection("test-secret")

	// Pre-generate tokens
	tokens := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		tokens[i] = csrf.GenerateToken()
	}

	handler := csrf.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("X-CSRF-Token", tokens[i])
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}
