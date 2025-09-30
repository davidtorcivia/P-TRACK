package repository

import (
	"database/sql"
	"fmt"

	"injection-tracker/internal/database"
	"injection-tracker/internal/models"
)

type InventoryRepository struct {
	db *database.DB
}

func NewInventoryRepository(db *database.DB) *InventoryRepository {
	return &InventoryRepository{db: db}
}

// GetByType retrieves an inventory item by type
func (r *InventoryRepository) GetByType(itemType string) (*models.InventoryItem, error) {
	query := `
		SELECT id, item_type, quantity, unit, expiration_date, lot_number, low_stock_threshold, notes, created_at, updated_at
		FROM inventory_items
		WHERE item_type = ?
	`
	var item models.InventoryItem
	err := r.db.QueryRow(query, itemType).Scan(
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
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory item: %w", err)
	}

	return &item, nil
}

// Upsert creates or updates an inventory item
func (r *InventoryRepository) Upsert(item *models.InventoryItem) error {
	query := `
		INSERT INTO inventory_items (item_type, quantity, unit, expiration_date, lot_number, low_stock_threshold, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(item_type) DO UPDATE SET
			quantity = excluded.quantity,
			unit = excluded.unit,
			expiration_date = excluded.expiration_date,
			lot_number = excluded.lot_number,
			low_stock_threshold = excluded.low_stock_threshold,
			notes = excluded.notes,
			updated_at = CURRENT_TIMESTAMP
	`
	result, err := r.db.Exec(query,
		item.ItemType,
		item.Quantity,
		item.Unit,
		item.ExpirationDate,
		item.LotNumber,
		item.LowStockThreshold,
		item.Notes,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert inventory item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	item.ID = id
	return nil
}

// UpdateQuantity updates the quantity of an inventory item
func (r *InventoryRepository) UpdateQuantity(itemType string, newQuantity float64) error {
	query := `
		UPDATE inventory_items
		SET quantity = ?, updated_at = CURRENT_TIMESTAMP
		WHERE item_type = ?
	`
	_, err := r.db.Exec(query, newQuantity, itemType)
	if err != nil {
		return fmt.Errorf("failed to update inventory quantity: %w", err)
	}
	return nil
}

// AdjustQuantity adjusts the quantity of an inventory item by a delta amount and logs the change
func (r *InventoryRepository) AdjustQuantity(itemType string, delta float64, reason string, referenceID sql.NullInt64, referenceType sql.NullString, userID sql.NullInt64, notes sql.NullString) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get current quantity
	var currentQuantity float64
	query := `SELECT quantity FROM inventory_items WHERE item_type = ?`
	err = tx.QueryRow(query, itemType).Scan(&currentQuantity)
	if err == sql.ErrNoRows {
		return fmt.Errorf("inventory item not found: %s", itemType)
	}
	if err != nil {
		return fmt.Errorf("failed to get current quantity: %w", err)
	}

	// Calculate new quantity
	newQuantity := currentQuantity + delta

	// Prevent negative quantities
	if newQuantity < 0 {
		return fmt.Errorf("insufficient inventory: cannot reduce %s below zero (current: %.2f, requested: %.2f)", itemType, currentQuantity, delta)
	}

	// Update quantity
	query = `UPDATE inventory_items SET quantity = ?, updated_at = CURRENT_TIMESTAMP WHERE item_type = ?`
	_, err = tx.Exec(query, newQuantity, itemType)
	if err != nil {
		return fmt.Errorf("failed to update quantity: %w", err)
	}

	// Log the change
	query = `
		INSERT INTO inventory_history (item_type, change_amount, quantity_before, quantity_after, reason, reference_id, reference_type, performed_by, timestamp, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
	`
	_, err = tx.Exec(query, itemType, delta, currentQuantity, newQuantity, reason, referenceID, referenceType, userID, notes)
	if err != nil {
		return fmt.Errorf("failed to log inventory change: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DecrementForInjection decrements inventory items for an injection and logs the changes
// This is a critical method that ensures atomicity across multiple inventory items
func (r *InventoryRepository) DecrementForInjection(injectionID int64, userID int64, progesteroneML float64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Define items to decrement
	decrements := map[string]float64{
		"progesterone":     progesteroneML, // Usually 1.0 mL
		"draw_needle":      1.0,
		"injection_needle": 1.0,
		"syringe":          1.0,
		"swab":             1.0,
	}

	// Validate all items have sufficient quantity before any changes
	for itemType, amount := range decrements {
		var currentQuantity float64
		query := `SELECT quantity FROM inventory_items WHERE item_type = ?`
		err = tx.QueryRow(query, itemType).Scan(&currentQuantity)
		if err == sql.ErrNoRows {
			return fmt.Errorf("inventory item not found: %s", itemType)
		}
		if err != nil {
			return fmt.Errorf("failed to get current quantity for %s: %w", itemType, err)
		}

		if currentQuantity < amount {
			return fmt.Errorf("insufficient inventory: %s (current: %.2f, required: %.2f)", itemType, currentQuantity, amount)
		}
	}

	// Decrement each item and log the change
	for itemType, amount := range decrements {
		// Get current quantity
		var currentQuantity float64
		query := `SELECT quantity FROM inventory_items WHERE item_type = ?`
		err = tx.QueryRow(query, itemType).Scan(&currentQuantity)
		if err != nil {
			return fmt.Errorf("failed to get current quantity for %s: %w", itemType, err)
		}

		newQuantity := currentQuantity - amount

		// Update quantity
		query = `UPDATE inventory_items SET quantity = ?, updated_at = CURRENT_TIMESTAMP WHERE item_type = ?`
		_, err = tx.Exec(query, newQuantity, itemType)
		if err != nil {
			return fmt.Errorf("failed to update quantity for %s: %w", itemType, err)
		}

		// Log the change
		query = `
			INSERT INTO inventory_history (item_type, change_amount, quantity_before, quantity_after, reason, reference_id, reference_type, performed_by, timestamp, notes)
			VALUES (?, ?, ?, ?, 'injection', ?, 'injection', ?, CURRENT_TIMESTAMP, NULL)
		`
		_, err = tx.Exec(query, itemType, -amount, currentQuantity, newQuantity, injectionID, userID)
		if err != nil {
			return fmt.Errorf("failed to log inventory change for %s: %w", itemType, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// List retrieves all inventory items
func (r *InventoryRepository) List() ([]*models.InventoryItem, error) {
	query := `
		SELECT id, item_type, quantity, unit, expiration_date, lot_number, low_stock_threshold, notes, created_at, updated_at
		FROM inventory_items
		ORDER BY item_type
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list inventory items: %w", err)
	}
	defer rows.Close()

	return r.scanInventoryItems(rows)
}

// ListLowStock retrieves inventory items below their threshold
func (r *InventoryRepository) ListLowStock() ([]*models.InventoryItem, error) {
	query := `
		SELECT id, item_type, quantity, unit, expiration_date, lot_number, low_stock_threshold, notes, created_at, updated_at
		FROM inventory_items
		WHERE low_stock_threshold IS NOT NULL AND quantity <= low_stock_threshold
		ORDER BY quantity ASC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list low stock items: %w", err)
	}
	defer rows.Close()

	return r.scanInventoryItems(rows)
}

// GetHistory retrieves inventory history for an item type
func (r *InventoryRepository) GetHistory(itemType string, limit, offset int) ([]*models.InventoryHistory, error) {
	query := `
		SELECT id, item_type, change_amount, quantity_before, quantity_after, reason, reference_id, reference_type, performed_by, timestamp, notes
		FROM inventory_history
		WHERE item_type = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, itemType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory history: %w", err)
	}
	defer rows.Close()

	return r.scanInventoryHistory(rows)
}

// GetAllHistory retrieves all inventory history with pagination
func (r *InventoryRepository) GetAllHistory(limit, offset int) ([]*models.InventoryHistory, error) {
	query := `
		SELECT id, item_type, change_amount, quantity_before, quantity_after, reason, reference_id, reference_type, performed_by, timestamp, notes
		FROM inventory_history
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get all inventory history: %w", err)
	}
	defer rows.Close()

	return r.scanInventoryHistory(rows)
}

// CountHistory counts inventory history records for an item type
func (r *InventoryRepository) CountHistory(itemType string) (int64, error) {
	query := `SELECT COUNT(*) FROM inventory_history WHERE item_type = ?`
	var count int64
	err := r.db.QueryRow(query, itemType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count inventory history: %w", err)
	}
	return count, nil
}

// Delete deletes an inventory item
func (r *InventoryRepository) Delete(itemType string) error {
	query := `DELETE FROM inventory_items WHERE item_type = ?`
	_, err := r.db.Exec(query, itemType)
	if err != nil {
		return fmt.Errorf("failed to delete inventory item: %w", err)
	}
	return nil
}

// scanInventoryItems is a helper to scan multiple inventory item rows
func (r *InventoryRepository) scanInventoryItems(rows *sql.Rows) ([]*models.InventoryItem, error) {
	var items []*models.InventoryItem
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
			return nil, fmt.Errorf("failed to scan inventory item: %w", err)
		}
		items = append(items, &item)
	}

	return items, rows.Err()
}

// scanInventoryHistory is a helper to scan multiple inventory history rows
func (r *InventoryRepository) scanInventoryHistory(rows *sql.Rows) ([]*models.InventoryHistory, error) {
	var history []*models.InventoryHistory
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
			return nil, fmt.Errorf("failed to scan inventory history: %w", err)
		}
		history = append(history, &h)
	}

	return history, rows.Err()
}