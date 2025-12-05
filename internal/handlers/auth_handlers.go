package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"injection-tracker/internal/auth"
	"injection-tracker/internal/database"
	"injection-tracker/internal/middleware"
	"injection-tracker/internal/models"
	"injection-tracker/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

const (
	MaxFailedAttempts   = 5
	LockoutDurationMins = 15
	BcryptCost          = 12
)

// LoginRequest represents the login request payload
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterRequest represents the registration request payload
type RegisterRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Email       string `json:"email,omitempty"`
	InviteToken string `json:"invite_token,omitempty"` // For joining existing account
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	Success bool          `json:"success"`
	Message string        `json:"message,omitempty"`
	User    *UserResponse `json:"user,omitempty"`
	Token   string        `json:"token,omitempty"`
}

// UserResponse represents user data in responses
type UserResponse struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	CreatedAt string `json:"created_at"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// HandleLogin handles user login with account lockout protection
func HandleLogin(db *database.DB, jwtManager *auth.JWTManager) http.HandlerFunc {
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)

	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest

		// Parse request - support both JSON and form data
		contentType := r.Header.Get("Content-Type")
		if contentType == "application/json" {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondErrorWithRequest(w, r, http.StatusBadRequest, "Invalid request body")
				return
			}
		} else {
			// Parse as form data
			if err := r.ParseForm(); err != nil {
				respondErrorWithRequest(w, r, http.StatusBadRequest, "Invalid form data")
				return
			}
			req.Username = r.FormValue("username")
			req.Password = r.FormValue("password")
		}

		// Validate input
		if req.Username == "" || req.Password == "" {
			respondErrorWithRequest(w, r, http.StatusBadRequest, "Username and password are required")
			return
		}

		ipAddress := getIPAddress(r)
		userAgent := r.Header.Get("User-Agent")

		// Get user by username
		user, err := userRepo.GetByUsername(req.Username)
		if err == repository.ErrNotFound {
			// Don't reveal that user doesn't exist - use same error as invalid password
			_ = auditRepo.LogWithDetails(
				sql.NullInt64{Valid: false},
				"login_failed",
				"user",
				sql.NullInt64{Valid: false},
				map[string]interface{}{"reason": "user_not_found", "username": req.Username},
				ipAddress,
				userAgent,
			)
			respondErrorWithRequest(w, r, http.StatusUnauthorized, "Invalid username or password")
			return
		}
		if err != nil {
			respondErrorWithRequest(w, r, http.StatusInternalServerError, "An error occurred")
			return
		}

		// Check if account is active
		if !user.IsActive {
			_ = auditRepo.LogWithDetails(
				sql.NullInt64{Int64: user.ID, Valid: true},
				"login_failed",
				"user",
				sql.NullInt64{Int64: user.ID, Valid: true},
				map[string]interface{}{"reason": "account_inactive"},
				ipAddress,
				userAgent,
			)
			respondErrorWithRequest(w, r, http.StatusForbidden, "Account is inactive")
			return
		}

		// Check if account is locked
		isLocked, err := userRepo.IsAccountLocked(user.ID)
		if err != nil {
			respondErrorWithRequest(w, r, http.StatusInternalServerError, "An error occurred")
			return
		}
		if isLocked {
			_ = auditRepo.LogWithDetails(
				sql.NullInt64{Int64: user.ID, Valid: true},
				"login_failed",
				"user",
				sql.NullInt64{Int64: user.ID, Valid: true},
				map[string]interface{}{"reason": "account_locked"},
				ipAddress,
				userAgent,
			)
			respondErrorWithRequest(w, r, http.StatusForbidden, fmt.Sprintf("Account is locked due to too many failed login attempts. Please try again in %d minutes.", LockoutDurationMins))
			return
		}

		// Verify password
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			// Increment failed attempts
			if err := userRepo.IncrementFailedLogins(user.ID); err != nil {
				// Log error but continue with response
				fmt.Printf("Error incrementing failed logins: %v\n", err)
			}

			// Check if we need to lock the account
			user.FailedLoginAttempts++
			if user.FailedLoginAttempts >= MaxFailedAttempts {
				lockUntil := time.Now().Add(LockoutDurationMins * time.Minute)
				if err := userRepo.LockAccount(user.ID, lockUntil); err != nil {
					fmt.Printf("Error locking account: %v\n", err)
				}

				_ = auditRepo.LogWithDetails(
					sql.NullInt64{Int64: user.ID, Valid: true},
					"account_locked",
					"user",
					sql.NullInt64{Int64: user.ID, Valid: true},
					map[string]interface{}{"reason": "max_failed_attempts", "attempts": user.FailedLoginAttempts},
					ipAddress,
					userAgent,
				)

				respondErrorWithRequest(w, r, http.StatusForbidden, fmt.Sprintf("Account locked due to too many failed login attempts. Please try again in %d minutes.", LockoutDurationMins))
				return
			}

			_ = auditRepo.LogWithDetails(
				sql.NullInt64{Int64: user.ID, Valid: true},
				"login_failed",
				"user",
				sql.NullInt64{Int64: user.ID, Valid: true},
				map[string]interface{}{"reason": "invalid_password", "attempts": user.FailedLoginAttempts},
				ipAddress,
				userAgent,
			)

			respondErrorWithRequest(w, r, http.StatusUnauthorized, "Invalid username or password")
			return
		}

		// Successful login - reset failed attempts
		if err := userRepo.ResetFailedLogins(user.ID); err != nil {
			fmt.Printf("Error resetting failed logins: %v\n", err)
		}

		// Update last login timestamp
		if err := userRepo.UpdateLastLogin(user.ID); err != nil {
			fmt.Printf("Error updating last login: %v\n", err)
		}

		// Get user's account
		accountRepo := repository.NewAccountRepository(db.DB)
		account, err := accountRepo.GetUserAccount(user.ID)
		if err != nil {
			_ = auditRepo.LogWithDetails(
				sql.NullInt64{Int64: user.ID, Valid: true},
				"login_failed",
				"user",
				sql.NullInt64{Int64: user.ID, Valid: true},
				map[string]interface{}{"reason": "no_account_found"},
				ipAddress,
				userAgent,
			)
			respondErrorWithRequest(w, r, http.StatusInternalServerError, "User account not properly configured. Please contact support.")
			return
		}

		// Get user's role in the account
		member, err := accountRepo.GetMember(account.ID, user.ID)
		if err != nil {
			respondErrorWithRequest(w, r, http.StatusInternalServerError, "Failed to retrieve account membership")
			return
		}

		// Generate JWT token with account info
		token, err := jwtManager.GenerateToken(user.ID, user.Username, account.ID, member.Role)
		if err != nil {
			respondErrorWithRequest(w, r, http.StatusInternalServerError, "Failed to generate authentication token")
			return
		}

		// Set HTTP-only cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    token,
			Path:     "/",
			MaxAge:   int(jwtManager.SessionDuration().Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})

		// Log successful login
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: user.ID, Valid: true},
			"login_success",
			"user",
			sql.NullInt64{Int64: user.ID, Valid: true},
			nil,
			ipAddress,
			userAgent,
		)

		// Respond based on request type
		if r.Header.Get("HX-Request") == "true" {
			// HTMX request - redirect to dashboard
			w.Header().Set("HX-Redirect", "/dashboard")
			w.WriteHeader(http.StatusOK)
		} else {
			// Standard JSON API response
			respondJSON(w, http.StatusOK, AuthResponse{
				Success: true,
				Message: "Login successful",
				User: &UserResponse{
					ID:        user.ID,
					Username:  user.Username,
					Email:     user.Email.String,
					CreatedAt: user.CreatedAt.Format(time.RFC3339),
				},
				Token: token,
			})
		}
	}
}

// HandleRegister handles user registration
func HandleRegister(db *database.DB) http.HandlerFunc {
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)

	return func(w http.ResponseWriter, r *http.Request) {
		// Parse request - support both JSON and form data
		var req RegisterRequest
		contentType := r.Header.Get("Content-Type")
		if contentType == "application/json" {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				respondErrorWithRequest(w, r, http.StatusBadRequest, "Invalid request body")
				return
			}
		} else {
			// Parse as form data (for HTMX)
			if err := r.ParseForm(); err != nil {
				respondErrorWithRequest(w, r, http.StatusBadRequest, "Invalid form data")
				return
			}
			req.Username = r.FormValue("username")
			req.Password = r.FormValue("password")
			req.Email = r.FormValue("email")
			req.InviteToken = r.FormValue("invite_token")
		}

		ipAddress := getIPAddress(r)
		userAgent := r.Header.Get("User-Agent")

		// Validate input
		if req.Username == "" || req.Password == "" {
			respondErrorWithRequest(w, r, http.StatusBadRequest, "Username and password are required")
			return
		}

		// Validate username length (matches DB constraint)
		if len(req.Username) < 3 || len(req.Username) > 50 {
			respondErrorWithRequest(w, r, http.StatusBadRequest, "Username must be between 3 and 50 characters")
			return
		}

		// Validate password strength
		if len(req.Password) < 8 {
			respondErrorWithRequest(w, r, http.StatusBadRequest, "Password must be at least 8 characters long")
			return
		}

		// Validate email format if provided
		if req.Email != "" && !strings.Contains(req.Email, "@") {
			respondErrorWithRequest(w, r, http.StatusBadRequest, "Invalid email format")
			return
		}

		// Check if username already exists
		existingUser, err := userRepo.GetByUsername(req.Username)
		if err == nil && existingUser != nil {
			_ = auditRepo.LogWithDetails(
				sql.NullInt64{Valid: false},
				"registration_failed",
				"user",
				sql.NullInt64{Valid: false},
				map[string]interface{}{"reason": "username_taken", "username": req.Username},
				ipAddress,
				userAgent,
			)
			respondErrorWithRequest(w, r, http.StatusConflict, "Username already exists")
			return
		}
		if err != nil && err != repository.ErrNotFound {
			respondErrorWithRequest(w, r, http.StatusInternalServerError, "An error occurred")
			return
		}

		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), BcryptCost)
		if err != nil {
			respondErrorWithRequest(w, r, http.StatusInternalServerError, "Failed to process password")
			return
		}

		// Create user
		user := &models.User{
			Username:     req.Username,
			PasswordHash: string(hashedPassword),
			IsActive:     true,
		}

		if req.Email != "" {
			user.Email = sql.NullString{String: req.Email, Valid: true}
		}

		if err := userRepo.Create(user); err != nil {
			// Check if it's a unique constraint violation (duplicate username)
			if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
				_ = auditRepo.LogWithDetails(
					sql.NullInt64{Valid: false},
					"registration_failed",
					"user",
					sql.NullInt64{Valid: false},
					map[string]interface{}{"reason": "username_taken", "username": req.Username},
					ipAddress,
					userAgent,
				)
				respondErrorWithRequest(w, r, http.StatusConflict, "Username already exists")
				return
			}
			respondErrorWithRequest(w, r, http.StatusInternalServerError, "Failed to create user")
			return
		}

		// Create or join account
		accountRepo := repository.NewAccountRepository(db.DB)
		var accountID int64

		// Check if registering with an invitation
		if req.InviteToken != "" {
			// Validate and accept invitation
			invitation, err := accountRepo.GetInvitationByToken(req.InviteToken)
			if err != nil {
				// Rollback: Delete the user if invitation is invalid
				_ = userRepo.Delete(user.ID)
				_ = auditRepo.LogWithDetails(
					sql.NullInt64{Int64: user.ID, Valid: true},
					"registration_failed",
					"user",
					sql.NullInt64{Int64: user.ID, Valid: true},
					map[string]interface{}{"reason": "invalid_invitation"},
					ipAddress,
					userAgent,
				)
				respondErrorWithRequest(w, r, http.StatusBadRequest, "Invalid or expired invitation")
				return
			}

			// Check if invitation is expired
			if time.Now().After(invitation.ExpiresAt) {
				_ = userRepo.Delete(user.ID)
				respondErrorWithRequest(w, r, http.StatusBadRequest, "Invitation has expired")
				return
			}

			// Check if already accepted
			if invitation.AcceptedAt.Valid {
				_ = userRepo.Delete(user.ID)
				respondErrorWithRequest(w, r, http.StatusBadRequest, "Invitation has already been used")
				return
			}

			// Accept the invitation
			if err := accountRepo.AcceptInvitation(invitation.ID, user.ID); err != nil {
				_ = userRepo.Delete(user.ID)
				_ = auditRepo.LogWithDetails(
					sql.NullInt64{Int64: user.ID, Valid: true},
					"registration_failed",
					"user",
					sql.NullInt64{Int64: user.ID, Valid: true},
					map[string]interface{}{"reason": "invitation_accept_failed"},
					ipAddress,
					userAgent,
				)
				respondErrorWithRequest(w, r, http.StatusInternalServerError, "Failed to accept invitation")
				return
			}

			accountID = invitation.AccountID

			_ = auditRepo.LogWithDetails(
				sql.NullInt64{Int64: user.ID, Valid: true},
				"registration_success",
				"user",
				sql.NullInt64{Int64: user.ID, Valid: true},
				map[string]interface{}{"account_id": accountID, "via_invitation": true},
				ipAddress,
				userAgent,
			)
		} else {
			// No invitation - create new account
			var err error
			accountID, err = accountRepo.Create(nil, user.ID) // nil = no custom account name
			if err != nil {
				// Rollback: Delete the user if account creation fails
				_ = userRepo.Delete(user.ID)
				_ = auditRepo.LogWithDetails(
					sql.NullInt64{Int64: user.ID, Valid: true},
					"registration_failed",
					"user",
					sql.NullInt64{Int64: user.ID, Valid: true},
					map[string]interface{}{"reason": "account_creation_failed"},
					ipAddress,
					userAgent,
				)
				respondErrorWithRequest(w, r, http.StatusInternalServerError, "Failed to create account")
				return
			}

			_ = auditRepo.LogWithDetails(
				sql.NullInt64{Int64: user.ID, Valid: true},
				"registration_success",
				"user",
				sql.NullInt64{Int64: user.ID, Valid: true},
				map[string]interface{}{"account_id": accountID},
				ipAddress,
				userAgent,
			)
		}

		// Log successful registration
		auditRepo.LogWithDetails(
			sql.NullInt64{Int64: user.ID, Valid: true},
			"registration_success",
			"user",
			sql.NullInt64{Int64: user.ID, Valid: true},
			map[string]interface{}{"account_id": accountID},
			ipAddress,
			userAgent,
		)

		// Continue with original audit log
		auditRepo.LogWithDetails(
			sql.NullInt64{Int64: user.ID, Valid: true},
			"registration_success",
			"user",
			sql.NullInt64{Int64: user.ID, Valid: true},
			map[string]interface{}{"username": user.Username},
			ipAddress,
			userAgent,
		)

		// Respond with success
		if r.Header.Get("HX-Request") == "true" {
			// HTMX request - show success message then redirect
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			successHTML := `
<div role="alert" style="
	background-color: #d4edda;
	border: 2px solid #28a745;
	border-radius: 8px;
	padding: 1rem 1.25rem;
	margin-bottom: 1.5rem;
	box-shadow: 0 2px 4px rgba(0,0,0,0.1);
