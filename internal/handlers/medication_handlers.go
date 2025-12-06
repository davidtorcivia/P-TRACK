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
	"injection-tracker/internal/models"
	"injection-tracker/internal/repository"

	"github.com/go-chi/chi/v5"
)

// CreateMedicationRequest represents the request body for creating a medication
type CreateMedicationRequest struct {
	Name              string  `json:"name"`
	Dosage            *string `json:"dosage,omitempty"`
	Frequency         *string `json:"frequency,omitempty"`
	StartDate         *string `json:"start_date,omitempty"`
	EndDate           *string `json:"end_date,omitempty"`
	Notes             *string `json:"notes,omitempty"`
	ScheduledTime     *string `json:"scheduled_time,omitempty"`      // HH:MM format
	TimeWindowMinutes *int64  `json:"time_window_minutes,omitempty"` // Optional time window
	ReminderEnabled   *bool   `json:"reminder_enabled,omitempty"`
	IsActive          *bool   `json:"is_active,omitempty"`
}

// UpdateMedicationRequest represents the request body for updating a medication
type UpdateMedicationRequest struct {
	Name              *string `json:"name,omitempty"`
	Dosage            *string `json:"dosage,omitempty"`
	Frequency         *string `json:"frequency,omitempty"`
	StartDate         *string `json:"start_date,omitempty"`
	EndDate           *string `json:"end_date,omitempty"`
	Notes             *string `json:"notes,omitempty"`
	ScheduledTime     *string `json:"scheduled_time,omitempty"`
	TimeWindowMinutes *int64  `json:"time_window_minutes,omitempty"`
	ReminderEnabled   *bool   `json:"reminder_enabled,omitempty"`
	IsActive          *bool   `json:"is_active,omitempty"`
}

// LogMedicationRequest represents the request body for logging medication taken/missed
type LogMedicationRequest struct {
	Timestamp *string `json:"timestamp,omitempty"`
	Taken     bool    `json:"taken"`
	Notes     *string `json:"notes,omitempty"`
}

