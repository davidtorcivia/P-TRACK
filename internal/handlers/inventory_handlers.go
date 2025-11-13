package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/middleware"
	"injection-tracker/internal/models"

	"github.com/go-chi/chi/v5"
)

// InventoryItemResponse represents the API response for inventory items
type InventoryItemResponse struct {
	ID                 int64      `json:"id"`
	ItemType           string     `json:"item_type"`
	Quantity           float64    `json:"quantity"`
	Unit               string     `json:"unit"`
	ExpirationDate     *time.Time `json:"expiration_date,omitempty"`
	LotNumber          *string    `json:"lot_number,omitempty"`
	LowStockThreshold  *float64   `json:"low_stock_threshold,omitempty"`
	Notes              *string    `json:"notes,omitempty"`
	IsLowStock         bool       `json:"is_low_stock"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// UpdateInventoryRequest represents the request to update an inventory item
type UpdateInventoryRequest struct {
	Quantity          *float64   `json:"quantity,omitempty"`
	ExpirationDate    *time.Time `json:"expiration_date,omitempty"`
	LotNumber         *string    `json:"lot_number,omitempty"`
	LowStockThreshold *float64   `json:"low_stock_threshold,omitempty"`
	Notes             *string    `json:"notes,omitempty"`
}

// FlexibleDate is a custom type that can unmarshal various date formats
type FlexibleDate struct {
	time.Time
}

// UnmarshalJSON implements custom JSON unmarshaling to handle multiple date formats
func (fd *FlexibleDate) UnmarshalJSON(b []byte) error {
	s := string(b)
	// Remove quotes
	if len(s) < 2 {
		return fmt.Errorf("invalid date string")
	}
	s = s[1 : len(s)-1]

	// Try multiple formats
	formats := []string{
		"2006-01-02",           // YYYY-MM-DD (from HTML date input)
		time.RFC3339,           // RFC3339
		"2006-01-02T15:04:05Z", // ISO 8601
		"01/02/2006",           // MM/DD/YYYY
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			fd.Time = t
			return nil
		}
	}

	return fmt.Errorf("unable to parse date: %s", s)
}

// AdjustInventoryRequest represents a manual inventory adjustment
type AdjustInventoryRequest struct {
	ChangeAmount   float64       `json:"change_amount"`
	Reason         string        `json:"reason"`
	Notes          *string       `json:"notes,omitempty"`
	ExpirationDate *FlexibleDate `json:"expiration_date,omitempty"`
	LotNumber      *string       `json:"lot_number,omitempty"`
}

// InventoryHistoryResponse represents an inventory history entry
type InventoryHistoryResponse struct {
	ID             int64      `json:"id"`
	ItemType       string     `json:"item_type"`
	ChangeAmount   float64    `json:"change_amount"`
	QuantityBefore float64    `json:"quantity_before"`
	QuantityAfter  float64    `json:"quantity_after"`
	Reason         string     `json:"reason"`
	ReferenceID    *int64     `json:"reference_id,omitempty"`
	ReferenceType  *string    `json:"reference_type,omitempty"`
	PerformedBy    *int64     `json:"performed_by,omitempty"`
	Timestamp      time.Time  `json:"timestamp"`
	Notes          *string    `json:"notes,omitempty"`
}

// InventoryAlertResponse represents a low stock alert
type InventoryAlertResponse struct {
	ItemType          string  `json:"item_type"`
	Quantity          float64 `json:"quantity"`
	LowStockThreshold float64 `json:"low_stock_threshold"`
	Unit              string  `json:"unit"`
	Severity          string  `json:"severity"` // "warning", "critical"
}

// HandleGetInventory returns all inventory items with current quantities
func HandleGetInventory(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Query all inventory items
		rows, err := db.Query(`
			SELECT id, item_type, quantity, unit, expiration_date,
				lot_number, low_stock_threshold, notes, created_at, updated_at
			FROM inventory_items
			ORDER BY item_type
		`)
		if err != nil {
			http.Error(w, "Failed to query inventory", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		items := []InventoryItemResponse{}
		for rows.Next() {
			var item models.InventoryItem
			err := rows.Scan(
				&item.ID,
				&item.ItemType,
				&item.Quantity,
				&item.Unit,
				&item.ExpirationDate,
				&item.LotNumber,
				&item.LowStockThreshold,
				&item.Notes,
				&item.CreatedAt,
				&item.UpdatedAt,
			)
			if err != nil {
				http.Error(w, "Failed to scan inventory item", http.StatusInternalServerError)
				return
			}

			// Convert to response format
			response := inventoryItemToResponse(&item)
			items = append(items, response)
		}

		if err := rows.Err(); err != nil {
			http.Error(w, "Error iterating inventory items", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	}
}

// HandleUpdateInventory updates a specific inventory item
func HandleUpdateInventory(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		itemType := chi.URLParam(r, "itemType")
		if !isValidItemType(itemType) {
			http.Error(w, "Invalid item type", http.StatusBadRequest)
			return
		}

		// Parse request body
		var req UpdateInventoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate quantity is non-negative if provided
		if req.Quantity != nil && *req.Quantity < 0 {
			http.Error(w, "Quantity cannot be negative", http.StatusBadRequest)
			return
		}

		// Validate low stock threshold is non-negative if provided
		if req.LowStockThreshold != nil && *req.LowStockThreshold < 0 {
			http.Error(w, "Low stock threshold cannot be negative", http.StatusBadRequest)
			return
		}

		// Build update query dynamically
		updates := []string{}
		args := []interface{}{}

		if req.Quantity != nil {
			updates = append(updates, "quantity = ?")
			args = append(args, *req.Quantity)
		}
		if req.ExpirationDate != nil {
			updates = append(updates, "expiration_date = ?")
			args = append(args, *req.ExpirationDate)
		}
		if req.LotNumber != nil {
			updates = append(updates, "lot_number = ?")
			args = append(args, *req.LotNumber)
		}
		if req.LowStockThreshold != nil {
			updates = append(updates, "low_stock_threshold = ?")
			args = append(args, *req.LowStockThreshold)
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
		args = append(args, itemType)

		query := "UPDATE inventory_items SET " + joinStrings(updates, ", ") + " WHERE item_type = ?"

		result, err := db.Exec(query, args...)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to update inventory: %v", err), http.StatusInternalServerError)
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil || rowsAffected == 0 {
			http.Error(w, "Inventory item not found", http.StatusNotFound)
			return
		}

		// Create audit log
		_, _ = db.Exec(`
			INSERT INTO audit_logs (user_id, action, entity_type, entity_id, details, timestamp)
			VALUES (?, ?, ?, ?, ?, ?)
		`, userID, "update", "inventory", 0, fmt.Sprintf("Updated inventory for %s", itemType), time.Now())

		// Return updated item
		item, err := getInventoryItemByType(db, itemType)
		if err != nil {
			http.Error(w, "Failed to retrieve updated inventory item", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(inventoryItemToResponse(item))
	}
}

// HandleGetInventoryHistory returns the history for a specific item type
func HandleGetInventoryHistory(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		itemType := chi.URLParam(r, "itemType")
		if !isValidItemType(itemType) {
			http.Error(w, "Invalid item type", http.StatusBadRequest)
			return
		}

		// Parse query parameters for pagination
		limit := r.URL.Query().Get("limit")
		if limit == "" {
			limit = "50" // Default limit
		}

		// Query history
		rows, err := db.Query(`
			SELECT id, item_type, change_amount, quantity_before, quantity_after,
				reason, reference_id, reference_type, performed_by, timestamp, notes
			FROM inventory_history
			WHERE item_type = ?
			ORDER BY timestamp DESC
			LIMIT ?
		`, itemType, limit)
		if err != nil {
			http.Error(w, "Failed to query inventory history", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		history := []InventoryHistoryResponse{}
		for rows.Next() {
			var h models.InventoryHistory
			err := rows.Scan(
				&h.ID,
				&h.ItemType,
				&h.ChangeAmount,
				&h.QuantityBefore,
				&h.QuantityAfter,
				&h.Reason,
				&h.ReferenceID,
				&h.ReferenceType,
				&h.PerformedBy,
				&h.Timestamp,
				&h.Notes,
			)
			if err != nil {
				http.Error(w, "Failed to scan history entry", http.StatusInternalServerError)
				return
			}

			response := InventoryHistoryResponse{
				ID:             h.ID,
				ItemType:       h.ItemType,
				ChangeAmount:   h.ChangeAmount,
				QuantityBefore: h.QuantityBefore,
				QuantityAfter:  h.QuantityAfter,
				Reason:         h.Reason,
				Timestamp:      h.Timestamp,
			}

			if h.ReferenceID.Valid {
				response.ReferenceID = &h.ReferenceID.Int64
			}
			if h.ReferenceType.Valid {
				response.ReferenceType = &h.ReferenceType.String
			}
			if h.PerformedBy.Valid {
				response.PerformedBy = &h.PerformedBy.Int64
			}
			if h.Notes.Valid {
				response.Notes = &h.Notes.String
			}

			history = append(history, response)
		}

		if err := rows.Err(); err != nil {
			http.Error(w, "Error iterating history entries", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(history)
	}
}

// HandleAdjustInventory performs a manual inventory adjustment with reason
func HandleAdjustInventory(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		itemType := chi.URLParam(r, "itemType")
		if !isValidItemType(itemType) {
			http.Error(w, "Invalid item type", http.StatusBadRequest)
			return
		}

		// Parse request body
		var req AdjustInventoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if req.ChangeAmount == 0 {
			http.Error(w, "change_amount is required and cannot be zero", http.StatusBadRequest)
			return
		}
		if req.Reason == "" {
			http.Error(w, "reason is required", http.StatusBadRequest)
			return
		}

		// Valid reasons for manual adjustment
		validReasons := map[string]bool{
			"restock":            true,
			"manual_adjustment":  true,
			"correction":         true,
			"expired":            true,
			"damaged":            true,
			"initial_setup":      true,
		}
		if !validReasons[req.Reason] {
			http.Error(w, "Invalid reason. Must be one of: restock, manual_adjustment, correction, expired, damaged, initial_setup", http.StatusBadRequest)
			return
		}

		// Begin transaction
		tx, err := db.BeginTx()
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// Get current quantity (or create item if doesn't exist)
		var currentQty float64
		var unit string
		err = tx.QueryRow(`SELECT quantity, unit FROM inventory_items WHERE item_type = ?`, itemType).Scan(&currentQty, &unit)

		if err == sql.ErrNoRows {
			// Item doesn't exist - create it with default unit and optional fields
			unit = getDefaultUnit(itemType)
			now := time.Now()

			insertQuery := `INSERT INTO inventory_items (item_type, quantity, unit`
			valuePlaceholders := `VALUES (?, ?, ?`
			insertValues := []interface{}{itemType, 0, unit}

			if req.ExpirationDate != nil {
				insertQuery += `, expiration_date`
				valuePlaceholders += `, ?`
				insertValues = append(insertValues, req.ExpirationDate.Time)
			}
			if req.LotNumber != nil {
				insertQuery += `, lot_number`
				valuePlaceholders += `, ?`
				insertValues = append(insertValues, *req.LotNumber)
			}

			insertQuery += `, created_at, updated_at) `
			valuePlaceholders += `, ?, ?)`
			insertValues = append(insertValues, now, now)

			_, err = tx.Exec(insertQuery+valuePlaceholders, insertValues...)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create inventory item: %v", err), http.StatusInternalServerError)
				return
			}
			currentQty = 0
		} else if err != nil {
			http.Error(w, "Failed to get current inventory", http.StatusInternalServerError)
			return
		}

		// Calculate new quantity
		newQty := currentQty + req.ChangeAmount

		// Validate new quantity is non-negative
		if newQty < 0 {
			http.Error(w, fmt.Sprintf("Cannot adjust: would result in negative quantity (%.2f)", newQty), http.StatusBadRequest)
			return
		}

		// Update inventory (including optional expiration_date and lot_number)
		updateQuery := `UPDATE inventory_items SET quantity = ?, updated_at = ?`
		updateArgs := []interface{}{newQty, time.Now()}

		if req.ExpirationDate != nil {
			updateQuery += `, expiration_date = ?`
			updateArgs = append(updateArgs, req.ExpirationDate.Time)
		}
		if req.LotNumber != nil {
			updateQuery += `, lot_number = ?`
			updateArgs = append(updateArgs, *req.LotNumber)
		}

		updateQuery += ` WHERE item_type = ?`
		updateArgs = append(updateArgs, itemType)

		_, err = tx.Exec(updateQuery, updateArgs...)
		if err != nil {
			http.Error(w, "Failed to update inventory", http.StatusInternalServerError)
			return
		}

		// Log the adjustment
		_, err = tx.Exec(`
			INSERT INTO inventory_history (
				item_type, change_amount, quantity_before, quantity_after,
				reason, performed_by, timestamp, notes
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
			itemType,
			req.ChangeAmount,
			currentQty,
			newQty,
			req.Reason,
			userID,
			time.Now(),
			nullString(req.Notes),
		)
		if err != nil {
			http.Error(w, "Failed to log inventory adjustment", http.StatusInternalServerError)
			return
		}

		// Create audit log
		_, _ = tx.Exec(`
			INSERT INTO audit_logs (user_id, action, entity_type, entity_id, details, timestamp)
			VALUES (?, ?, ?, ?, ?, ?)
		`,
			userID,
			"adjust",
			"inventory",
			0,
			fmt.Sprintf("Adjusted %s inventory by %.2f (reason: %s)", itemType, req.ChangeAmount, req.Reason),
			time.Now(),
		)

		// Commit transaction
		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}

		// Return updated item
		item, err := getInventoryItemByType(db, itemType)
		if err != nil {
			http.Error(w, "Adjustment successful but failed to retrieve updated item", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(inventoryItemToResponse(item))
	}
}

