package handlers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	netsmtp "net/smtp"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/middleware"
)

// ============================================
// ADMIN TYPES
// ============================================

// SMTPSettings represents SMTP configuration
type SMTPSettings struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password,omitempty"` // Only used for updates, never returned
	FromName  string `json:"from_name"`
	FromEmail string `json:"from_email"`
	Enabled   bool   `json:"enabled"`
}

// SiteSettings represents site-wide configuration
type SiteSettings struct {
	SiteURL         string `json:"site_url"`
	SiteTitle       string `json:"site_title"`
	SiteDescription string `json:"site_description"`
}

// AdminSettingsResponse represents all admin settings
type AdminSettingsResponse struct {
	SMTP       SMTPSettings  `json:"smtp"`
	Site       *SiteSettings `json:"site,omitempty"`
	SiteStats  *SiteStats    `json:"site_stats,omitempty"`
	IsAdmin    bool          `json:"is_admin"`
	AdminSetup bool          `json:"admin_setup"` // true if admin has configured initial settings
}

// SiteStats represents site-wide statistics
type SiteStats struct {
	TotalUsers      int64 `json:"total_users"`
	TotalAccounts   int64 `json:"total_accounts"`
	TotalInjections int64 `json:"total_injections"`
}

// UserInfo represents user information for admin view
type UserInfo struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	AccountID int64  `json:"account_id"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
	LastLogin string `json:"last_login,omitempty"`
}

// AccountInfo represents account information for admin view
type AccountInfo struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	MemberCount int    `json:"member_count"`
	CreatedAt   string `json:"created_at"`
	OwnerName   string `json:"owner_name"`
}

// ============================================
// ADMIN MIDDLEWARE
// ============================================

// RequireAdmin middleware ensures only the first user (admin) can access admin routes
func RequireAdmin(db *database.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := middleware.GetUserID(r.Context())
			if userID == 0 {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check if this user is the first user (admin)
			var firstUserID int64
			err := db.QueryRow("SELECT id FROM users ORDER BY id LIMIT 1").Scan(&firstUserID)
			if err != nil {
				http.Error(w, "Failed to verify admin status", http.StatusInternalServerError)
				return
			}

			if userID != firstUserID {
				http.Error(w, "Admin access required", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// IsAdmin checks if the current user is the admin
func IsAdmin(db *database.DB, userID int64) bool {
	var firstUserID int64
	err := db.QueryRow("SELECT id FROM users ORDER BY id LIMIT 1").Scan(&firstUserID)
	if err != nil {
		return false
	}
	return userID == firstUserID
}

// ============================================
// ADMIN HANDLERS
// ============================================

// HandleGetAdminSettings returns all admin settings
func HandleGetAdminSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		isAdmin := IsAdmin(db, userID)
		if !isAdmin {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		// Get SMTP settings (without password)
		smtp := getSMTPSettings(db)

		// Get site settings
		site := getSiteSettings(db)

		// Get site stats
		stats := getSiteStats(db)

		response := AdminSettingsResponse{
			SMTP:       smtp,
			Site:       site,
			SiteStats:  stats,
			IsAdmin:    true,
			AdminSetup: smtp.Host != "",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleUpdateSMTPSettings updates SMTP configuration
func HandleUpdateSMTPSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		var req SMTPSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields if enabled
		if req.Enabled {
			if req.Host == "" || req.Port == 0 || req.FromEmail == "" {
				http.Error(w, "Host, port, and from_email are required when SMTP is enabled", http.StatusBadRequest)
				return
			}
		}

		// Begin transaction
		tx, err := db.BeginTx()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		now := time.Now()

		// Upsert each setting
		settings := map[string]string{
			"smtp_host":       req.Host,
			"smtp_port":       fmt.Sprintf("%d", req.Port),
			"smtp_username":   req.Username,
			"smtp_from_name":  req.FromName,
			"smtp_from_email": req.FromEmail,
			"smtp_enabled":    fmt.Sprintf("%t", req.Enabled),
		}

		// Only update password if provided
		if req.Password != "" {
			// In production, encrypt this password
			settings["smtp_password"] = req.Password
		}

		for key, value := range settings {
			_, err := tx.Exec(`
				INSERT INTO settings (key, value, updated_at, updated_by)
				VALUES (?, ?, ?, ?)
				ON CONFLICT(key) DO UPDATE SET
					value = excluded.value,
					updated_at = excluded.updated_at,
					updated_by = excluded.updated_by
			`, key, value, now, userID)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to save setting %s: %v", key, err), http.StatusInternalServerError)
				return
			}
		}

		// Create audit log
		_, _ = tx.Exec(`
			INSERT INTO audit_logs (user_id, action, entity_type, entity_id, details, timestamp)
			VALUES (?, ?, ?, ?, ?, ?)
		`, userID, "update", "admin_settings", 0, "Updated SMTP settings", now)

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		// Return updated settings (without password)
		smtp := getSMTPSettings(db)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "SMTP settings updated successfully",
			"smtp":    smtp,
		})
	}
}

// HandleTestSMTP sends a test email to verify SMTP settings
func HandleTestSMTP(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
			http.Error(w, "Email address is required", http.StatusBadRequest)
			return
		}

		// Get SMTP settings
		smtp := getSMTPSettings(db)
		if !smtp.Enabled {
			http.Error(w, "SMTP is not enabled", http.StatusBadRequest)
			return
		}

		if smtp.Host == "" || smtp.Port == 0 {
			http.Error(w, "SMTP is not properly configured", http.StatusBadRequest)
			return
		}

		// Get password for sending
		var password string
		_ = db.QueryRow("SELECT value FROM settings WHERE key = 'smtp_password'").Scan(&password)

		// Send test email
		err := sendTestEmail(smtp, password, req.Email)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": fmt.Sprintf("Failed to send test email: %v", err),
				"success": false,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": fmt.Sprintf("Test email sent successfully to %s", req.Email),
			"success": true,
		})
	}
}

// HandleGetSiteStats returns site-wide statistics
func HandleGetSiteStats(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		stats := getSiteStats(db)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

// HandleCheckAdmin checks if the current user is an admin
func HandleCheckAdmin(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{
			"is_admin": IsAdmin(db, userID),
		})
	}
}

// ============================================
// SITE CONFIGURATION HANDLERS
// ============================================

// HandleGetSiteSettings returns site configuration
func HandleGetSiteSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		settings := getSiteSettings(db)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settings)
	}
}

// HandleUpdateSiteSettings updates site configuration
func HandleUpdateSiteSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		var req SiteSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		now := time.Now()

		// Handle site_url specially - delete if empty to revert to default
		if req.SiteURL == "" {
			_, _ = db.Exec(`DELETE FROM settings WHERE key = 'site_url'`)
		} else {
			_, err := db.Exec(`
				INSERT INTO settings (key, value, updated_at, updated_by)
				VALUES (?, ?, ?, ?)
				ON CONFLICT(key) DO UPDATE SET
					value = excluded.value,
					updated_at = excluded.updated_at,
					updated_by = excluded.updated_by
			`, "site_url", req.SiteURL, now, userID)
			if err != nil {
				http.Error(w, "Failed to save site_url", http.StatusInternalServerError)
				return
			}
		}

		// Upsert other settings (only update non-empty values)
		settings := map[string]string{
			"site_title":       req.SiteTitle,
			"site_description": req.SiteDescription,
		}

		for key, value := range settings {
			if value != "" { // Only update non-empty values
				_, err := db.Exec(`
					INSERT INTO settings (key, value, updated_at, updated_by)
					VALUES (?, ?, ?, ?)
					ON CONFLICT(key) DO UPDATE SET
						value = excluded.value,
						updated_at = excluded.updated_at,
						updated_by = excluded.updated_by
				`, key, value, now, userID)
				if err != nil {
					http.Error(w, fmt.Sprintf("Failed to save setting %s", key), http.StatusInternalServerError)
					return
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":  "Site settings updated successfully",
			"settings": getSiteSettings(db),
		})
	}
}

// ============================================
// USER MANAGEMENT HANDLERS
// ============================================

// HandleGetAllUsers returns all users for admin management
func HandleGetAllUsers(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		rows, err := db.Query(`
			SELECT u.id, u.username, COALESCE(u.email, '') as email, 
			       COALESCE(am.account_id, 0) as account_id, COALESCE(am.role, 'member') as role,
			       u.is_active, u.created_at, u.last_login
			FROM users u
			LEFT JOIN account_members am ON u.id = am.user_id
			ORDER BY u.id
		`)
		if err != nil {
			http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		users := []UserInfo{}
		for rows.Next() {
			var u UserInfo
			var createdAt time.Time
			var nullLastLogin *time.Time

			err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.AccountID, &u.Role, &u.IsActive, &createdAt, &nullLastLogin)
			if err != nil {
				continue
			}

			u.CreatedAt = createdAt.Format("2006-01-02 15:04")
			if nullLastLogin != nil {
				u.LastLogin = nullLastLogin.Format("2006-01-02 15:04")
			}

			users = append(users, u)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	}
}

// HandleGetAllAccounts returns all accounts for admin management
func HandleGetAllAccounts(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		rows, err := db.Query(`
			SELECT a.id, COALESCE(a.name, 'Account ' || a.id) as name, a.created_at,
			       (SELECT COUNT(*) FROM account_members WHERE account_id = a.id) as member_count,
			       COALESCE((SELECT u.username FROM users u 
			                 JOIN account_members am ON u.id = am.user_id 
			                 WHERE am.account_id = a.id AND am.role = 'owner' LIMIT 1), 'Unknown') as owner_name
			FROM accounts a
			ORDER BY a.id
		`)
		if err != nil {
			http.Error(w, "Failed to fetch accounts", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		accounts := []AccountInfo{}
		for rows.Next() {
			var a AccountInfo
			var createdAt time.Time

			err := rows.Scan(&a.ID, &a.Name, &createdAt, &a.MemberCount, &a.OwnerName)
			if err != nil {
				continue
			}

			a.CreatedAt = createdAt.Format("2006-01-02")
			accounts = append(accounts, a)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(accounts)
	}
}

// HandleDeleteAccount deletes an account and all its data
func HandleDeleteAccount(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		var req struct {
			AccountID int64 `json:"account_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.AccountID == 0 {
			http.Error(w, "Account ID is required", http.StatusBadRequest)
			return
		}

		// Get admin's account to prevent self-deletion
		var adminAccountID int64
		_ = db.QueryRow("SELECT account_id FROM account_members WHERE user_id = ?", userID).Scan(&adminAccountID)
		if req.AccountID == adminAccountID {
			http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
			return
		}

		// Begin transaction
		tx, err := db.BeginTx()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		// Delete in order due to foreign keys
		_, _ = tx.Exec("DELETE FROM symptom_logs WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)", req.AccountID)
		_, _ = tx.Exec("DELETE FROM injections WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)", req.AccountID)
		_, _ = tx.Exec("DELETE FROM courses WHERE account_id = ?", req.AccountID)
		_, _ = tx.Exec("DELETE FROM medications WHERE account_id = ?", req.AccountID)
		_, _ = tx.Exec("DELETE FROM account_invitations WHERE account_id = ?", req.AccountID)
		_, _ = tx.Exec("DELETE FROM account_members WHERE account_id = ?", req.AccountID)
		_, _ = tx.Exec("DELETE FROM accounts WHERE id = ?", req.AccountID)

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to delete account", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Account deleted successfully",
			"success": true,
		})
	}
}