">
	<div style="display: flex; align-items: start; gap: 0.75rem;">
		<span style="font-size: 1.5rem; line-height: 1;">✓</span>
		<div style="flex: 1;">
			<strong style="color: #28a745; font-size: 1rem; display: block; margin-bottom: 0.25rem;">Success!</strong>
			<p style="color: #155724; margin: 0; font-size: 0.95rem; line-height: 1.5;">
				Account created successfully. Redirecting to login...
			</p>
		</div>
	</div>
</div>
<script>
	setTimeout(function() {
		window.location.href = "/login?registered=true";
	}, 1500);
</script>`
			fmt.Fprint(w, successHTML)
		} else {
			// Standard JSON API response
			respondJSON(w, http.StatusCreated, AuthResponse{
				Success: true,
				Message: "Registration successful",
				User: &UserResponse{
					ID:        user.ID,
					Username:  user.Username,
					Email:     user.Email.String,
					CreatedAt: user.CreatedAt.Format(time.RFC3339),
				},
			})
		}
	}
}

// HandleLogout handles user logout
func HandleLogout(db *database.DB) http.HandlerFunc {
	auditRepo := repository.NewAuditRepository(db)

	return func(w http.ResponseWriter, r *http.Request) {
		// Get user context if available
		userCtx := middleware.GetUserContext(r)
		ipAddress := getIPAddress(r)
		userAgent := r.Header.Get("User-Agent")

		// Log logout
		if userCtx != nil {
			auditRepo.LogWithDetails(
				sql.NullInt64{Int64: userCtx.UserID, Valid: true},
				"logout",
				"user",
				sql.NullInt64{Int64: userCtx.UserID, Valid: true},
				nil,
				ipAddress,
				userAgent,
			)
		}

		// Clear authentication cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Logout successful",
		})
	}
}

// HandleGetCurrentUser returns the current authenticated user's information
func HandleGetCurrentUser(db *database.DB) http.HandlerFunc {
	userRepo := repository.NewUserRepository(db)

	return func(w http.ResponseWriter, r *http.Request) {
		// Get user context (set by auth middleware)
		userCtx := middleware.GetUserContext(r)
		if userCtx == nil {
			respondErrorWithRequest(w, r, http.StatusUnauthorized, "Not authenticated")
			return
		}

		// Get full user details
		user, err := userRepo.GetByID(userCtx.UserID)
		if err == repository.ErrNotFound {
			respondErrorWithRequest(w, r, http.StatusNotFound, "User not found")
			return
		}
		if err != nil {
			respondErrorWithRequest(w, r, http.StatusInternalServerError, "Failed to retrieve user information")
			return
		}

		// Check if account is still active
		if !user.IsActive {
			respondErrorWithRequest(w, r, http.StatusForbidden, "Account is inactive")
			return
		}

		// Respond with user information
		respondJSON(w, http.StatusOK, UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email.String,
			CreatedAt: user.CreatedAt.Format(time.RFC3339),
		})
	}
}

// HandleRefreshToken generates a new JWT token from an existing (possibly expired) token
func HandleRefreshToken(db *database.DB, jwtManager *auth.JWTManager) http.HandlerFunc {
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)

	return func(w http.ResponseWriter, r *http.Request) {
		ipAddress := getIPAddress(r)
		userAgent := r.Header.Get("User-Agent")

		// Get token from cookie or Authorization header
		token := getTokenFromRequest(r)
		if token == "" {
			respondErrorWithRequest(w, r, http.StatusUnauthorized, "No token provided")
			return
		}

		// Attempt to refresh the token (this works even if token is expired)
		newToken, err := jwtManager.RefreshToken(token)
		if err != nil {
			auditRepo.LogWithDetails(
				sql.NullInt64{Valid: false},
				"token_refresh_failed",
				"token",
				sql.NullInt64{Valid: false},
				map[string]interface{}{"reason": err.Error()},
				ipAddress,
				userAgent,
			)
			respondErrorWithRequest(w, r, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		// Validate the new token to get user info
		claims, err := jwtManager.ValidateToken(newToken)
		if err != nil {
			respondErrorWithRequest(w, r, http.StatusInternalServerError, "Failed to validate new token")
			return
		}

		// Verify user still exists and is active
		user, err := userRepo.GetByID(claims.UserID)
		if err == repository.ErrNotFound {
			respondErrorWithRequest(w, r, http.StatusUnauthorized, "User not found")
			return
		}
		if err != nil {
			respondErrorWithRequest(w, r, http.StatusInternalServerError, "Failed to verify user")
			return
		}

		if !user.IsActive {
			respondErrorWithRequest(w, r, http.StatusForbidden, "Account is inactive")
			return
		}

		// Check if account is locked
		isLocked, err := userRepo.IsAccountLocked(user.ID)
		if err != nil {
			respondErrorWithRequest(w, r, http.StatusInternalServerError, "An error occurred")
			return
		}
		if isLocked {
			respondErrorWithRequest(w, r, http.StatusForbidden, "Account is locked")
			return
		}

		// Set new token in cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    newToken,
			Path:     "/",
			MaxAge:   int(jwtManager.SessionDuration().Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})

		// Log token refresh
		auditRepo.LogWithDetails(
			sql.NullInt64{Int64: user.ID, Valid: true},
			"token_refreshed",
			"token",
			sql.NullInt64{Int64: user.ID, Valid: true},
			nil,
			ipAddress,
			userAgent,
		)

		// Respond with new token
		respondJSON(w, http.StatusOK, AuthResponse{
			Success: true,
			Message: "Token refreshed successfully",
			Token:   newToken,
		})
	}
}

// Helper functions

// getIPAddress extracts the client IP address from the request
func getIPAddress(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	ip = r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}

	// Fall back to RemoteAddr
	ip = r.RemoteAddr
	// RemoteAddr includes port, strip it
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	return ip
}

// getTokenFromRequest extracts JWT token from request (cookie or header)
func getTokenFromRequest(r *http.Request) string {
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

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Printf("Error encoding JSON response: %v\n", err)
	}
}

// respondError sends an error response
func respondError(w http.ResponseWriter, statusCode int, message string) {
	respondJSON(w, statusCode, ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	})
}

// respondErrorWithRequest sends an error response (HTML for HTMX, JSON otherwise)
func respondErrorWithRequest(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	if r.Header.Get("HX-Request") == "true" {
		// HTMX request - return prominent HTML error message
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(statusCode)
		// Create a prominent, styled error alert
		errorHTML := fmt.Sprintf(`