// HandleGetInventoryAlerts returns items below low stock threshold
func HandleGetInventoryAlerts(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Query items where quantity is below threshold
		rows, err := db.Query(`
			SELECT item_type, quantity, low_stock_threshold, unit
			FROM inventory_items
			WHERE low_stock_threshold IS NOT NULL
			  AND quantity <= low_stock_threshold
			ORDER BY
				CASE
					WHEN quantity <= low_stock_threshold / 2 THEN 1
					ELSE 2
				END,
				quantity ASC
		`)
		if err != nil {
			http.Error(w, "Failed to query inventory alerts", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		alerts := []InventoryAlertResponse{}
		for rows.Next() {
			var alert InventoryAlertResponse
			var threshold sql.NullFloat64
			err := rows.Scan(
				&alert.ItemType,
				&alert.Quantity,
				&threshold,
				&alert.Unit,
			)
			if err != nil {
				http.Error(w, "Failed to scan alert", http.StatusInternalServerError)
				return
			}

			if threshold.Valid {
				alert.LowStockThreshold = threshold.Float64

				// Determine severity
				if alert.Quantity <= alert.LowStockThreshold/2 {
					alert.Severity = "critical"
				} else {
					alert.Severity = "warning"
				}
			}

			alerts = append(alerts, alert)
		}

		if err := rows.Err(); err != nil {
			http.Error(w, "Error iterating alerts", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts)
	}
}

// Helper functions

func isValidItemType(itemType string) bool {
	validTypes := map[string]bool{
		"progesterone":      true,
		"draw_needle":       true,
		"injection_needle":  true,
		"syringe":           true,
		"swab":              true,
		"gauze":             true,
	}
	return validTypes[itemType]
}

func getDefaultUnit(itemType string) string {
	if itemType == "progesterone" {
		return "mL"
	}
	return "count"
}

func getInventoryItemByType(db *database.DB, itemType string) (*models.InventoryItem, error) {
	var item models.InventoryItem
	err := db.QueryRow(`
		SELECT id, item_type, quantity, unit, expiration_date,
			lot_number, low_stock_threshold, notes, created_at, updated_at
		FROM inventory_items
		WHERE item_type = ?
	`, itemType).Scan(
		&item.ID,
		&item.ItemType,
		&item.Quantity,
		&item.Unit,
		&item.ExpirationDate,
		&item.LotNumber,
		&item.LowStockThreshold,
		&item.Notes,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func inventoryItemToResponse(item *models.InventoryItem) InventoryItemResponse {
	response := InventoryItemResponse{
		ID:        item.ID,
		ItemType:  item.ItemType,
		Quantity:  item.Quantity,
		Unit:      item.Unit,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}

	if item.ExpirationDate.Valid {
		response.ExpirationDate = &item.ExpirationDate.Time
	}
	if item.LotNumber.Valid {
		response.LotNumber = &item.LotNumber.String
	}
	if item.LowStockThreshold.Valid {
		response.LowStockThreshold = &item.LowStockThreshold.Float64
		// Check if low stock
		response.IsLowStock = item.Quantity <= item.LowStockThreshold.Float64
	}
	if item.Notes.Valid {
		response.Notes = &item.Notes.String
	}

	return response
}

// HandleUpdateInventorySettings updates inventory auto-deduction settings
func HandleUpdateInventorySettings(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// For now, just return success
		// TODO: Implement settings storage in database
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "Settings updated successfully"}`))
	}
}

// HandleGetRecentInventoryChanges returns recent inventory changes
func HandleGetRecentInventoryChanges(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get recent inventory changes
		rows, err := db.Query(`
			SELECT item_type, change_amount, reason, timestamp, notes
			FROM inventory_history
			ORDER BY timestamp DESC
			LIMIT 10
		`)
		if err != nil {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<p>Error loading inventory changes</p>`))
			return
		}
		defer rows.Close()

		type Change struct {
			ItemType     string
			ChangeAmount float64
			Reason       string
			Timestamp    time.Time
			Notes        sql.NullString
		}

		changes := []Change{}
		for rows.Next() {
			var change Change
			if err := rows.Scan(&change.ItemType, &change.ChangeAmount, &change.Reason, &change.Timestamp, &change.Notes); err == nil {
				changes = append(changes, change)
			}
		}

		if len(changes) == 0 {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`
				<div style="text-align: center; padding: 2rem; color: var(--pico-muted-color);">
					<p>No recent changes.</p>
				</div>
			`))
			return
		}

		// Display names for item types
		displayNames := map[string]string{
			"progesterone":     "Progesterone",
			"draw_needle":      "Draw Needles",
			"injection_needle": "Injection Needles",
			"syringe":          "Syringes",
			"swab":             "Alcohol Swabs",
			"gauze":            "Gauze Pads",
		}

		w.Header().Set("Content-Type", "text/html")
		html := `<div style="display: flex; flex-direction: column; gap: 0.5rem;">`

		for _, change := range changes {
			itemName := displayNames[change.ItemType]
			if itemName == "" {
				itemName = change.ItemType
			}

			sign := "+"
			color := "var(--pico-ins-color)"
			if change.ChangeAmount < 0 {
				sign = ""
				color = "var(--pico-del-color)"
			}

			html += `<article style="margin: 0; padding: 0.75rem;">`
			html += `<div style="display: flex; justify-content: space-between; align-items: start;">`
			html += `<div><strong>` + itemName + `</strong> `
			html += `<span style="color: ` + color + `;">` + sign + fmt.Sprintf("%.1f", change.ChangeAmount) + `</span>`
			html += `<br><small style="color: var(--pico-muted-color);">` + strings.Title(strings.ReplaceAll(change.Reason, "_", " ")) + `</small>`

			if change.Notes.Valid && change.Notes.String != "" {
				html += `<br><small>` + change.Notes.String + `</small>`
			}

			html += `</div>`
			html += `<small style="color: var(--pico-muted-color); white-space: nowrap;">` + formatTimeAgo(change.Timestamp) + `</small>`
			html += `</div></article>`
		}

		html += `</div>`
		w.Write([]byte(html))
	}
}