// HandleDeactivateUser deactivates a user account
func HandleDeactivateUser(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		var req struct {
			TargetUserID int64 `json:"user_id"`
			Active       bool  `json:"active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.TargetUserID == userID {
			http.Error(w, "Cannot deactivate yourself", http.StatusBadRequest)
			return
		}

		_, err := db.Exec("UPDATE users SET is_active = ? WHERE id = ?", req.Active, req.TargetUserID)
		if err != nil {
			http.Error(w, "Failed to update user", http.StatusInternalServerError)
			return
		}

		action := "deactivated"
		if req.Active {
			action = "activated"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": fmt.Sprintf("User %s successfully", action),
			"success": true,
		})
	}
}

// HandleDeleteUser permanently deletes a user
func HandleDeleteUser(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 || !IsAdmin(db, userID) {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		var req struct {
			TargetUserID int64 `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Cannot delete admin (first user)
		if req.TargetUserID == 1 {
			http.Error(w, "Cannot delete the admin user", http.StatusBadRequest)
			return
		}

		// Cannot delete yourself
		if req.TargetUserID == userID {
			http.Error(w, "Cannot delete yourself", http.StatusBadRequest)
			return
		}

		// Get user's account info
		var accountID int64
		var memberCount int
		err := db.QueryRow("SELECT account_id FROM account_members WHERE user_id = ?", req.TargetUserID).Scan(&accountID)
		if err != nil {
			// User exists but not in any account - just delete user
			_, err = db.Exec("DELETE FROM users WHERE id = ?", req.TargetUserID)
			if err != nil {
				http.Error(w, "Failed to delete user", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "User deleted successfully",
				"success": true,
			})
			return
		}

		// Count members in this account
		_ = db.QueryRow("SELECT COUNT(*) FROM account_members WHERE account_id = ?", accountID).Scan(&memberCount)

		tx, err := db.BeginTx()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		if memberCount == 1 {
			// Sole member - delete entire account and all data
			_, _ = tx.Exec("DELETE FROM symptom_logs WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
			_, _ = tx.Exec("DELETE FROM injections WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
			_, _ = tx.Exec("DELETE FROM courses WHERE account_id = ?", accountID)
			_, _ = tx.Exec("DELETE FROM medications WHERE account_id = ?", accountID)
			_, _ = tx.Exec("DELETE FROM account_invitations WHERE account_id = ?", accountID)
			_, _ = tx.Exec("DELETE FROM account_members WHERE account_id = ?", accountID)
			_, _ = tx.Exec("DELETE FROM accounts WHERE id = ?", accountID)
		} else {
			// Multiple members - just remove user from account, keep data
			_, _ = tx.Exec("DELETE FROM account_members WHERE user_id = ?", req.TargetUserID)
		}

		// Delete the user
		_, _ = tx.Exec("DELETE FROM session_tokens WHERE user_id = ?", req.TargetUserID)
		_, _ = tx.Exec("DELETE FROM password_reset_tokens WHERE user_id = ?", req.TargetUserID)
		_, _ = tx.Exec("DELETE FROM notifications WHERE user_id = ?", req.TargetUserID)
		_, _ = tx.Exec("DELETE FROM users WHERE id = ?", req.TargetUserID)

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to delete user", http.StatusInternalServerError)
			return
		}

		message := "User deleted successfully"
		if memberCount == 1 {
			message = "User and their account data deleted successfully"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": message,
			"success": true,
		})
	}
}

