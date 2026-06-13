package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/middleware"
	"injection-tracker/internal/models"

	"github.com/go-chi/chi/v5"
)

// CreateInjectionRequest represents the request body for creating an injection
type CreateInjectionRequest struct {
	CourseID       int64    `json:"course_id"`
	Side           string   `json:"side"`
	Timestamp      *string  `json:"timestamp,omitempty"`
	SiteX          *float64 `json:"site_x,omitempty"`
	SiteY          *float64 `json:"site_y,omitempty"`
	PainLevel      *int     `json:"pain_level,omitempty"`
	HasKnots       bool     `json:"has_knots"`
	SiteReaction   *string  `json:"site_reaction,omitempty"`
	Notes          *string  `json:"notes,omitempty"`
	AdministeredBy *int64   `json:"administered_by,omitempty"`
}

// UpdateInjectionRequest represents the request body for updating an injection
type UpdateInjectionRequest struct {
	Side         *string  `json:"side,omitempty"`
	Timestamp    *string  `json:"timestamp,omitempty"`
	SiteX        *float64 `json:"site_x,omitempty"`
	SiteY        *float64 `json:"site_y,omitempty"`
	PainLevel    *int     `json:"pain_level,omitempty"`
	HasKnots     *bool    `json:"has_knots,omitempty"`
	SiteReaction *string  `json:"site_reaction,omitempty"`
	Notes        *string  `json:"notes,omitempty"`
}

// InjectionStatsResponse represents injection statistics
type InjectionStatsResponse struct {
	TotalInjections int               `json:"total_injections"`
	LeftCount       int               `json:"left_count"`
	RightCount      int               `json:"right_count"`
	AvgPainLevel    float64           `json:"avg_pain_level"`
	LastInjection   *models.Injection `json:"last_injection,omitempty"`
	FrequencyByDay  map[string]int    `json:"frequency_by_day"`
	PainTrend       []PainTrendPoint  `json:"pain_trend"`
}

// PainTrendPoint represents a point in the pain trend graph
type PainTrendPoint struct {
	Date      string  `json:"date"`
	PainLevel float64 `json:"pain_level"`
}