<div role="alert" style="
	background-color: #fee;
	border: 2px solid #c33;
	border-radius: 8px;
	padding: 1rem 1.25rem;
	margin-bottom: 1.5rem;
	box-shadow: 0 2px 4px rgba(0,0,0,0.1);
">
	<div style="display: flex; align-items: start; gap: 0.75rem;">
		<span style="font-size: 1.5rem; line-height: 1;">⚠️</span>
		<div style="flex: 1;">
			<strong style="color: #c33; font-size: 1rem; display: block; margin-bottom: 0.25rem;">Error</strong>
			<p style="color: #333; margin: 0; font-size: 0.95rem; line-height: 1.5;">%s</p>
		</div>
	</div>
</div>`, message)
		fmt.Fprint(w, errorHTML)
	} else {
		// Standard JSON response
		respondError(w, statusCode, message)
	}
}

// HandleSetup handles first-run setup (creates initial admin user)
func HandleSetup(db *database.DB) http.HandlerFunc {
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)

	return func(w http.ResponseWriter, r *http.Request) {
		// Check if users already exist (prevent setup bypass)
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if count > 0 {
			http.Error(w, "Setup already completed", http.StatusForbidden)
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")
		email := strings.TrimSpace(r.FormValue("email"))

		// Validate inputs
		if username == "" || password == "" {
			http.Error(w, "Username and password are required", http.StatusBadRequest)
			return
		}

		if password != confirmPassword {
			http.Error(w, "Passwords do not match", http.StatusBadRequest)
			return
		}

		if len(password) < 8 {
			http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
			return
		}

		if len(username) < 3 || len(username) > 50 {
			http.Error(w, "Username must be 3-50 characters", http.StatusBadRequest)
			return
		}

		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
		if err != nil {
			http.Error(w, "Failed to process password", http.StatusInternalServerError)
			return
		}

		// Create user model
		user := &models.User{
			Username:     username,
			PasswordHash: string(hashedPassword),
			Email:        sql.NullString{String: email, Valid: email != ""},
			IsActive:     true,
		}

		// Create user in database
		if err := userRepo.Create(user); err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				http.Error(w, "Username already exists", http.StatusConflict)
				return
			}
			http.Error(w, "Failed to create user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Create account for the first user
		accountRepo := repository.NewAccountRepository(db.DB)
		accountID, err := accountRepo.Create(nil, user.ID) // nil = no custom account name
		if err != nil {
			// Rollback: Delete the user if account creation fails
			_ = userRepo.Delete(user.ID)
			http.Error(w, "Failed to create account: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Log the setup
		ipAddress := r.RemoteAddr
		userAgent := r.UserAgent()
		auditRepo.LogWithDetails(
			sql.NullInt64{Int64: user.ID, Valid: true},
			"first_run_setup",
			"user",
			sql.NullInt64{Int64: user.ID, Valid: true},
			map[string]interface{}{"username": username, "account_id": accountID},
			ipAddress,
			userAgent,
		)

		// Redirect to login page
		http.Redirect(w, r, "/login?setup=success", http.StatusSeeOther)
	}
}