// ============================================
// HELPER FUNCTIONS
// ============================================

func getSMTPSettings(db *database.DB) SMTPSettings {
	smtp := SMTPSettings{}

	var value string
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'smtp_host'").Scan(&value); err == nil {
		smtp.Host = value
	}
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'smtp_port'").Scan(&value); err == nil {
		fmt.Sscanf(value, "%d", &smtp.Port)
	}
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'smtp_username'").Scan(&value); err == nil {
		smtp.Username = value
	}
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'smtp_from_name'").Scan(&value); err == nil {
		smtp.FromName = value
	}
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'smtp_from_email'").Scan(&value); err == nil {
		smtp.FromEmail = value
	}
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'smtp_enabled'").Scan(&value); err == nil {
		smtp.Enabled = value == "true"
	}

	// Never return password
	return smtp
}

func getSiteSettings(db *database.DB) *SiteSettings {
	site := &SiteSettings{
		SiteTitle: "P-TRACK", // Default
	}

	var value string
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'site_url'").Scan(&value); err == nil {
		site.SiteURL = value
	}
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'site_title'").Scan(&value); err == nil && value != "" {
		site.SiteTitle = value
	}
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'site_description'").Scan(&value); err == nil {
		site.SiteDescription = value
	}

	return site
}

func getSiteStats(db *database.DB) *SiteStats {
	stats := &SiteStats{}

	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.TotalUsers)
	db.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&stats.TotalAccounts)
	db.QueryRow("SELECT COUNT(*) FROM injections").Scan(&stats.TotalInjections)

	return stats
}

