package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/middleware"
	"injection-tracker/internal/models"
	"injection-tracker/internal/repository"

	"github.com/go-chi/chi/v5"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// CreateSymptomRequest represents the request body for creating a symptom log
type CreateSymptomRequest struct {
	CourseID     int64    `json:"course_id"`
	Timestamp    *string  `json:"timestamp,omitempty"`
	PainLevel    *int     `json:"pain_level,omitempty"`
	PainLocation *string  `json:"pain_location,omitempty"`
	PainType     *string  `json:"pain_type,omitempty"`
	Symptoms     []string `json:"symptoms,omitempty"`
	Notes        *string  `json:"notes,omitempty"`
}

// UpdateSymptomRequest represents the request body for updating a symptom log
type UpdateSymptomRequest struct {
	CourseID     *int64   `json:"course_id,omitempty"`
	Timestamp    *string  `json:"timestamp,omitempty"`
	PainLevel    *int     `json:"pain_level,omitempty"`
	PainLocation *string  `json:"pain_location,omitempty"`
	PainType     *string  `json:"pain_type,omitempty"`
	Symptoms     []string `json:"symptoms,omitempty"`
	Notes        *string  `json:"notes,omitempty"`
}

// HandleGetSymptoms returns a list of symptom logs with optional filtering
func HandleGetSymptoms(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse query parameters
		courseID := r.URL.Query().Get("course_id")
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

		symptomRepo := repository.NewSymptomRepository(db)
		var symptoms []*models.SymptomLog
		var err error

		// Filter by course or date range
		if courseID != "" {
			cid, err := strconv.ParseInt(courseID, 10, 64)
			if err != nil {
				http.Error(w, "Invalid course_id", http.StatusBadRequest)
				return
			}
			symptoms, err = symptomRepo.ListByCourse(cid, accountID, limit, offset)
			if err != nil {
				http.Error(w, "Failed to retrieve symptom logs", http.StatusInternalServerError)
				return
			}
		} else if startDate != "" && endDate != "" {
			start, err1 := time.Parse("2006-01-02", startDate)
			end, err2 := time.Parse("2006-01-02", endDate)
			if err1 != nil || err2 != nil {
				http.Error(w, "Invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
			symptoms, err = symptomRepo.ListByDateRange(accountID, start, end, limit, offset)
		} else {
			symptoms, err = symptomRepo.List(accountID, limit, offset)
		}

		if err != nil {
			http.Error(w, "Failed to retrieve symptom logs", http.StatusInternalServerError)
			return
		}

		// Get user's timezone preference
		userTimezone := GetUserTimezone(db, userID)

		// Convert to JSON-serializable format
		response := make([]map[string]interface{}, len(symptoms))
		for i, symptom := range symptoms {
			// Convert timestamps to user's timezone
			timestamp := ConvertToUserTZ(symptom.Timestamp, userTimezone)
			createdAt := ConvertToUserTZ(symptom.CreatedAt, userTimezone)
			updatedAt := ConvertToUserTZ(symptom.UpdatedAt, userTimezone)

			response[i] = map[string]interface{}{
				"id":            symptom.ID,
				"course_id":     symptom.CourseID,
				"logged_by":     nullInt64ToInt(symptom.LoggedBy),
				"timestamp":     timestamp.Format(time.RFC3339),
				"pain_level":    nullInt64ToInt(symptom.PainLevel),
				"pain_location": nullStringToString(symptom.PainLocation),
				"pain_type":     nullStringToString(symptom.PainType),
				"symptoms":      nullStringToString(symptom.Symptoms),
				"notes":         nullStringToString(symptom.Notes),
				"created_at":    createdAt.Format(time.RFC3339),
				"updated_at":    updatedAt.Format(time.RFC3339),
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode symptoms response: %v", err)
		}
	}
}

// HandleCreateSymptom creates a new symptom log
func HandleCreateSymptom(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req CreateSymptomRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if req.CourseID == 0 {
			http.Error(w, "course_id is required", http.StatusBadRequest)
			return
		}

		// Validate pain level if provided
		if req.PainLevel != nil && (*req.PainLevel < 1 || *req.PainLevel > 10) {
			http.Error(w, "pain_level must be between 1 and 10", http.StatusBadRequest)
			return
		}

		// Parse timestamp or use current time
		var timestamp time.Time
		if req.Timestamp != nil {
			var err error
			timestamp, err = time.Parse(time.RFC3339, *req.Timestamp)
			if err != nil {
				http.Error(w, "Invalid timestamp format, use RFC3339", http.StatusBadRequest)
				return
			}
		} else {
			timestamp = time.Now()
		}

		// Convert symptoms array to JSON string
		var symptomsJSON sql.NullString
		if len(req.Symptoms) > 0 {
			jsonBytes, err := json.Marshal(req.Symptoms)
			if err != nil {
				http.Error(w, "Failed to encode symptoms", http.StatusInternalServerError)
				return
			}
			symptomsJSON = sql.NullString{String: string(jsonBytes), Valid: true}
		}

		// Create symptom log
		symptom := &models.SymptomLog{
			CourseID:     req.CourseID,
			LoggedBy:     sql.NullInt64{Int64: userID, Valid: true},
			Timestamp:    timestamp,
			PainLevel:    nullInt64Ptr(req.PainLevel),
			PainLocation: nullString(req.PainLocation),
			PainType:     nullString(req.PainType),
			Symptoms:     symptomsJSON,
			Notes:        nullString(req.Notes),
		}

		symptomRepo := repository.NewSymptomRepository(db)
		if err := symptomRepo.Create(symptom); err != nil {
			http.Error(w, fmt.Sprintf("Failed to create symptom log: %v", err), http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditRepo := repository.NewAuditRepository(db)
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: userID, Valid: true},
			"create",
			"symptom_log",
			sql.NullInt64{Int64: symptom.ID, Valid: true},
			map[string]interface{}{
				"course_id":  symptom.CourseID,
				"pain_level": req.PainLevel,
			},
			r.RemoteAddr,
			r.UserAgent(),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(symptom); err != nil {
			log.Printf("Failed to encode symptom response: %v", err)
		}
	}
}

// HandleGetSymptom returns a single symptom log by ID
func HandleGetSymptom(db *database.DB) http.HandlerFunc {
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
			http.Error(w, "Invalid symptom log ID", http.StatusBadRequest)
			return
		}

		symptomRepo := repository.NewSymptomRepository(db)
		symptom, err := symptomRepo.GetByID(id, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Symptom log not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve symptom log", http.StatusInternalServerError)
			return
		}

		// Convert to JSON-serializable format
		response := map[string]interface{}{
			"id":            symptom.ID,
			"course_id":     symptom.CourseID,
			"logged_by":     nullInt64ToInt(symptom.LoggedBy),
			"timestamp":     symptom.Timestamp.Format(time.RFC3339),
			"pain_level":    nullInt64ToInt(symptom.PainLevel),
			"pain_location": nullStringToString(symptom.PainLocation),
			"pain_type":     nullStringToString(symptom.PainType),
			"symptoms":      nullStringToString(symptom.Symptoms),
			"notes":         nullStringToString(symptom.Notes),
			"created_at":    symptom.CreatedAt.Format(time.RFC3339),
			"updated_at":    symptom.UpdatedAt.Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode symptom response: %v", err)
		}
	}
}

