package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/middleware"
)

// SettingsResponse represents the settings API response
type SettingsResponse struct {
	AdvancedModeEnabled bool      `json:"advanced_mode_enabled"`
	HeatMapDays         int       `json:"heat_map_days"`
	LowStockAlerts      bool      `json:"low_stock_alerts"`
	InjectionReminders  bool      `json:"injection_reminders"`
	ReminderTime        string    `json:"reminder_time"`      // HH:MM format
	ReminderFrequency   int       `json:"reminder_frequency"` // Hours between injections
	UpdatedAt           time.Time `json:"updated_at"`
}

// UpdateSettingsRequest represents the request to update settings
type UpdateSettingsRequest struct {
	AdvancedModeEnabled *bool   `json:"advanced_mode_enabled,omitempty"`
	HeatMapDays         *int    `json:"heat_map_days,omitempty"`
	LowStockAlerts      *bool   `json:"low_stock_alerts,omitempty"`
	InjectionReminders  *bool   `json:"injection_reminders,omitempty"`
	ReminderTime        *string `json:"reminder_time,omitempty"`
	ReminderFrequency   *int    `json:"reminder_frequency,omitempty"`
}

// Default settings values
const (
	DefaultAdvancedMode       = false
	DefaultHeatMapDays        = 14
	DefaultLowStockAlerts     = true
	DefaultInjectionReminders = false
	DefaultReminderTime       = "19:00"
	DefaultReminderFrequency  = 24
)

// HandleGetSettings returns all application settings
func HandleGetSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())

		settings, err := getSettings(db, accountID)
		if err != nil {
			http.Error(w, "Failed to get settings", http.StatusInternalServerError)
			return
		}

		// Add user-specific settings
		response := map[string]interface{}{
			"advanced_mode_enabled": settings.AdvancedModeEnabled,
			"heat_map_days":         settings.HeatMapDays,
			"low_stock_alerts":      settings.LowStockAlerts,
			"injection_reminders":   settings.InjectionReminders,
			"reminder_time":         settings.ReminderTime,
			"reminder_frequency":    settings.ReminderFrequency,
			"updated_at":            settings.UpdatedAt,
			"theme":                 "auto", // default
			"timezone":              "America/New_York",
			"date_format":           "MM/DD/YYYY",
			"time_format":           "12h",
		}

		// Load user-specific settings if authenticated
		if userID != 0 {
			var theme, timezone, dateFormat, timeFormat string
			err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, fmt.Sprintf("user_theme_%d", userID)).Scan(&theme)
			if err == nil {
				response["theme"] = theme
			}
			err = db.QueryRow(`SELECT value FROM settings WHERE key = ?`, fmt.Sprintf("user_timezone_%d", userID)).Scan(&timezone)
			if err == nil {
				response["timezone"] = timezone
			}
			err = db.QueryRow(`SELECT value FROM settings WHERE key = ?`, fmt.Sprintf("user_date_format_%d", userID)).Scan(&dateFormat)
			if err == nil {
				response["date_format"] = dateFormat
			}
			err = db.QueryRow(`SELECT value FROM settings WHERE key = ?`, fmt.Sprintf("user_time_format_%d", userID)).Scan(&timeFormat)
			if err == nil {
				response["time_format"] = timeFormat
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode settings response: %v", err)
		}
	}
}