// HandleGetMedications returns a list of medications
func HandleGetMedications(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check for filter parameter
		filter := r.URL.Query().Get("filter")

		medicationRepo := repository.NewMedicationRepository(db)
		var medications []*models.Medication
		var err error

		if filter == "active" {
			medications, err = medicationRepo.ListActive(accountID)
		} else {
			medications, err = medicationRepo.List(accountID)
		}

		if err != nil {
			http.Error(w, "Failed to retrieve medications", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(medications); err != nil {
			log.Printf("Failed to encode medications response: %v", err)
		}
	}
}

// HandleCreateMedication creates a new medication
func HandleCreateMedication(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req CreateMedicationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		// Parse dates if provided
		var startDate sql.NullTime
		if req.StartDate != nil && *req.StartDate != "" {
			parsedDate, err := time.Parse("2006-01-02", *req.StartDate)
			if err != nil {
				http.Error(w, "Invalid start_date format, use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
			startDate = sql.NullTime{Time: parsedDate, Valid: true}
		}

		var endDate sql.NullTime
		if req.EndDate != nil && *req.EndDate != "" {
			parsedDate, err := time.Parse("2006-01-02", *req.EndDate)
			if err != nil {
				http.Error(w, "Invalid end_date format, use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
			endDate = sql.NullTime{Time: parsedDate, Valid: true}
		}

		// Set is_active default to true if not specified
		isActive := true
		if req.IsActive != nil {
			isActive = *req.IsActive
		}

		// Set reminder_enabled default to false if not specified
		reminderEnabled := false
		if req.ReminderEnabled != nil {
			reminderEnabled = *req.ReminderEnabled
		}

		// Create medication
		medication := &models.Medication{
			Name:              req.Name,
			Dosage:            nullString(req.Dosage),
			Frequency:         nullString(req.Frequency),
			StartDate:         startDate,
			EndDate:           endDate,
			IsActive:          isActive,
			Notes:             nullString(req.Notes),
			ScheduledTime:     nullString(req.ScheduledTime),
			TimeWindowMinutes: nullInt64(req.TimeWindowMinutes),
			ReminderEnabled:   reminderEnabled,
			AccountID:         accountID,
		}

		medicationRepo := repository.NewMedicationRepository(db)
		if err := medicationRepo.Create(medication); err != nil {
			http.Error(w, fmt.Sprintf("Failed to create medication: %v", err), http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditRepo := repository.NewAuditRepository(db)
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: userID, Valid: true},
			"create",
			"medication",
			sql.NullInt64{Int64: medication.ID, Valid: true},
			map[string]interface{}{
				"name":      medication.Name,
				"is_active": medication.IsActive,
			},
			r.RemoteAddr,
			r.UserAgent(),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(medication); err != nil {
			log.Printf("Failed to encode medication response: %v", err)
		}
	}
}

// HandleGetMedication returns a single medication by ID
func HandleGetMedication(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid medication ID", http.StatusBadRequest)
			return
		}

		medicationRepo := repository.NewMedicationRepository(db)
		medication, err := medicationRepo.GetByID(id, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Medication not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve medication", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(medication); err != nil {
			log.Printf("Failed to encode medication response: %v", err)
		}
	}
}

// HandleUpdateMedication updates an existing medication
func HandleUpdateMedication(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid medication ID", http.StatusBadRequest)
			return
		}

		var req UpdateMedicationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Get existing medication
		medicationRepo := repository.NewMedicationRepository(db)
		medication, err := medicationRepo.GetByID(id, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Medication not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve medication", http.StatusInternalServerError)
			return
		}

		// Update fields if provided
		if req.Name != nil {
			medication.Name = *req.Name
		}
		if req.Dosage != nil {
			if *req.Dosage == "" {
				medication.Dosage = sql.NullString{Valid: false}
			} else {
				medication.Dosage = sql.NullString{String: *req.Dosage, Valid: true}
			}
		}
		if req.Frequency != nil {
			if *req.Frequency == "" {
				medication.Frequency = sql.NullString{Valid: false}
			} else {
				medication.Frequency = sql.NullString{String: *req.Frequency, Valid: true}
			}
		}
		if req.StartDate != nil {
			if *req.StartDate == "" {
				medication.StartDate = sql.NullTime{Valid: false}
			} else {
				parsedDate, err := time.Parse("2006-01-02", *req.StartDate)
				if err != nil {
					http.Error(w, "Invalid start_date format, use YYYY-MM-DD", http.StatusBadRequest)
					return
				}
				medication.StartDate = sql.NullTime{Time: parsedDate, Valid: true}
			}
		}
		if req.EndDate != nil {
			if *req.EndDate == "" {
				medication.EndDate = sql.NullTime{Valid: false}
			} else {
				parsedDate, err := time.Parse("2006-01-02", *req.EndDate)
				if err != nil {
					http.Error(w, "Invalid end_date format, use YYYY-MM-DD", http.StatusBadRequest)
					return
				}
				medication.EndDate = sql.NullTime{Time: parsedDate, Valid: true}
			}
		}
		if req.Notes != nil {
			if *req.Notes == "" {
				medication.Notes = sql.NullString{Valid: false}
			} else {
				medication.Notes = sql.NullString{String: *req.Notes, Valid: true}
			}
		}
		if req.IsActive != nil {
			medication.IsActive = *req.IsActive
		}

		// Update medication
		if err := medicationRepo.Update(medication, accountID); err != nil {
			http.Error(w, "Failed to update medication", http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditRepo := repository.NewAuditRepository(db)
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: userID, Valid: true},
			"update",
			"medication",
			sql.NullInt64{Int64: medication.ID, Valid: true},
			map[string]interface{}{
				"name": medication.Name,
			},
			r.RemoteAddr,
			r.UserAgent(),
		)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(medication); err != nil {
			log.Printf("Failed to encode medication response: %v", err)
		}
	}
}

// HandleDeleteMedication soft-deletes a medication by setting is_active to false
func HandleDeleteMedication(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid medication ID", http.StatusBadRequest)
			return
		}

		// Get medication details for audit log
		medicationRepo := repository.NewMedicationRepository(db)
		medication, err := medicationRepo.GetByID(id, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Medication not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve medication", http.StatusInternalServerError)
			return
		}

		// Hard delete medication (this will cascade delete all logs)
		if err := medicationRepo.HardDelete(id, accountID); err != nil {
			http.Error(w, "Failed to delete medication", http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditRepo := repository.NewAuditRepository(db)
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: userID, Valid: true},
			"delete",
			"medication",
			sql.NullInt64{Int64: id, Valid: true},
			map[string]interface{}{
				"name": medication.Name,
			},
			r.RemoteAddr,
			r.UserAgent(),
		)

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleLogMedication creates a log entry for medication taken or missed
func HandleLogMedication(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		medicationID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid medication ID", http.StatusBadRequest)
			return
		}

		var req LogMedicationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Verify medication exists
		medicationRepo := repository.NewMedicationRepository(db)
		medication, err := medicationRepo.GetByID(medicationID, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Medication not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve medication", http.StatusInternalServerError)
			return
		}

		// Parse timestamp or use current time
		var timestamp time.Time
		if req.Timestamp != nil && *req.Timestamp != "" {
			timestamp, err = time.Parse(time.RFC3339, *req.Timestamp)
			if err != nil {
				http.Error(w, "Invalid timestamp format, use RFC3339", http.StatusBadRequest)
				return
			}
		} else {
			timestamp = time.Now()
		}

		// Create medication log
		medLog := &models.MedicationLog{
			MedicationID: medicationID,
			LoggedBy:     sql.NullInt64{Int64: userID, Valid: true},
			Timestamp:    timestamp,
			Taken:        req.Taken,
			Notes:        nullString(req.Notes),
		}

		if err := medicationRepo.CreateLog(medLog); err != nil {
			http.Error(w, fmt.Sprintf("Failed to create medication log: %v", err), http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditRepo := repository.NewAuditRepository(db)
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: userID, Valid: true},
			"log_medication",
			"medication_log",
			sql.NullInt64{Int64: medLog.ID, Valid: true},
			map[string]interface{}{
				"medication_id":   medicationID,
				"medication_name": medication.Name,
				"taken":           req.Taken,
			},
			r.RemoteAddr,
			r.UserAgent(),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(medLog); err != nil {
			log.Printf("Failed to encode medication log response: %v", err)
		}
	}
}

