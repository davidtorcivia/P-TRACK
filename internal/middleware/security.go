package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// SecurityHeaders adds security headers to all responses
func SecurityHeaders(cspEnabled, hstsEnabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Content Security Policy
			if cspEnabled {
				// Generate nonce for inline scripts
				nonce := generateNonce()
				// Note: Nonce can be added to context in production
				// ctx := context.WithValue(r.Context(), cspNonceKey, nonce)
				// r = r.WithContext(ctx)

				csp := fmt.Sprintf(
					"default-src 'self'; "+
						"script-src 'self' 'nonce-%s' https://unpkg.com https://cdn.jsdelivr.net; "+
						"style-src 'self' 'unsafe-inline' https://unpkg.com https://cdn.jsdelivr.net; "+
						"img-src 'self' data: https:; "+
						"font-src 'self' data:; "+
						"connect-src 'self'; "+
						"frame-ancestors 'none'; "+
						"base-uri 'self'; "+
						"form-action 'self'",
					nonce,
				)
				w.Header().Set("Content-Security-Policy", csp)
			}

			// HTTP Strict Transport Security
			if hstsEnabled {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			// X-Frame-Options
			w.Header().Set("X-Frame-Options", "DENY")

			// X-Content-Type-Options
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// X-XSS-Protection (legacy but still useful)
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Referrer-Policy
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Permissions-Policy
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

			// Remove server header
			w.Header().Set("X-Powered-By", "")
			w.Header().Del("Server")

			next.ServeHTTP(w, r)
		})
	}
}

// CSRF protection middleware
type CSRFProtection struct {
	secret string
	tokens sync.Map // map[string]time.Time for token expiration
}

func NewCSRFProtection(secret string) *CSRFProtection {
	csrf := &CSRFProtection{
		secret: secret,
	}

	// Start cleanup goroutine
	go csrf.cleanupExpiredTokens()

	return csrf
}

func (c *CSRFProtection) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CSRF for GET, HEAD, OPTIONS (safe methods)
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Get CSRF token from header or form
		token := r.Header.Get("X-CSRF-Token")
		if token == "" {
			token = r.FormValue("csrf_token")
		}

		// Validate token
		if !c.ValidateToken(token) {
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (c *CSRFProtection) GenerateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	token := base64.URLEncoding.EncodeToString(b)

	// Store token with expiration
	c.tokens.Store(token, time.Now().Add(24*time.Hour))

	return token
}

func (c *CSRFProtection) ValidateToken(token string) bool {
	if token == "" {
		return false
	}

	expiry, ok := c.tokens.Load(token)
	if !ok {
		return false
	}

	expiryTime, ok := expiry.(time.Time)
	if !ok || time.Now().After(expiryTime) {
		c.tokens.Delete(token)
		return false
	}

	// Token is valid - delete it to enforce one-time use
	c.tokens.Delete(token)
	return true
}

func (c *CSRFProtection) cleanupExpiredTokens() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		c.tokens.Range(func(key, value interface{}) bool {
			if expiry, ok := value.(time.Time); ok && now.After(expiry) {
				c.tokens.Delete(key)
			}
			return true
		})
	}
}

// RateLimiter implements rate limiting per IP address
type RateLimiter struct {
	visitors map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

func NewRateLimiter(requestsPerWindow int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*rate.Limiter),
		rate:     rate.Limit(float64(requestsPerWindow) / window.Seconds()),
		burst:    requestsPerWindow,
	}

	// Start cleanup goroutine
	go rl.cleanupVisitors()

	return rl
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		limiter := rl.getLimiter(ip)

		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = limiter
	}

	return limiter
}

func (rl *RateLimiter) cleanupVisitors() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		// Remove visitors that haven't made requests recently
		for ip := range rl.visitors {
			delete(rl.visitors, ip)
		}
		rl.mu.Unlock()
	}
}

// getIP extracts the real IP address from the request
func getIP(r *http.Request) string {
	// Check X-Forwarded-For header (if behind proxy)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Take the first IP
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fallback to RemoteAddr
	return r.RemoteAddr
}

// generateNonce generates a random nonce for CSP
func generateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// SecureCompare performs constant-time comparison of two strings
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}