// HandleUpdateSettings updates application settings
func HandleUpdateSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse request body
		var req UpdateSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate settings
		if req.HeatMapDays != nil && (*req.HeatMapDays < 1 || *req.HeatMapDays > 90) {
			http.Error(w, "heat_map_days must be between 1 and 90", http.StatusBadRequest)
			return
		}

		if req.ReminderTime != nil {
			if !isValidTimeFormat(*req.ReminderTime) {
				http.Error(w, "reminder_time must be in HH:MM format (24-hour)", http.StatusBadRequest)
				return
			}
		}

		if req.ReminderFrequency != nil && (*req.ReminderFrequency < 1 || *req.ReminderFrequency > 168) {
			http.Error(w, "reminder_frequency must be between 1 and 168 hours", http.StatusBadRequest)
			return
		}

		// Begin transaction
		tx, err := db.BeginTx()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		now := time.Now()

		// Update each setting if provided. These are account-level settings,
		// namespaced by account so one account's preferences don't affect another.
		if req.AdvancedModeEnabled != nil {
			if err := upsertSetting(tx, scopedSettingKey(accountID, "advanced_mode_enabled"), boolToString(*req.AdvancedModeEnabled), userID, now); err != nil {
				http.Error(w, "Failed to update advanced_mode_enabled", http.StatusInternalServerError)
				return
			}
		}

		if req.HeatMapDays != nil {
			if err := upsertSetting(tx, scopedSettingKey(accountID, "heat_map_days"), fmt.Sprintf("%d", *req.HeatMapDays), userID, now); err != nil {
				http.Error(w, "Failed to update heat_map_days", http.StatusInternalServerError)
				return
			}
		}

		if req.LowStockAlerts != nil {
			if err := upsertSetting(tx, scopedSettingKey(accountID, "low_stock_alerts"), boolToString(*req.LowStockAlerts), userID, now); err != nil {
				http.Error(w, "Failed to update low_stock_alerts", http.StatusInternalServerError)
				return
			}
		}

		if req.InjectionReminders != nil {
			if err := upsertSetting(tx, scopedSettingKey(accountID, "injection_reminders"), boolToString(*req.InjectionReminders), userID, now); err != nil {
				http.Error(w, "Failed to update injection_reminders", http.StatusInternalServerError)
				return
			}
		}

		if req.ReminderTime != nil {
			if err := upsertSetting(tx, scopedSettingKey(accountID, "reminder_time"), *req.ReminderTime, userID, now); err != nil {
				http.Error(w, "Failed to update reminder_time", http.StatusInternalServerError)
				return
			}
		}

		if req.ReminderFrequency != nil {
			if err := upsertSetting(tx, scopedSettingKey(accountID, "reminder_frequency"), fmt.Sprintf("%d", *req.ReminderFrequency), userID, now); err != nil {
				http.Error(w, "Failed to update reminder_frequency", http.StatusInternalServerError)
				return
			}
		}

		// Create audit log
		_, _ = tx.Exec(`
			INSERT INTO audit_logs (user_id, action, entity_type, entity_id, details, timestamp)
			VALUES (?, ?, ?, ?, ?, ?)
		`, userID, "update", "settings", 0, "Updated application settings", now)

		// Commit transaction
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		// Return updated settings
		settings, err := getSettings(db, accountID)
		if err != nil {
			http.Error(w, "Settings updated but failed to retrieve", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(settings); err != nil {
			log.Printf("Failed to encode settings response: %v", err)
		}
	}
}

// Helper functions

// scopedSettingKey namespaces an account-level setting key by account so that
// one account's preferences cannot affect another account.
func scopedSettingKey(accountID int64, key string) string {
	return fmt.Sprintf("account_%d_%s", accountID, key)
}

// readAccountSetting reads an account-level setting, preferring the account-scoped
// key and falling back to the legacy global key (written by older versions) so
// existing data keeps working without a migration.
func readAccountSetting(db *database.DB, accountID int64, key string) (string, time.Time, bool) {
	var value string
	var updatedAt time.Time
	err := db.QueryRow(`SELECT value, updated_at FROM settings WHERE key = ?`, scopedSettingKey(accountID, key)).Scan(&value, &updatedAt)
	if err == nil {
		return value, updatedAt, true
	}
	// Legacy global fallback.
	err = db.QueryRow(`SELECT value, updated_at FROM settings WHERE key = ?`, key).Scan(&value, &updatedAt)
	if err == nil {
		return value, updatedAt, true
	}
	return "", time.Time{}, false
}

