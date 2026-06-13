package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
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
						"script-src 'self' 'unsafe-inline' 'unsafe-eval' 'nonce-%s' https://unpkg.com https://cdn.jsdelivr.net; "+
						"style-src 'self' 'unsafe-inline' https://unpkg.com https://cdn.jsdelivr.net https://fonts.googleapis.com; "+
						"img-src 'self' data: https:; "+
						"font-src 'self' data: https://fonts.gstatic.com; "+
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

	// Token is valid - keep it for reuse within expiration window (SPA-friendly)
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
	visitors map[string]*visitor
	mu       sync.Mutex
	rate     rate.Limit
	burst    int
	window   time.Duration
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewRateLimiter(requestsPerWindow int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate.Limit(float64(requestsPerWindow) / window.Seconds()),
		burst:    requestsPerWindow,
		window:   window,
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

	v, exists := rl.visitors[ip]
	if !exists {
		v = &visitor{limiter: rate.NewLimiter(rl.rate, rl.burst)}
		rl.visitors[ip] = v
	}
	v.lastSeen = time.Now()

	return v.limiter
}

func (rl *RateLimiter) cleanupVisitors() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		// Only evict visitors idle for longer than the rate-limit window, so an
		// active attacker's bucket is never reset out from under the limiter.
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > rl.window {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// trustedProxies holds the parsed CIDRs whose forwarding headers we trust.
// It is set once at startup via SetTrustedProxies and read-only thereafter.
var trustedProxies []*net.IPNet

// SetTrustedProxies configures which proxy CIDRs may set X-Forwarded-For /
// X-Real-IP. Invalid entries are ignored. Call once during startup.
func SetTrustedProxies(cidrs []string) {
	parsed := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		// Allow bare IPs as well as CIDRs.
		if !strings.Contains(c, "/") {
			if strings.Contains(c, ":") {
				c += "/128"
			} else {
				c += "/32"
			}
		}
		if _, n, err := net.ParseCIDR(c); err == nil {
			parsed = append(parsed, n)
		}
	}
	trustedProxies = parsed
}

func isTrustedProxy(ip net.IP) bool {
	for _, n := range trustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIP returns the trusted client IP for the request, honoring forwarding
// headers only from configured trusted proxies. Exported for use by handlers
// (e.g. audit logging) so IP resolution is consistent across the app.
func ClientIP(r *http.Request) string {
	return getIP(r)
}

// getIP extracts the client IP address from the request. Forwarding headers
// (X-Forwarded-For / X-Real-IP) are honored only when the direct peer is a
// configured trusted proxy; otherwise they are attacker-controlled and ignored,
// preventing rate-limit / lockout bypass via header spoofing.
func getIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)

	if peer != nil && isTrustedProxy(peer) {
		// Trust the left-most (original client) entry of X-Forwarded-For.
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
			if first != "" {
				return first
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}

	return host
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