// HandleUpdateSymptom updates an existing symptom log
func HandleUpdateSymptom(db *database.DB) http.HandlerFunc {
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
			http.Error(w, "Invalid symptom log ID", http.StatusBadRequest)
			return
		}

		var req UpdateSymptomRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate pain level if provided
		if req.PainLevel != nil && (*req.PainLevel < 1 || *req.PainLevel > 10) {
			http.Error(w, "pain_level must be between 1 and 10", http.StatusBadRequest)
			return
		}

		// Get existing symptom log
		symptomRepo := repository.NewSymptomRepository(db)
		symptom, err := symptomRepo.GetByID(id, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Symptom log not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve symptom log", http.StatusInternalServerError)
			return
		}

		// Update fields if provided
		if req.CourseID != nil {
			symptom.CourseID = *req.CourseID
		}
		if req.Timestamp != nil {
			timestamp, err := time.Parse(time.RFC3339, *req.Timestamp)
			if err != nil {
				http.Error(w, "Invalid timestamp format, use RFC3339", http.StatusBadRequest)
				return
			}
			symptom.Timestamp = timestamp
		}
		if req.PainLevel != nil {
			symptom.PainLevel = sql.NullInt64{Int64: int64(*req.PainLevel), Valid: true}
		}
		if req.PainLocation != nil {
			if *req.PainLocation == "" {
				symptom.PainLocation = sql.NullString{Valid: false}
			} else {
				symptom.PainLocation = sql.NullString{String: *req.PainLocation, Valid: true}
			}
		}
		if req.PainType != nil {
			if *req.PainType == "" {
				symptom.PainType = sql.NullString{Valid: false}
			} else {
				symptom.PainType = sql.NullString{String: *req.PainType, Valid: true}
			}
		}
		if req.Symptoms != nil {
			if len(req.Symptoms) == 0 {
				symptom.Symptoms = sql.NullString{Valid: false}
			} else {
				jsonBytes, err := json.Marshal(req.Symptoms)
				if err != nil {
					http.Error(w, "Failed to encode symptoms", http.StatusInternalServerError)
					return
				}
				symptom.Symptoms = sql.NullString{String: string(jsonBytes), Valid: true}
			}
		}
		if req.Notes != nil {
			if *req.Notes == "" {
				symptom.Notes = sql.NullString{Valid: false}
			} else {
				symptom.Notes = sql.NullString{String: *req.Notes, Valid: true}
			}
		}

		// Update symptom log
		if err := symptomRepo.Update(symptom, accountID); err != nil {
			http.Error(w, "Failed to update symptom log", http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditRepo := repository.NewAuditRepository(db)
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: userID, Valid: true},
			"update",
			"symptom_log",
			sql.NullInt64{Int64: id, Valid: true},
			map[string]interface{}{
				"course_id": symptom.CourseID,
			},
			r.RemoteAddr,
			r.UserAgent(),
		)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(symptom); err != nil {
			log.Printf("Failed to encode symptom response: %v", err)
		}
	}
}

