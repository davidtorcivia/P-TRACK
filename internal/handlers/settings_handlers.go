package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/middleware"
)

// SettingsResponse represents the settings API response
type SettingsResponse struct {
	AdvancedModeEnabled bool   `json:"advanced_mode_enabled"`
	HeatMapDays         int    `json:"heat_map_days"`
	LowStockAlerts      bool   `json:"low_stock_alerts"`
	InjectionReminders  bool   `json:"injection_reminders"`
	ReminderTime        string `json:"reminder_time"` // HH:MM format
	ReminderFrequency   int    `json:"reminder_frequency"` // Hours between injections
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
	DefaultAdvancedMode      = false
	DefaultHeatMapDays       = 14
	DefaultLowStockAlerts    = true
	DefaultInjectionReminders = false
	DefaultReminderTime      = "19:00"
	DefaultReminderFrequency = 24
)

// HandleGetSettings returns all application settings
func HandleGetSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())

		settings, err := getSettings(db)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get settings: %v", err), http.StatusInternalServerError)
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
		json.NewEncoder(w).Encode(response)
	}
}

// HandleUpdateSettings updates application settings
func HandleUpdateSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
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
		defer tx.Rollback()

		now := time.Now()

		// Update each setting if provided
		if req.AdvancedModeEnabled != nil {
			if err := upsertSetting(tx, "advanced_mode_enabled", boolToString(*req.AdvancedModeEnabled), userID, now); err != nil {
				http.Error(w, "Failed to update advanced_mode_enabled", http.StatusInternalServerError)
				return
			}
		}

		if req.HeatMapDays != nil {
			if err := upsertSetting(tx, "heat_map_days", fmt.Sprintf("%d", *req.HeatMapDays), userID, now); err != nil {
				http.Error(w, "Failed to update heat_map_days", http.StatusInternalServerError)
				return
			}
		}

		if req.LowStockAlerts != nil {
			if err := upsertSetting(tx, "low_stock_alerts", boolToString(*req.LowStockAlerts), userID, now); err != nil {
				http.Error(w, "Failed to update low_stock_alerts", http.StatusInternalServerError)
				return
			}
		}

		if req.InjectionReminders != nil {
			if err := upsertSetting(tx, "injection_reminders", boolToString(*req.InjectionReminders), userID, now); err != nil {
				http.Error(w, "Failed to update injection_reminders", http.StatusInternalServerError)
				return
			}
		}

		if req.ReminderTime != nil {
			if err := upsertSetting(tx, "reminder_time", *req.ReminderTime, userID, now); err != nil {
				http.Error(w, "Failed to update reminder_time", http.StatusInternalServerError)
				return
			}
		}

		if req.ReminderFrequency != nil {
			if err := upsertSetting(tx, "reminder_frequency", fmt.Sprintf("%d", *req.ReminderFrequency), userID, now); err != nil {
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
		settings, err := getSettings(db)
		if err != nil {
			http.Error(w, "Settings updated but failed to retrieve", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settings)
	}
}

// Helper functions

// getSettings retrieves all settings from the database with defaults
func getSettings(db *database.DB) (*SettingsResponse, error) {
	settings := &SettingsResponse{
		AdvancedModeEnabled: DefaultAdvancedMode,
		HeatMapDays:         DefaultHeatMapDays,
		LowStockAlerts:      DefaultLowStockAlerts,
		InjectionReminders:  DefaultInjectionReminders,
		ReminderTime:        DefaultReminderTime,
		ReminderFrequency:   DefaultReminderFrequency,
		UpdatedAt:           time.Now(),
	}

	// Query all settings
	rows, err := db.Query(`
		SELECT key, value, updated_at
		FROM settings
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var latestUpdate time.Time

	for rows.Next() {
		var key, value string
		var updatedAt time.Time

		if err := rows.Scan(&key, &value, &updatedAt); err != nil {
			return nil, err
		}

		// Track the latest update time
		if updatedAt.After(latestUpdate) {
			latestUpdate = updatedAt
		}

		// Parse each setting
		switch key {
		case "advanced_mode_enabled":
			settings.AdvancedModeEnabled = stringToBool(value)
		case "heat_map_days":
			if days, err := strconv.Atoi(value); err == nil {
				settings.HeatMapDays = days
			}
		case "low_stock_alerts":
			settings.LowStockAlerts = stringToBool(value)
		case "injection_reminders":
			settings.InjectionReminders = stringToBool(value)
		case "reminder_time":
			settings.ReminderTime = value
		case "reminder_frequency":
			if freq, err := strconv.Atoi(value); err == nil {
				settings.ReminderFrequency = freq
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

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
		w.Write([]byte(`{"message": "Profile updated successfully"}`))
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
		w.Write([]byte(`{"message": "Password changed successfully"}`))
	}
}

// HandleUpdateAppSettings updates application settings (theme, timezone, etc.)
func HandleUpdateAppSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
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
		defer tx.Rollback()

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

		if err := upsertSetting(tx, "advanced_mode_enabled", boolToString(req.AdvancedMode), userID, now); err != nil {
			http.Error(w, "Failed to update advanced mode", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Settings updated successfully"}`))
	}
}

// HandleUpdateNotificationSettings updates notification settings
func HandleUpdateNotificationSettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
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
		defer tx.Rollback()

		now := time.Now()

		if err := upsertSetting(tx, fmt.Sprintf("user_enable_notifications_%d", userID), boolToString(req.EnableNotifications), userID, now); err != nil {
			http.Error(w, "Failed to update enable notifications", http.StatusInternalServerError)
			return
		}

		if err := upsertSetting(tx, "injection_reminders", boolToString(req.InjectionReminders), userID, now); err != nil {
			http.Error(w, "Failed to update injection reminders", http.StatusInternalServerError)
			return
		}

		if req.ReminderTime != "" {
			if err := upsertSetting(tx, "reminder_time", req.ReminderTime, userID, now); err != nil {
				http.Error(w, "Failed to update reminder time", http.StatusInternalServerError)
				return
			}
		}

		if err := upsertSetting(tx, "low_stock_alerts", boolToString(req.LowStockAlerts), userID, now); err != nil {
			http.Error(w, "Failed to update low stock alerts", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Notification settings updated successfully"}`))
	}
}