// getSettings retrieves account-level settings from the database with defaults.
func getSettings(db *database.DB, accountID int64) (*SettingsResponse, error) {
	settings := &SettingsResponse{
		AdvancedModeEnabled: DefaultAdvancedMode,
		HeatMapDays:         DefaultHeatMapDays,
		LowStockAlerts:      DefaultLowStockAlerts,
		InjectionReminders:  DefaultInjectionReminders,
		ReminderTime:        DefaultReminderTime,
		ReminderFrequency:   DefaultReminderFrequency,
		UpdatedAt:           time.Now(),
	}

	var latestUpdate time.Time
	apply := func(key string, set func(string)) {
		if value, updatedAt, ok := readAccountSetting(db, accountID, key); ok {
			set(value)
			if updatedAt.After(latestUpdate) {
				latestUpdate = updatedAt
			}
		}
	}

	apply("advanced_mode_enabled", func(v string) { settings.AdvancedModeEnabled = stringToBool(v) })
	apply("heat_map_days", func(v string) {
		if days, err := strconv.Atoi(v); err == nil {
			settings.HeatMapDays = days
		}
	})
	apply("low_stock_alerts", func(v string) { settings.LowStockAlerts = stringToBool(v) })
	apply("injection_reminders", func(v string) { settings.InjectionReminders = stringToBool(v) })
	apply("reminder_time", func(v string) { settings.ReminderTime = v })
	apply("reminder_frequency", func(v string) {
		if freq, err := strconv.Atoi(v); err == nil {
			settings.ReminderFrequency = freq
		}
	})

	if !latestUpdate.IsZero() {
		settings.UpdatedAt = latestUpdate
	}

	return settings, nil
}

// upsertSetting inserts or updates a setting
func upsertSetting(tx *sql.Tx, key, value string, userID int64, now time.Time) error {
	// Check if setting exists
	var exists bool
	err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM settings WHERE key = ?)", key).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		// Update existing setting
		_, err = tx.Exec(`
			UPDATE settings
			SET value = ?, updated_at = ?, updated_by = ?
			WHERE key = ?
		`, value, now, userID, key)
	} else {
		// Insert new setting
		_, err = tx.Exec(`
			INSERT INTO settings (key, value, updated_at, updated_by)
			VALUES (?, ?, ?, ?)
		`, key, value, now, userID)
	}

	return err
}

// isValidTimeFormat validates HH:MM time format
func isValidTimeFormat(timeStr string) bool {
	_, err := time.Parse("15:04", timeStr)
	return err == nil
}

// boolToString converts bool to string
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// stringToBool converts string to bool
func stringToBool(s string) bool {
	return s == "true" || s == "1" || s == "yes" || s == "on"
}

// GetUserTimezone retrieves the user's timezone preference from the database
// Returns "America/New_York" (ET with automatic DST) as default
func GetUserTimezone(db *database.DB, userID int64) string {
	var timezone string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`,
		fmt.Sprintf("user_timezone_%d", userID)).Scan(&timezone)
	if err != nil || timezone == "" {
		return "America/New_York" // Default to ET
	}
	return timezone
}

// ConvertToUserTZ converts a time.Time to the user's timezone
// Automatically handles DST transitions via Go's time.LoadLocation
func ConvertToUserTZ(t time.Time, timezone string) time.Time {
	if t.IsZero() {
		return t
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fallback to default timezone if invalid
		loc, _ = time.LoadLocation("America/New_York")
	}
	return t.In(loc)
}

// FormatTimeForUser formats a time according to user's time format preference
func FormatTimeForUser(db *database.DB, userID int64, t time.Time) string {
	var timeFormat string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`,
		fmt.Sprintf("user_time_format_%d", userID)).Scan(&timeFormat)

	// Convert to user's timezone first
	timezone := GetUserTimezone(db, userID)
	t = ConvertToUserTZ(t, timezone)

	// Format based on preference
	if err == nil && timeFormat == "24h" {
		return t.Format("15:04") // 24-hour format
	}
	return t.Format("3:04 PM") // 12-hour format (default)
}

// FormatDateTimeForUser formats a date and time according to user preferences
func FormatDateTimeForUser(db *database.DB, userID int64, t time.Time) string {
	var dateFormat string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`,
		fmt.Sprintf("user_date_format_%d", userID)).Scan(&dateFormat)

	// Convert to user's timezone first
	timezone := GetUserTimezone(db, userID)
	t = ConvertToUserTZ(t, timezone)

	// Determine date format
	var goDateFormat string
	if err == nil {
		switch dateFormat {
		case "DD/MM/YYYY":
			goDateFormat = "02/01/2006"
		case "YYYY-MM-DD":
			goDateFormat = "2006-01-02"
		default: // MM/DD/YYYY
			goDateFormat = "01/02/2006"
		}
	} else {
		goDateFormat = "01/02/2006" // Default MM/DD/YYYY
	}

	// Get time format
	timeStr := FormatTimeForUser(db, userID, t)

	return fmt.Sprintf("%s %s", t.Format(goDateFormat), timeStr)
}

// HandleUpdateProfile updates user profile information
func HandleUpdateProfile(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// For now, just return success
		// TODO: Implement profile update
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "Profile updated successfully"}`))
	}
}