// HandleDeleteSymptom deletes a symptom log
func HandleDeleteSymptom(db *database.DB) http.HandlerFunc {
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
			http.Error(w, "Invalid symptom log ID", http.StatusBadRequest)
			return
		}

		// Verify symptom log exists
		symptomRepo := repository.NewSymptomRepository(db)
		symptom, err := symptomRepo.GetByID(id, accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Symptom log not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve symptom log", http.StatusInternalServerError)
			return
		}

		// Delete symptom log
		if err := symptomRepo.Delete(id, accountID); err != nil {
			http.Error(w, "Failed to delete symptom log", http.StatusInternalServerError)
			return
		}

		// Create audit log
		auditRepo := repository.NewAuditRepository(db)
		_ = auditRepo.LogWithDetails(
			sql.NullInt64{Int64: userID, Valid: true},
			"delete",
			"symptom_log",
			sql.NullInt64{Int64: id, Valid: true},
			map[string]interface{}{
				"course_id": symptom.CourseID,
			},
			r.RemoteAddr,
			r.UserAgent(),
		)

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleGetRecentSymptoms returns recent symptom logs
func HandleGetRecentSymptoms(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		symptomRepo := repository.NewSymptomRepository(db)
		symptoms, err := symptomRepo.List(accountID, 10, 0)
		if err != nil {
			http.Error(w, "Failed to retrieve symptoms", http.StatusInternalServerError)
			return
		}

		// Return empty state HTML if no symptoms
		if len(symptoms) == 0 {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`
				<div style="text-align: center; padding: 2rem; color: var(--pico-muted-color);">
					<p>No symptoms logged yet.</p>
					<small>Use the form above to log your first symptom.</small>
				</div>
			`))
			return
		}

		// Build HTML for symptoms list
		w.Header().Set("Content-Type", "text/html")
		html := `<div style="display: flex; flex-direction: column; gap: 1rem;">`

		for _, symptom := range symptoms {
			symptomsJSON := ""
			if symptom.Symptoms.Valid {
				symptomsJSON = symptom.Symptoms.String
			}

			// Format timestamp
			formattedTime := symptom.Timestamp.Format("Jan 2, 2006 3:04 PM")
			timeAgo := formatTimeAgo(symptom.Timestamp)

			// Get pain level (handle null)
			painLevel := int64(0)
			if symptom.PainLevel.Valid {
				painLevel = symptom.PainLevel.Int64
			}

			html += fmt.Sprintf(`
				<article style="margin: 0;">
					<header style="margin-bottom: 0.5rem;">
						<div style="display: flex; justify-content: space-between; align-items: center;">
							<strong>%s</strong>
							<small>%s</small>
						</div>
					</header>
					<div style="margin-bottom: 0.5rem;">
						<strong>Pain Level:</strong> %d/10 &nbsp;
						<strong>Location:</strong> %s &nbsp;
						<strong>Type:</strong> %s
					</div>`,
				formattedTime,
				timeAgo,
				painLevel,
				nullStringValue(symptom.PainLocation, "N/A"),
				nullStringValue(symptom.PainType, "N/A"),
			)

			if symptomsJSON != "" && symptomsJSON != "[]" && symptomsJSON != "null" {
				// Parse JSON symptoms array
				var symptoms []string
				if err := json.Unmarshal([]byte(symptomsJSON), &symptoms); err == nil && len(symptoms) > 0 {
					html += `<div><strong>Symptoms:</strong> `
					for i, symptom := range symptoms {
						if i > 0 {
							html += ", "
						}
						// Format symptom names nicely
						formattedSymptom := strings.ReplaceAll(symptom, "_", " ")
						formattedSymptom = cases.Title(language.English).String(formattedSymptom)
						html += formattedSymptom
					}
					html += `</div>`
				}
			}

			if symptom.Notes.Valid && symptom.Notes.String != "" {
				html += fmt.Sprintf(`<div><strong>Notes:</strong> %s</div>`, symptom.Notes.String)
			}

			// Add action buttons
			html += fmt.Sprintf(`
				<footer style="margin-top: 1rem; padding-top: 1rem; border-top: 1px solid var(--pico-muted-border-color);">
					<div class="grid" style="grid-template-columns: 1fr 1fr;">
						<button data-action="delete-symptom" data-symptom-id="%d" class="outline secondary" style="font-size: 0.9rem;">
							Delete
						</button>
						<button data-action="edit-symptom" data-symptom-id="%d" class="outline" style="font-size: 0.9rem;">
							Edit
						</button>
					</div>
				</footer>
			`, symptom.ID, symptom.ID)

			html += `</article>`
		}

		html += `</div>`
		_, _ = w.Write([]byte(html))
	}
}

