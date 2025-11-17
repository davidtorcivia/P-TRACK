package middleware

import (
	"context"
	"net/http"
	"strings"

	"injection-tracker/internal/auth"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

// UserContext holds user information in the request context
type UserContext struct {
	UserID    int64
	Username  string
	AccountID int64  // Account the user belongs to
	Role      string // 'owner' or 'member'
}

// AuthMiddleware validates JWT tokens and adds user context
type AuthMiddleware struct {
	jwtManager *auth.JWTManager
}

func NewAuthMiddleware(jwtManager *auth.JWTManager) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager: jwtManager,
	}
}

// RequireAuth ensures the user is authenticated
func (am *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get token from cookie or Authorization header
		token := am.getToken(r)
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate token
		claims, err := am.jwtManager.ValidateToken(token)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Add user context
		userCtx := &UserContext{
			UserID:    claims.UserID,
			Username:  claims.Username,
			AccountID: claims.AccountID,
			Role:      claims.Role,
		}
		ctx := context.WithValue(r.Context(), UserContextKey, userCtx)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getToken extracts JWT token from request
func (am *AuthMiddleware) getToken(r *http.Request) string {
	// Try cookie first
	if cookie, err := r.Cookie("auth_token"); err == nil {
		return cookie.Value
	}

	// Try Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	return ""
}

// GetUserContext retrieves user context from request
func GetUserContext(r *http.Request) *UserContext {
	if userCtx, ok := r.Context().Value(UserContextKey).(*UserContext); ok {
		return userCtx
	}
	return nil
}

// GetUserID retrieves user ID from request context
func GetUserID(ctx context.Context) int64 {
	if userCtx, ok := ctx.Value(UserContextKey).(*UserContext); ok {
		return userCtx.UserID
	}
	return 0
}

// GetAccountID retrieves account ID from request context
func GetAccountID(ctx context.Context) int64 {
	if userCtx, ok := ctx.Value(UserContextKey).(*UserContext); ok {
		return userCtx.AccountID
	}
	return 0
}

// GetRole retrieves user role from request context
func GetRole(ctx context.Context) string {
	if userCtx, ok := ctx.Value(UserContextKey).(*UserContext); ok {
		return userCtx.Role
	}
	return ""
}