// HandleGetMedicationLogs returns medication logs with optional filtering
func HandleGetMedicationLogs(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		medicationID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid medication ID", http.StatusBadRequest)
			return
		}

		// Verify medication exists
		medicationRepo := repository.NewMedicationRepository(db)
		_, err = medicationRepo.GetByID(medicationID, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Medication not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve medication", http.StatusInternalServerError)
			return
		}

		// Parse query parameters
		startDate := r.URL.Query().Get("start_date")
		endDate := r.URL.Query().Get("end_date")
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")

		// Set defaults
		limit := 50
		offset := 0

		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}
		if offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		var logs []*models.MedicationLog

		// Filter by date range if provided
		if startDate != "" && endDate != "" {
			start, err1 := time.Parse("2006-01-02", startDate)
			end, err2 := time.Parse("2006-01-02", endDate)
			if err1 != nil || err2 != nil {
				http.Error(w, "Invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
			logs, err = medicationRepo.ListLogsByDateRange(medicationID, start, end, limit, offset)
		} else {
			logs, err = medicationRepo.ListLogs(medicationID, limit, offset)
		}

		if err != nil {
			http.Error(w, "Failed to retrieve medication logs", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(logs); err != nil {
			log.Printf("Failed to encode medication logs response: %v", err)
		}
	}
}

// HandleGetTodaySchedule returns medications scheduled for today
func HandleGetTodaySchedule(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		medicationRepo := repository.NewMedicationRepository(db)
		medications, err := medicationRepo.ListActive(accountID)
		if err != nil {
			http.Error(w, "Failed to retrieve medications", http.StatusInternalServerError)
			return
		}

		// Return empty state HTML if no medications
		if len(medications) == 0 {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`
				<div style="text-align: center; padding: 2rem; color: var(--pico-muted-color);">
					<p>No medications scheduled for today.</p>
				</div>
			`))
			return
		}

		// Return as JSON for now
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(medications); err != nil {
			log.Printf("Failed to encode medications response: %v", err)
		}
	}
}

// HandleGetAdherence returns adherence statistics
func HandleGetAdherence(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// For now, return empty data
		_ = r.URL.Query().Get("days") // Unused for now
		response := map[string]interface{}{
			"medications": []string{},
			"taken":       []int{},
			"missed":      []int{},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode adherence response: %v", err)
		}
	}
}

// HandleGetDailySchedule returns HTML for today's medication schedule
func HandleGetDailySchedule(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		medicationRepo := repository.NewMedicationRepository(db)
		activeMeds, err := medicationRepo.ListActive(accountID)
		if err != nil || len(activeMeds) == 0 {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`
				<div style="text-align: center; padding: 2rem; color: var(--pico-muted-color);">
					<p>No active medications.</p>
				</div>
			`))
			return
		}

		// Check which medications were taken today
		for _, med := range activeMeds {
			var count int
			_ = db.QueryRow(`
				SELECT COUNT(*) FROM medication_logs
				WHERE medication_id = ?
				AND DATE(timestamp) = DATE('now')
				AND taken = 1
			`, med.ID).Scan(&count)
			med.TakenToday = count > 0
		}

		// Build HTML
		html := `<div style="display: flex; flex-direction: column; gap: 0.5rem;">`
		for _, med := range activeMeds {
			status := "⚠️ Not taken"
			statusColor := "var(--pico-warning)"
			if med.TakenToday {
				status = "✓ Taken"
				statusColor = "var(--pico-success)"
			}

			// Extract string values from NullString
			dosage := "N/A"
			if med.Dosage.Valid {
				dosage = med.Dosage.String
			}
			frequency := "N/A"
			if med.Frequency.Valid {
				frequency = med.Frequency.String
			}

			html += fmt.Sprintf(`
				<div style="display: flex; justify-content: space-between; align-items: center; padding: 0.5rem; border: 1px solid var(--pico-muted-border-color); border-radius: var(--pico-border-radius);">
					<div>
						<strong>%s</strong><br>
						<small>%s • %s</small>
					</div>
					<div style="color: %s; font-weight: bold;">
						%s
					</div>
				</div>
			`, med.Name, dosage, frequency, statusColor, status)
		}
		html += `</div>`

		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}
}