// nullStringValue returns the string value or a default if null
func nullStringValue(ns sql.NullString, defaultVal string) string {
	if ns.Valid {
		return ns.String
	}
	return defaultVal
}

// nullStringToString returns the string value or empty if null
func nullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// nullInt64ToInt returns the int64 value or 0 if null
func nullInt64ToInt(ni sql.NullInt64) *int64 {
	if ni.Valid {
		return &ni.Int64
	}
	return nil
}

// formatTimeAgo returns a human-readable time ago string
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)
	if duration.Hours() < 1 {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration.Hours() < 24 {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

// HandleGetSymptomTrends returns symptom trend data for charts
func HandleGetSymptomTrends(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		days := 30
		if daysParam := r.URL.Query().Get("days"); daysParam != "" {
			if d, err := strconv.Atoi(daysParam); err == nil && d > 0 {
				days = d
			}
		}

		symptomRepo := repository.NewSymptomRepository(db)
		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, -days)

		symptoms, err := symptomRepo.ListByDateRange(accountID, startDate, endDate, 1000, 0)
		if err != nil {
			http.Error(w, "Failed to retrieve symptom trends", http.StatusInternalServerError)
			return
		}

		// Build trend data
		dates := []string{}
		painLevels := []int{}

		for _, symptom := range symptoms {
			dates = append(dates, symptom.Timestamp.Format("2006-01-02"))
			if symptom.PainLevel.Valid {
				painLevels = append(painLevels, int(symptom.PainLevel.Int64))
			} else {
				painLevels = append(painLevels, 0)
			}
		}

		response := map[string]interface{}{
			"dates":      dates,
			"painLevels": painLevels,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode symptom trends response: %v", err)
		}
	}
}

// Helper function to convert *int to sql.NullInt64
func nullInt64Ptr(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}