// HandleGetAllInventoryHistory returns all inventory history (for /api/inventory/history)
func HandleGetAllInventoryHistory(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get limit from query params (default 100)
		limit := 100
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if parsedLimit, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && parsedLimit == 1 {
				if limit > 1000 {
					limit = 1000 // Cap at 1000
				}
			}
		}

		// Get all inventory changes
		rows, err := db.Query(`
			SELECT item_type, change_amount, reason, timestamp, notes
			FROM inventory_history
			ORDER BY timestamp DESC
			LIMIT ?
		`, limit)
		if err != nil {
			http.Error(w, "Failed to retrieve inventory history", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type HistoryEntry struct {
			ItemType     string  `json:"item_type"`
			ChangeAmount float64 `json:"change_amount"`
			Reason       string  `json:"reason"`
			Timestamp    string  `json:"timestamp"`
			Notes        *string `json:"notes,omitempty"`
		}

		history := []HistoryEntry{}
		for rows.Next() {
			var entry HistoryEntry
			var notes sql.NullString
			var timestamp time.Time

			if err := rows.Scan(&entry.ItemType, &entry.ChangeAmount, &entry.Reason, &timestamp, &notes); err == nil {
				entry.Timestamp = timestamp.Format(time.RFC3339)
				if notes.Valid {
					entry.Notes = &notes.String
				}
				history = append(history, entry)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(history)
	}
}