// HandleCreateInjection creates a new injection and automatically decrements inventory
func HandleCreateInjection(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user ID from context
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse request body
		var req CreateInjectionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if req.CourseID == 0 {
			http.Error(w, "course_id is required", http.StatusBadRequest)
			return
		}

		// Ensure the target course belongs to the caller's account (prevents
		// logging injections into another account's course).
		if !courseBelongsToAccount(db, req.CourseID, accountID) {
			http.Error(w, "Course not found", http.StatusNotFound)
			return
		}
		if req.Side != "left" && req.Side != "right" {
			http.Error(w, "side must be 'left' or 'right'", http.StatusBadRequest)
			return
		}

		// Validate optional fields
		if req.PainLevel != nil && (*req.PainLevel < 1 || *req.PainLevel > 10) {
			http.Error(w, "pain_level must be between 1 and 10", http.StatusBadRequest)
			return
		}
		if req.SiteReaction != nil {
			validReactions := map[string]bool{"none": true, "redness": true, "swelling": true, "bruising": true, "other": true}
			if !validReactions[*req.SiteReaction] {
				http.Error(w, "invalid site_reaction value", http.StatusBadRequest)
				return
			}
		}

		// Parse timestamp or use current time
		var timestamp time.Time
		if req.Timestamp != nil {
			var err error
			timestamp, err = time.Parse(time.RFC3339, *req.Timestamp)
			if err != nil {
				http.Error(w, "invalid timestamp format, use RFC3339", http.StatusBadRequest)
				return
			}
		} else {
			timestamp = time.Now()
		}

		// Set administered_by to current user if not specified
		if req.AdministeredBy == nil {
			req.AdministeredBy = &userID
		}

		// Begin transaction for atomic operation
		tx, err := db.BeginTx()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		// Insert injection
		result, err := tx.Exec(`
			INSERT INTO injections (
				course_id, administered_by, timestamp, side,
				site_x, site_y, pain_level, has_knots,
				site_reaction, notes, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			req.CourseID,
			nullInt64(req.AdministeredBy),
			timestamp,
			req.Side,
			nullFloat64(req.SiteX),
			nullFloat64(req.SiteY),
			nullInt(req.PainLevel),
			req.HasKnots,
			nullString(req.SiteReaction),
			nullString(req.Notes),
			time.Now(),
			time.Now(),
		)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create injection: %v", err), http.StatusInternalServerError)
			return
		}

		injectionID, err := result.LastInsertId()
		if err != nil {
			http.Error(w, "Failed to get injection ID", http.StatusInternalServerError)
			return
		}

		// **CRITICAL: Automatically decrement inventory**
		inventoryItems := []struct {
			itemType string
			amount   float64
			unit     string
		}{
			{"progesterone", 1.0, "mL"},
			{"draw_needle", 1.0, "count"},
			{"injection_needle", 1.0, "count"},
			{"syringe", 1.0, "count"},
			{"swab", 1.0, "count"},
		}

		for _, item := range inventoryItems {
			// Get current quantity for this account's item
			var currentQty float64
			err := tx.QueryRow(`
				SELECT quantity FROM inventory_items WHERE item_type = ? AND account_id = ?
			`, item.itemType, accountID).Scan(&currentQty)

			if err != nil {
				if err == sql.ErrNoRows {
					// Item doesn't exist for this account - initialize with 0 quantity
					_, err = tx.Exec(`
						INSERT INTO inventory_items (item_type, quantity, unit, account_id, created_at, updated_at)
						VALUES (?, ?, ?, ?, ?, ?)
					`, item.itemType, 0.0, item.unit, accountID, time.Now(), time.Now())
					if err != nil {
						http.Error(w, "Failed to initialize inventory", http.StatusInternalServerError)
						return
					}
					currentQty = 0.0
				} else {
					http.Error(w, "Failed to check inventory", http.StatusInternalServerError)
					return
				}
			}

			// Calculate new quantity (don't go below 0). Inventory tracking is
			// best-effort: a depleted/untracked count must not block logging.
			newQty := currentQty - item.amount
			if newQty < 0 {
				newQty = 0
			}

			// Update inventory
			_, err = tx.Exec(`
				UPDATE inventory_items
				SET quantity = ?, updated_at = ?
				WHERE item_type = ? AND account_id = ?
			`, newQty, time.Now(), item.itemType, accountID)
			if err != nil {
				http.Error(w, "Failed to update inventory", http.StatusInternalServerError)
				return
			}

			// Log inventory change
			_, err = tx.Exec(`
				INSERT INTO inventory_history (
					item_type, change_amount, quantity_before, quantity_after,
					reason, reference_id, reference_type, performed_by, timestamp, notes, account_id
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`,
				item.itemType,
				-item.amount,
				currentQty,
				newQty,
				"injection",
				injectionID,
				"injection",
				userID,
				time.Now(),
				fmt.Sprintf("Auto-decremented for injection #%d", injectionID),
				accountID,
			)
			if err != nil {
				http.Error(w, "Failed to log inventory history", http.StatusInternalServerError)
				return
			}
		}

		// Create audit log
		_, err = tx.Exec(`
			INSERT INTO audit_logs (user_id, action, entity_type, entity_id, details, timestamp)
			VALUES (?, ?, ?, ?, ?, ?)
		`,
			userID,
			"create",
			"injection",
			injectionID,
			fmt.Sprintf("Created injection on %s side with auto inventory decrement", req.Side),
			time.Now(),
		)
		if err != nil {
			http.Error(w, "Failed to create audit log", http.StatusInternalServerError)
			return
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		// Retrieve the created injection
		injection, err := getInjectionByID(db, injectionID, accountID)
		if err != nil {
			http.Error(w, "Injection created but failed to retrieve", http.StatusInternalServerError)
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(injection); err != nil {
			log.Printf("Failed to encode injection response: %v", err)
		}
	}
}

// HandleGetInjections returns a list of injections with optional filtering
func HandleGetInjections(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse query parameters
		accountID := middleware.GetAccountID(r.Context())
		if accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		courseID := r.URL.Query().Get("course_id")
		side := r.URL.Query().Get("side")
		startDate := r.URL.Query().Get("start_date")
		endDate := r.URL.Query().Get("end_date")
		limit := r.URL.Query().Get("limit")
		offset := r.URL.Query().Get("offset")

		// Build query, scoped to the caller's account via the injection's course.
		query := `
			SELECT i.id, i.course_id, i.administered_by, i.timestamp, i.side,
				i.site_x, i.site_y, i.pain_level, i.has_knots, i.site_reaction,
				i.notes, i.created_at, i.updated_at
			FROM injections i
			JOIN courses c ON c.id = i.course_id
			WHERE c.account_id = ?
		`
		args := []interface{}{accountID}

		if courseID != "" {
			query += " AND i.course_id = ?"
			args = append(args, courseID)
		}
		if side != "" {
			query += " AND i.side = ?"
			args = append(args, side)
		}
		if startDate != "" {
			query += " AND i.timestamp >= ?"
			args = append(args, startDate)
		}
		if endDate != "" {
			query += " AND i.timestamp <= ?"
			args = append(args, endDate)
		}

		query += " ORDER BY i.timestamp DESC"

		// Always bound the result set; cap and sanitize pagination params.
		lim := parsePositiveInt(limit, 200, 1000)
		off := parsePositiveInt(offset, 0, 1<<31)
		query += " LIMIT ? OFFSET ?"
		args = append(args, lim, off)

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, "Failed to query injections", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Get user's timezone preference
		userID := middleware.GetUserID(r.Context())
		userTimezone := GetUserTimezone(db, userID)

		injections := []models.Injection{}
		for rows.Next() {
			var inj models.Injection
			err := rows.Scan(
				&inj.ID,
				&inj.CourseID,
				&inj.AdministeredBy,
				&inj.Timestamp,
				&inj.Side,
				&inj.SiteX,
				&inj.SiteY,
				&inj.PainLevel,
				&inj.HasKnots,
				&inj.SiteReaction,
				&inj.Notes,
				&inj.CreatedAt,
				&inj.UpdatedAt,
			)
			if err != nil {
				http.Error(w, "Failed to scan injection", http.StatusInternalServerError)
				return
			}

			// Convert timestamps to user's timezone
			inj.Timestamp = ConvertToUserTZ(inj.Timestamp, userTimezone)
			inj.CreatedAt = ConvertToUserTZ(inj.CreatedAt, userTimezone)
			inj.UpdatedAt = ConvertToUserTZ(inj.UpdatedAt, userTimezone)

			injections = append(injections, inj)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(injections); err != nil {
			log.Printf("Failed to encode injections response: %v", err)
		}
	}
}

// HandleGetInjection returns a single injection by ID
func HandleGetInjection(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := middleware.GetAccountID(r.Context())
		if accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid injection ID", http.StatusBadRequest)
			return
		}

		injection, err := getInjectionByID(db, id, accountID)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Injection not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to get injection", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(injection); err != nil {
			log.Printf("Failed to encode injection response: %v", err)
		}
	}
}

// HandleUpdateInjection updates an existing injection
func HandleUpdateInjection(db *database.DB) http.HandlerFunc {
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
			http.Error(w, "Invalid injection ID", http.StatusBadRequest)
			return
		}

		// Parse request body
		var req UpdateInjectionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate side if provided
		if req.Side != nil && *req.Side != "left" && *req.Side != "right" {
			http.Error(w, "side must be 'left' or 'right'", http.StatusBadRequest)
			return
		}

		// Validate pain level if provided
		if req.PainLevel != nil && (*req.PainLevel < 1 || *req.PainLevel > 10) {
			http.Error(w, "pain_level must be between 1 and 10", http.StatusBadRequest)
			return
		}

		// Build update query dynamically
		updates := []string{}
		args := []interface{}{}

		if req.Side != nil {
			updates = append(updates, "side = ?")
			args = append(args, *req.Side)
		}
		if req.Timestamp != nil {
			timestamp, err := time.Parse(time.RFC3339, *req.Timestamp)
			if err != nil {
				http.Error(w, "invalid timestamp format", http.StatusBadRequest)
				return
			}
			updates = append(updates, "timestamp = ?")
			args = append(args, timestamp)
		}
		if req.SiteX != nil {
			updates = append(updates, "site_x = ?")
			args = append(args, *req.SiteX)
		}
		if req.SiteY != nil {
			updates = append(updates, "site_y = ?")
			args = append(args, *req.SiteY)
		}
		if req.PainLevel != nil {
			updates = append(updates, "pain_level = ?")
			args = append(args, *req.PainLevel)
		}
		if req.HasKnots != nil {
			updates = append(updates, "has_knots = ?")
			args = append(args, *req.HasKnots)
		}
		if req.SiteReaction != nil {
			updates = append(updates, "site_reaction = ?")
			args = append(args, *req.SiteReaction)
		}
		if req.Notes != nil {
			updates = append(updates, "notes = ?")
			args = append(args, *req.Notes)
		}

		if len(updates) == 0 {
			http.Error(w, "No fields to update", http.StatusBadRequest)
			return
		}

		updates = append(updates, "updated_at = ?")
		args = append(args, time.Now())
		args = append(args, id)
		args = append(args, accountID)

		// Scope the update to the caller's account via the injection's course.
		query := "UPDATE injections SET " + joinStrings(updates, ", ") +
			" WHERE id = ? AND EXISTS (SELECT 1 FROM courses c WHERE c.id = injections.course_id AND c.account_id = ?)"

		result, err := db.Exec(query, args...)
		if err != nil {
			http.Error(w, "Failed to update injection", http.StatusInternalServerError)
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "Failed to update injection", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			http.Error(w, "Injection not found", http.StatusNotFound)
			return
		}

		// Create audit log
		_, _ = db.Exec(`
			INSERT INTO audit_logs (user_id, action, entity_type, entity_id, details, timestamp)
			VALUES (?, ?, ?, ?, ?, ?)
		`, userID, "update", "injection", id, "Updated injection", time.Now())

		// Return updated injection
		injection, err := getInjectionByID(db, id, accountID)
		if err != nil {
			http.Error(w, "Failed to retrieve updated injection", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(injection); err != nil {
			log.Printf("Failed to encode injection response: %v", err)
		}
	}
}

// HandleDeleteInjection deletes an injection and ROLLBACKS inventory changes
func HandleDeleteInjection(db *database.DB) http.HandlerFunc {
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
			http.Error(w, "Invalid injection ID", http.StatusBadRequest)
			return
		}

		// Verify the injection exists and belongs to the caller's account before
		// touching any inventory.
		if _, err := getInjectionByID(db, id, accountID); err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Injection not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to get injection", http.StatusInternalServerError)
			return
		}

		// Begin transaction
		tx, err := db.BeginTx()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		// Get inventory changes for this injection (scoped to the account)
		rows, err := tx.Query(`
			SELECT item_type, change_amount, quantity_before
			FROM inventory_history
			WHERE reference_id = ? AND reference_type = 'injection' AND account_id = ?
		`, id, accountID)
		if err != nil {
			http.Error(w, "Failed to query inventory history", http.StatusInternalServerError)
			return
		}

		type inventoryRollback struct {
			itemType  string
			amount    float64
			qtyBefore float64
		}
		rollbacks := []inventoryRollback{}

		for rows.Next() {
			var rb inventoryRollback
			if err := rows.Scan(&rb.itemType, &rb.amount, &rb.qtyBefore); err != nil {
				rows.Close()
				http.Error(w, "Failed to scan inventory history", http.StatusInternalServerError)
				return
			}
			rollbacks = append(rollbacks, rb)
		}
		rows.Close()

		// Rollback inventory changes
		for _, rb := range rollbacks {
			// Get current quantity (scoped to the account)
			var currentQty float64
			err := tx.QueryRow(`SELECT quantity FROM inventory_items WHERE item_type = ? AND account_id = ?`, rb.itemType, accountID).Scan(&currentQty)
			if err != nil {
				http.Error(w, "Failed to get current inventory", http.StatusInternalServerError)
				return
			}

			// Reverse the change (add back what was subtracted)
			newQty := currentQty - rb.amount

			// Update inventory
			_, err = tx.Exec(`
				UPDATE inventory_items
				SET quantity = ?, updated_at = ?
				WHERE item_type = ? AND account_id = ?
			`, newQty, time.Now(), rb.itemType, accountID)
			if err != nil {
				http.Error(w, "Failed to rollback inventory", http.StatusInternalServerError)
				return
			}

			// Log the rollback
			_, err = tx.Exec(`
				INSERT INTO inventory_history (
					item_type, change_amount, quantity_before, quantity_after,
					reason, reference_id, reference_type, performed_by, timestamp, notes, account_id
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`,
				rb.itemType,
				-rb.amount, // Opposite of the original change
				currentQty,
				newQty,
				"other",
				id,
				"injection",
				userID,
				time.Now(),
				fmt.Sprintf("Rollback for deleted injection #%d", id),
				accountID,
			)
			if err != nil {
				http.Error(w, "Failed to log inventory rollback", http.StatusInternalServerError)
				return
			}
		}

		// Delete the injection (scoped to the account via its course)
		result, err := tx.Exec(`
			DELETE FROM injections
			WHERE id = ?
			AND EXISTS (SELECT 1 FROM courses c WHERE c.id = injections.course_id AND c.account_id = ?)
		`, id, accountID)
		if err != nil {
			http.Error(w, "Failed to delete injection", http.StatusInternalServerError)
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "Failed to delete injection", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			http.Error(w, "Injection not found", http.StatusNotFound)
			return
		}

		// Create audit log
		_, _ = tx.Exec(`
			INSERT INTO audit_logs (user_id, action, entity_type, entity_id, details, timestamp)
			VALUES (?, ?, ?, ?, ?, ?)
		`, userID, "delete", "injection", id, "Deleted injection with inventory rollback", time.Now())

		// Commit transaction
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleGetRecentInjections returns the last 10 injections
func HandleGetRecentInjections(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := middleware.GetAccountID(r.Context())
		if accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		rows, err := db.Query(`
			SELECT i.id, i.course_id, i.administered_by, i.timestamp, i.side,
				i.site_x, i.site_y, i.pain_level, i.has_knots, i.site_reaction,
				i.notes, i.created_at, i.updated_at
			FROM injections i
			JOIN courses c ON c.id = i.course_id
			WHERE c.account_id = ?
			ORDER BY i.timestamp DESC
			LIMIT 10
		`, accountID)
		if err != nil {
			http.Error(w, "Failed to query recent injections", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		injections := []models.Injection{}
		for rows.Next() {
			var inj models.Injection
			err := rows.Scan(
				&inj.ID,
				&inj.CourseID,
				&inj.AdministeredBy,
				&inj.Timestamp,
				&inj.Side,
				&inj.SiteX,
				&inj.SiteY,
				&inj.PainLevel,
				&inj.HasKnots,
				&inj.SiteReaction,
				&inj.Notes,
				&inj.CreatedAt,
				&inj.UpdatedAt,
			)
			if err != nil {
				http.Error(w, "Failed to scan injection", http.StatusInternalServerError)
				return
			}
			injections = append(injections, inj)
		}

		// Check if request wants HTML (from HTMX)
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html")
			if len(injections) == 0 {
				_, _ = w.Write([]byte(`<p style="text-align: center; color: var(--pico-muted-color);">No injections recorded yet.</p>`))
				return
			}

			html := `<div class="overflow-auto"><table><thead><tr>
				<th>Date</th><th>Side</th><th>Pain</th><th>Notes</th>
			</tr></thead><tbody>`

			for _, inj := range injections {
				pain := "N/A"
				if inj.PainLevel.Valid {
					pain = fmt.Sprintf("%d/10", inj.PainLevel.Int64)
				}
				notes := ""
				if inj.Notes.Valid {
					notes = truncateRunes(inj.Notes.String, 50)
				}
				html += fmt.Sprintf(`<tr>
					<td>%s</td>
					<td>%s</td>
					<td>%s</td>
					<td>%s</td>
				</tr>`, inj.Timestamp.Format("Jan 2, 2006 3:04 PM"), template.HTMLEscapeString(inj.Side), pain, template.HTMLEscapeString(notes))
			}

			html += `</tbody></table></div>`
			_, _ = w.Write([]byte(html))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(injections); err != nil {
			log.Printf("Failed to encode injections response: %v", err)
		}
	}
}

// HandleGetInjectionStats returns statistics for injections
func HandleGetInjectionStats(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID := middleware.GetAccountID(r.Context())
		if accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		courseID := r.URL.Query().Get("course_id")

		stats := InjectionStatsResponse{
			FrequencyByDay: make(map[string]int),
			PainTrend:      []PainTrendPoint{},
		}

		// Base scoped to the caller's account via the injection's course.
		whereClause := " FROM injections i JOIN courses c ON c.id = i.course_id WHERE c.account_id = ?"
		args := []interface{}{accountID}
		if courseID != "" {
			whereClause += " AND i.course_id = ?"
			args = append(args, courseID)
		}

		// Get total count
		query := "SELECT COUNT(*)" + whereClause
		_ = db.QueryRow(query, args...).Scan(&stats.TotalInjections)

		// Get left/right counts
		// Note: Assuming 'left' and 'right' are lowercase in DB as enforced by HandleCreateInjection
		query = "SELECT COUNT(*)" + whereClause + " AND i.side = 'left'"
		_ = db.QueryRow(query, args...).Scan(&stats.LeftCount)

		query = "SELECT COUNT(*)" + whereClause + " AND i.side = 'right'"
		_ = db.QueryRow(query, args...).Scan(&stats.RightCount)

		// Get average pain level
		query = "SELECT AVG(CAST(i.pain_level AS REAL))" + whereClause + " AND i.pain_level IS NOT NULL"
		_ = db.QueryRow(query, args...).Scan(&stats.AvgPainLevel)

		// Get last injection
		query = `
			SELECT i.id, i.course_id, i.administered_by, i.timestamp, i.side,
				i.site_x, i.site_y, i.pain_level, i.has_knots, i.site_reaction,
				i.notes, i.created_at, i.updated_at
		` + whereClause + " ORDER BY i.timestamp DESC LIMIT 1"

		var lastInj models.Injection
		err := db.QueryRow(query, args...).Scan(
			&lastInj.ID,
			&lastInj.CourseID,
			&lastInj.AdministeredBy,
			&lastInj.Timestamp,
			&lastInj.Side,
			&lastInj.SiteX,
			&lastInj.SiteY,
			&lastInj.PainLevel,
			&lastInj.HasKnots,
			&lastInj.SiteReaction,
			&lastInj.Notes,
			&lastInj.CreatedAt,
			&lastInj.UpdatedAt,
		)
		if err == nil {
			stats.LastInjection = &lastInj
		}

		// Get frequency by day
		query = `
			SELECT DATE(i.timestamp) as day, COUNT(*) as count
		` + whereClause + `
			GROUP BY DATE(i.timestamp)
			ORDER BY day DESC
			LIMIT 30
		`
		rows, err := db.Query(query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var day string
				var count int
				if err := rows.Scan(&day, &count); err == nil {
					stats.FrequencyByDay[day] = count
				}
			}
		}

		// Get pain trend (last 30 days)
		query = `
			SELECT DATE(i.timestamp) as day, AVG(CAST(i.pain_level AS REAL)) as avg_pain
		` + whereClause + ` AND i.pain_level IS NOT NULL
			GROUP BY DATE(i.timestamp)
			ORDER BY day DESC
			LIMIT 30
		`
		rows, err = db.Query(query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var point PainTrendPoint
				if err := rows.Scan(&point.Date, &point.PainLevel); err == nil {
					stats.PainTrend = append(stats.PainTrend, point)
				}
			}
		}

		// Check if request wants HTML (from HTMX)
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html")
			html := fmt.Sprintf(`
				<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 1rem;">
					<div style="text-align: center;">
						<div style="font-size: 0.85rem; color: var(--color-text-secondary); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 0.5rem;">Total</div>
						<div style="font-size: 2rem; font-weight: bold; color: var(--brand-primary); line-height: 1;">%d</div>
					</div>
					<div style="text-align: center;">
						<div style="font-size: 0.85rem; color: var(--color-text-secondary); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 0.5rem;">Left</div>
						<div style="font-size: 2rem; font-weight: bold; color: var(--color-text-primary); line-height: 1;">%d</div>
					</div>
					<div style="text-align: center;">
						<div style="font-size: 0.85rem; color: var(--color-text-secondary); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 0.5rem;">Right</div>
						<div style="font-size: 2rem; font-weight: bold; color: var(--color-text-primary); line-height: 1;">%d</div>
					</div>
					<div style="text-align: center;">
						<div style="font-size: 0.85rem; color: var(--color-text-secondary); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 0.5rem;">Avg Pain</div>
						<div style="font-size: 2rem; font-weight: bold; color: var(--color-text-primary); line-height: 1;">%.1f<small style="font-size: 1rem; color: var(--color-text-muted);">/10</small></div>
					</div>
				</div>
			`, stats.TotalInjections, stats.LeftCount, stats.RightCount, stats.AvgPainLevel)
			_, _ = w.Write([]byte(html))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			log.Printf("Failed to encode stats response: %v", err)
		}
	}
}

// Helper functions

// parsePositiveInt parses s as a non-negative integer, returning defaultVal when
// empty/invalid and capping the result at maxVal to bound resource usage.
func parsePositiveInt(s string, defaultVal, maxVal int) int {
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return defaultVal
	}
	if n > maxVal {
		return maxVal
	}
	return n
}

// courseBelongsToAccount reports whether the given course exists and is owned by
// the account, used to prevent cross-account writes that reference a course_id.
func courseBelongsToAccount(db *database.DB, courseID, accountID int64) bool {
	var exists int
	err := db.QueryRow(
		`SELECT 1 FROM courses WHERE id = ? AND account_id = ?`,
		courseID, accountID,
	).Scan(&exists)
	return err == nil
}

func getInjectionByID(db *database.DB, id int64, accountID int64) (*models.Injection, error) {
	var inj models.Injection
	err := db.QueryRow(`
		SELECT i.id, i.course_id, i.administered_by, i.timestamp, i.side,
			i.site_x, i.site_y, i.pain_level, i.has_knots, i.site_reaction,
			i.notes, i.created_at, i.updated_at
		FROM injections i
		JOIN courses c ON c.id = i.course_id
		WHERE i.id = ? AND c.account_id = ?
	`, id, accountID).Scan(
		&inj.ID,
		&inj.CourseID,
		&inj.AdministeredBy,
		&inj.Timestamp,
		&inj.Side,
		&inj.SiteX,
		&inj.SiteY,
		&inj.PainLevel,
		&inj.HasKnots,
		&inj.SiteReaction,
		&inj.Notes,
		&inj.CreatedAt,
		&inj.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inj, nil
}

func nullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func nullFloat64(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}

func nullString(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *v, Valid: true}
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