// HandleChangePassword changes user password
func HandleChangePassword(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// For now, just return success
		// TODO: Implement password change with current password verification
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "Password changed successfully"}`))
	}
}

// HandleUpdateAppSettings updates application settings (theme, timezone, etc.)
func HandleUpdateAppSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req struct {
			Theme        string `json:"theme"`
			Timezone     string `json:"timezone"`
			DateFormat   string `json:"date_format"`
			TimeFormat   string `json:"time_format"`
			AdvancedMode bool   `json:"advanced_mode"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate theme
		validThemes := map[string]bool{"light": true, "dark": true, "auto": true}
		if req.Theme != "" && !validThemes[req.Theme] {
			http.Error(w, "Invalid theme", http.StatusBadRequest)
			return
		}

		// Validate timezone
		if req.Timezone != "" {
			if _, err := time.LoadLocation(req.Timezone); err != nil {
				http.Error(w, "Invalid timezone", http.StatusBadRequest)
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

		// Store settings with user ID prefix
		if req.Theme != "" {
			if err := upsertSetting(tx, fmt.Sprintf("user_theme_%d", userID), req.Theme, userID, now); err != nil {
				http.Error(w, "Failed to update theme", http.StatusInternalServerError)
				return
			}
		}

		if req.Timezone != "" {
			if err := upsertSetting(tx, fmt.Sprintf("user_timezone_%d", userID), req.Timezone, userID, now); err != nil {
				http.Error(w, "Failed to update timezone", http.StatusInternalServerError)
				return
			}
		}

		if req.DateFormat != "" {
			if err := upsertSetting(tx, fmt.Sprintf("user_date_format_%d", userID), req.DateFormat, userID, now); err != nil {
				http.Error(w, "Failed to update date format", http.StatusInternalServerError)
				return
			}
		}

		if req.TimeFormat != "" {
			if err := upsertSetting(tx, fmt.Sprintf("user_time_format_%d", userID), req.TimeFormat, userID, now); err != nil {
				http.Error(w, "Failed to update time format", http.StatusInternalServerError)
				return
			}
		}

		if err := upsertSetting(tx, scopedSettingKey(accountID, "advanced_mode_enabled"), boolToString(req.AdvancedMode), userID, now); err != nil {
			http.Error(w, "Failed to update advanced mode", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "Settings updated successfully"}`))
	}
}

// HandleUpdateNotificationSettings updates notification settings
func HandleUpdateNotificationSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req struct {
			EnableNotifications bool   `json:"enable_notifications"`
			InjectionReminders  bool   `json:"injection_reminders"`
			ReminderTime        string `json:"reminder_time"`
			LowStockAlerts      bool   `json:"low_stock_alerts"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Begin transaction
		tx, err := db.BeginTx()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		now := time.Now()

		if err := upsertSetting(tx, fmt.Sprintf("user_enable_notifications_%d", userID), boolToString(req.EnableNotifications), userID, now); err != nil {
			http.Error(w, "Failed to update enable notifications", http.StatusInternalServerError)
			return
		}

		if err := upsertSetting(tx, scopedSettingKey(accountID, "injection_reminders"), boolToString(req.InjectionReminders), userID, now); err != nil {
			http.Error(w, "Failed to update injection reminders", http.StatusInternalServerError)
			return
		}

		if req.ReminderTime != "" {
			if err := upsertSetting(tx, scopedSettingKey(accountID, "reminder_time"), req.ReminderTime, userID, now); err != nil {
				http.Error(w, "Failed to update reminder time", http.StatusInternalServerError)
				return
			}
		}

		if err := upsertSetting(tx, scopedSettingKey(accountID, "low_stock_alerts"), boolToString(req.LowStockAlerts), userID, now); err != nil {
			http.Error(w, "Failed to update low stock alerts", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "Notification settings updated successfully"}`))
	}
}