// IsSMTPConfigured checks if SMTP is configured and enabled
func IsSMTPConfigured(db *database.DB) bool {
	smtp := getSMTPSettings(db)
	return smtp.Enabled && smtp.Host != "" && smtp.Port > 0 && smtp.FromEmail != ""
}

// sendTestEmail sends a test email using the provided SMTP settings
func sendTestEmail(settings SMTPSettings, password string, toEmail string) error {
	addr := fmt.Sprintf("%s:%d", settings.Host, settings.Port)

	// Setup message
	from := settings.FromEmail
	if settings.FromName != "" {
		from = fmt.Sprintf("%s <%s>", settings.FromName, settings.FromEmail)
	}

	subject := "P-TRACK SMTP Test"
	body := "This is a test email from P-TRACK to verify your SMTP configuration is working correctly."

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		from, toEmail, subject, body)

	// Use TLS for port 465, STARTTLS for other ports
	if settings.Port == 465 {
		// Direct TLS connection
		tlsConfig := &tls.Config{
			ServerName: settings.Host,
		}

		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("TLS connection failed: %w", err)
		}
		defer conn.Close()

		client, err := netsmtp.NewClient(conn, settings.Host)
		if err != nil {
			return fmt.Errorf("SMTP client creation failed: %w", err)
		}
		defer client.Close()

		// Auth if credentials provided
		if settings.Username != "" && password != "" {
			auth := netsmtp.PlainAuth("", settings.Username, password, settings.Host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
		}

		if err := client.Mail(settings.FromEmail); err != nil {
			return fmt.Errorf("MAIL FROM failed: %w", err)
		}
		if err := client.Rcpt(toEmail); err != nil {
			return fmt.Errorf("RCPT TO failed: %w", err)
		}

		wc, err := client.Data()
		if err != nil {
			return fmt.Errorf("DATA failed: %w", err)
		}
		_, err = wc.Write([]byte(msg))
		wc.Close()
		if err != nil {
			return fmt.Errorf("write message failed: %w", err)
		}

		return client.Quit()
	}

	// Standard SMTP with optional STARTTLS
	var auth netsmtp.Auth
	if settings.Username != "" && password != "" {
		auth = netsmtp.PlainAuth("", settings.Username, password, settings.Host)
	}

	err := netsmtp.SendMail(addr, auth, settings.FromEmail, []string{toEmail}, []byte(msg))
	if err != nil {
		return fmt.Errorf("send mail failed: %w", err)
	}

	return nil
}
