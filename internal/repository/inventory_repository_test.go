package repository

import (
	"database/sql"
	"path/filepath"
	"testing"

	"injection-tracker/internal/database"
	"injection-tracker/internal/models"
)

func setupInventoryTestDB(t *testing.T) *database.DB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create schema
	schema := `
		CREATE TABLE inventory_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			item_type TEXT UNIQUE NOT NULL CHECK(item_type IN ('progesterone', 'draw_needle', 'injection_needle', 'syringe', 'swab', 'gauze')),
			quantity REAL NOT NULL,
			unit TEXT NOT NULL,
			expiration_date TIMESTAMP,
			lot_number TEXT,
			low_stock_threshold REAL,
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE inventory_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			item_type TEXT NOT NULL,
			change_amount REAL NOT NULL,
			quantity_before REAL NOT NULL,
			quantity_after REAL NOT NULL,
			reason TEXT NOT NULL,
			reference_id INTEGER,
			reference_type TEXT,
			performed_by INTEGER,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			notes TEXT
		);

		CREATE INDEX idx_inventory_history_type ON inventory_history(item_type);
		CREATE INDEX idx_inventory_history_timestamp ON inventory_history(timestamp);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}

func createTestInventoryItems(t *testing.T, db *database.DB) {
	items := []struct {
		itemType  string
		quantity  float64
		unit      string
		threshold float64
	}{
		{"progesterone", 10.0, "mL", 2.0},
		{"draw_needle", 20.0, "count", 5.0},
		{"injection_needle", 20.0, "count", 5.0},
		{"syringe", 20.0, "count", 5.0},
		{"swab", 50.0, "count", 10.0},
	}

	for _, item := range items {
		_, err := db.Exec(
			"INSERT INTO inventory_items (item_type, quantity, unit, low_stock_threshold) VALUES (?, ?, ?, ?)",
			item.itemType, item.quantity, item.unit, item.threshold,
		)
		if err != nil {
			t.Fatalf("Failed to create inventory item %s: %v", item.itemType, err)
		}
	}
}

func TestInventoryRepository_GetByType(t *testing.T) {
	db := setupInventoryTestDB(t)
	defer db.Close()

	createTestInventoryItems(t, db)
	repo := NewInventoryRepository(db)

	tests := []struct {
		name        string
		itemType    string
		expectError bool
	}{
		{
			name:        "Valid item type - progesterone",
			itemType:    "progesterone",
			expectError: false,
		},
		{
			name:        "Valid item type - draw_needle",
			itemType:    "draw_needle",
			expectError: false,
		},
		{
			name:        "Non-existent item type",
			itemType:    "nonexistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := repo.GetByType(tt.itemType)

			if tt.expectError {
				if err != ErrNotFound {
					t.Errorf("Expected ErrNotFound, got %v", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if item.ItemType != tt.itemType {
				t.Errorf("Expected item type %s, got %s", tt.itemType, item.ItemType)
			}
		})
	}
}

func TestInventoryRepository_Upsert(t *testing.T) {
	db := setupInventoryTestDB(t)
	defer db.Close()

	repo := NewInventoryRepository(db)

	// Test insert
	item := &models.InventoryItem{
		ItemType:          "progesterone",
		Quantity:          10.0,
		Unit:              "mL",
		LowStockThreshold: sql.NullFloat64{Float64: 2.0, Valid: true},
	}

	if err := repo.Upsert(item); err != nil {
		t.Fatalf("Failed to insert item: %v", err)
	}

	// Verify insert
	retrieved, err := repo.GetByType("progesterone")
	if err != nil {
		t.Fatalf("Failed to retrieve item: %v", err)
	}

	if retrieved.Quantity != 10.0 {
		t.Errorf("Expected quantity 10.0, got %f", retrieved.Quantity)
	}

	// Test update (upsert existing item)
	item.Quantity = 15.0
	if err := repo.Upsert(item); err != nil {
		t.Fatalf("Failed to update item: %v", err)
	}

	// Verify update
	retrieved, err = repo.GetByType("progesterone")
	if err != nil {
		t.Fatalf("Failed to retrieve updated item: %v", err)
	}

	if retrieved.Quantity != 15.0 {
		t.Errorf("Expected quantity 15.0, got %f", retrieved.Quantity)
	}
}

func TestInventoryRepository_UpdateQuantity(t *testing.T) {
	db := setupInventoryTestDB(t)
	defer db.Close()

	createTestInventoryItems(t, db)
	repo := NewInventoryRepository(db)

	// Update quantity
	if err := repo.UpdateQuantity("progesterone", 25.0); err != nil {
		t.Fatalf("Failed to update quantity: %v", err)
	}

	// Verify update
	item, err := repo.GetByType("progesterone")
	if err != nil {
		t.Fatalf("Failed to retrieve item: %v", err)
	}

	if item.Quantity != 25.0 {
		t.Errorf("Expected quantity 25.0, got %f", item.Quantity)
	}
}

// CRITICAL TEST: Test inventory adjustment with transaction and history logging
func TestInventoryRepository_AdjustQuantity(t *testing.T) {
	db := setupInventoryTestDB(t)
	defer db.Close()

	createTestInventoryItems(t, db)
	repo := NewInventoryRepository(db)

	tests := []struct {
		name           string
		itemType       string
		delta          float64
		reason         string
		expectError    bool
		expectedFinal  float64
	}{
		{
			name:          "Positive adjustment (restock)",
			itemType:      "progesterone",
			delta:         5.0,
			reason:        "restock",
			expectError:   false,
			expectedFinal: 15.0,
		},
		{
			name:          "Negative adjustment (usage)",
			itemType:      "draw_needle",
			delta:         -3.0,
			reason:        "manual_usage",
			expectError:   false,
			expectedFinal: 17.0,
		},
		{
			name:          "Cannot go negative",
			itemType:      "swab",
			delta:         -100.0,
			reason:        "test",
			expectError:   true,
			expectedFinal: 50.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get initial quantity
			initialItem, _ := repo.GetByType(tt.itemType)
			initialQty := initialItem.Quantity

			// Adjust quantity
			err := repo.AdjustQuantity(
				tt.itemType,
				tt.delta,
				tt.reason,
				sql.NullInt64{},
				sql.NullString{},
				sql.NullInt64{Int64: 1, Valid: true},
				sql.NullString{},
			)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}

				// Verify quantity unchanged on error
				item, _ := repo.GetByType(tt.itemType)
				if item.Quantity != tt.expectedFinal {
					t.Errorf("Expected quantity %f after error, got %f", tt.expectedFinal, item.Quantity)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify new quantity
			item, err := repo.GetByType(tt.itemType)
			if err != nil {
				t.Errorf("Failed to retrieve item: %v", err)
				return
			}

			if item.Quantity != tt.expectedFinal {
				t.Errorf("Expected quantity %f, got %f", tt.expectedFinal, item.Quantity)
			}

			// Verify history was logged
			history, err := repo.GetHistory(tt.itemType, 10, 0)
			if err != nil {
				t.Errorf("Failed to get history: %v", err)
				return
			}

			if len(history) == 0 {
				t.Error("Expected history entry but got none")
				return
			}

			lastEntry := history[0]
			if lastEntry.ChangeAmount != tt.delta {
				t.Errorf("Expected change amount %f, got %f", tt.delta, lastEntry.ChangeAmount)
			}

			if lastEntry.QuantityBefore != initialQty {
				t.Errorf("Expected quantity before %f, got %f", initialQty, lastEntry.QuantityBefore)
			}

			if lastEntry.QuantityAfter != tt.expectedFinal {
				t.Errorf("Expected quantity after %f, got %f", tt.expectedFinal, lastEntry.QuantityAfter)
			}
		})
	}
}

// CRITICAL TEST: Test decrement for injection with full transaction atomicity
func TestInventoryRepository_DecrementForInjection(t *testing.T) {
	db := setupInventoryTestDB(t)
	defer db.Close()

	createTestInventoryItems(t, db)
	repo := NewInventoryRepository(db)

	injectionID := int64(123)
	userID := int64(1)
	progesteroneML := 1.0

	// Record initial quantities
	initialQuantities := make(map[string]float64)
	itemTypes := []string{"progesterone", "draw_needle", "injection_needle", "syringe", "swab"}
	for _, itemType := range itemTypes {
		item, _ := repo.GetByType(itemType)
		initialQuantities[itemType] = item.Quantity
	}

	// Decrement for injection
	err := repo.DecrementForInjection(injectionID, userID, progesteroneML)
	if err != nil {
		t.Fatalf("Failed to decrement for injection: %v", err)
	}

	// Verify all items were decremented correctly
	expectedDecrements := map[string]float64{
		"progesterone":     progesteroneML,
		"draw_needle":      1.0,
		"injection_needle": 1.0,
		"syringe":          1.0,
		"swab":             1.0,
	}

	for itemType, expectedDecrement := range expectedDecrements {
		item, err := repo.GetByType(itemType)
		if err != nil {
			t.Errorf("Failed to retrieve %s: %v", itemType, err)
			continue
		}

		expectedQuantity := initialQuantities[itemType] - expectedDecrement
		if item.Quantity != expectedQuantity {
			t.Errorf("%s: Expected quantity %f, got %f", itemType, expectedQuantity, item.Quantity)
		}

		// Verify history was logged
		history, err := repo.GetHistory(itemType, 1, 0)
		if err != nil {
			t.Errorf("Failed to get history for %s: %v", itemType, err)
			continue
		}

		if len(history) == 0 {
			t.Errorf("No history entry for %s", itemType)
			continue
		}

		entry := history[0]
		if entry.Reason != "injection" {
			t.Errorf("Expected reason 'injection', got %s", entry.Reason)
		}

		if entry.ReferenceID.Int64 != injectionID {
			t.Errorf("Expected reference ID %d, got %d", injectionID, entry.ReferenceID.Int64)
		}

		if entry.ChangeAmount != -expectedDecrement {
			t.Errorf("Expected change amount %f, got %f", -expectedDecrement, entry.ChangeAmount)
		}
	}
}

// CRITICAL TEST: Test transaction rollback on insufficient inventory
func TestInventoryRepository_DecrementForInjection_InsufficientInventory(t *testing.T) {
	db := setupInventoryTestDB(t)
	defer db.Close()

	repo := NewInventoryRepository(db)

	// Create items with low quantities
	_, err := db.Exec(
		"INSERT INTO inventory_items (item_type, quantity, unit) VALUES (?, ?, ?)",
		"progesterone", 0.5, "mL",
	)
	if err != nil {
		t.Fatalf("Failed to create inventory item: %v", err)
	}

	_, err = db.Exec(
		"INSERT INTO inventory_items (item_type, quantity, unit) VALUES (?, ?, ?)",
		"draw_needle", 10.0, "count",
	)
	if err != nil {
		t.Fatalf("Failed to create inventory item: %v", err)
	}

	// Record initial quantity
	initialItem, _ := repo.GetByType("draw_needle")
	initialDrawNeedleQty := initialItem.Quantity

	// Attempt to decrement (should fail due to insufficient progesterone)
	err = repo.DecrementForInjection(123, 1, 1.0)
	if err == nil {
		t.Fatal("Expected error for insufficient inventory but got none")
	}

	// Verify NO items were decremented (transaction rollback)
	item, err := repo.GetByType("draw_needle")
	if err != nil {
		t.Fatalf("Failed to retrieve draw_needle: %v", err)
	}

	if item.Quantity != initialDrawNeedleQty {
		t.Errorf("Expected draw_needle quantity unchanged at %f, got %f (transaction did not roll back)",
			initialDrawNeedleQty, item.Quantity)
	}

	// Verify no history was logged
	history, _ := repo.GetHistory("draw_needle", 10, 0)
	if len(history) > 0 {
		t.Error("Expected no history entries after failed transaction")
	}
}

// CRITICAL TEST: Test transaction rollback on missing item
func TestInventoryRepository_DecrementForInjection_MissingItem(t *testing.T) {
	db := setupInventoryTestDB(t)
	defer db.Close()

	repo := NewInventoryRepository(db)

	// Create only some items (missing swab)
	items := []string{"progesterone", "draw_needle", "injection_needle", "syringe"}
	for _, itemType := range items {
		_, err := db.Exec(
			"INSERT INTO inventory_items (item_type, quantity, unit) VALUES (?, ?, ?)",
			itemType, 10.0, "count",
		)
		if err != nil {
			t.Fatalf("Failed to create inventory item: %v", err)
		}
	}

	// Record initial quantities
	initialProgesterone, _ := repo.GetByType("progesterone")
	initialQty := initialProgesterone.Quantity

	// Attempt to decrement (should fail due to missing swab)
	err := repo.DecrementForInjection(123, 1, 1.0)
	if err == nil {
		t.Fatal("Expected error for missing item but got none")
	}

	// Verify progesterone was NOT decremented (transaction rollback)
	item, err := repo.GetByType("progesterone")
	if err != nil {
		t.Fatalf("Failed to retrieve progesterone: %v", err)
	}

	if item.Quantity != initialQty {
		t.Errorf("Expected progesterone quantity unchanged at %f, got %f (transaction did not roll back)",
			initialQty, item.Quantity)
	}
}

func TestInventoryRepository_List(t *testing.T) {
	db := setupInventoryTestDB(t)
	defer db.Close()

	createTestInventoryItems(t, db)
	repo := NewInventoryRepository(db)

	list, err := repo.List()
	if err != nil {
		t.Fatalf("Failed to list inventory: %v", err)
	}

	if len(list) != 5 {
		t.Errorf("Expected 5 items, got %d", len(list))
	}

	// Verify items are sorted by type
	for i := 1; i < len(list); i++ {
		if list[i-1].ItemType > list[i].ItemType {
			t.Error("Items not sorted by type")
		}
	}
}

func TestInventoryRepository_ListLowStock(t *testing.T) {
	db := setupInventoryTestDB(t)
	defer db.Close()

	repo := NewInventoryRepository(db)

	// Create items with different stock levels
	items := []struct {
		itemType  string
		quantity  float64
		threshold float64
	}{
		{"progesterone", 1.0, 2.0},        // Below threshold
		{"draw_needle", 3.0, 5.0},         // Below threshold
		{"injection_needle", 10.0, 5.0},   // Above threshold
		{"syringe", 20.0, 5.0},            // Above threshold
	}

	for _, item := range items {
		_, err := db.Exec(
			"INSERT INTO inventory_items (item_type, quantity, unit, low_stock_threshold) VALUES (?, ?, ?, ?)",
			item.itemType, item.quantity, "count", item.threshold,
		)
		if err != nil {
			t.Fatalf("Failed to create inventory item: %v", err)
		}
	}

	lowStock, err := repo.ListLowStock()
	if err != nil {
		t.Fatalf("Failed to list low stock items: %v", err)
	}

	// Should only return 2 items below threshold
	if len(lowStock) != 2 {
		t.Errorf("Expected 2 low stock items, got %d", len(lowStock))
	}

	// Verify returned items are actually low stock
	for _, item := range lowStock {
		if item.Quantity > item.LowStockThreshold.Float64 {
			t.Errorf("Item %s not actually low stock: quantity %f > threshold %f",
				item.ItemType, item.Quantity, item.LowStockThreshold.Float64)
		}
	}
}

func TestInventoryRepository_GetHistory(t *testing.T) {
	db := setupInventoryTestDB(t)
	defer db.Close()

	createTestInventoryItems(t, db)
	repo := NewInventoryRepository(db)

	// Make several adjustments
	for i := 0; i < 5; i++ {
		repo.AdjustQuantity(
			"progesterone",
			-0.5,
			"test_usage",
			sql.NullInt64{},
			sql.NullString{},
			sql.NullInt64{Int64: 1, Valid: true},
			sql.NullString{},
		)
	}

	// Get history
	history, err := repo.GetHistory("progesterone", 10, 0)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}

	if len(history) != 5 {
		t.Errorf("Expected 5 history entries, got %d", len(history))
	}

	// Verify entries are ordered by timestamp DESC (most recent first)
	for i := 1; i < len(history); i++ {
		if history[i-1].Timestamp.Before(history[i].Timestamp) {
			t.Error("History not ordered by timestamp DESC")
		}
	}
}

// Test concurrent inventory operations
func TestInventoryRepository_ConcurrentAdjustments(t *testing.T) {
	db := setupInventoryTestDB(t)
	defer db.Close()

	createTestInventoryItems(t, db)
	repo := NewInventoryRepository(db)

	const goroutines = 10
	done := make(chan error, goroutines)

	// Concurrent decrements
	for i := 0; i < goroutines; i++ {
		go func() {
			err := repo.AdjustQuantity(
				"progesterone",
				-0.1,
				"concurrent_test",
				sql.NullInt64{},
				sql.NullString{},
				sql.NullInt64{Int64: 1, Valid: true},
				sql.NullString{},
			)
			done <- err
		}()
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent adjustment failed: %v", err)
		}
	}

	// Verify final quantity
	item, err := repo.GetByType("progesterone")
	if err != nil {
		t.Fatalf("Failed to retrieve item: %v", err)
	}

	expectedQuantity := 10.0 - (float64(goroutines) * 0.1)
	if item.Quantity != expectedQuantity {
		t.Errorf("Expected quantity %f, got %f", expectedQuantity, item.Quantity)
	}

	// Verify history count
	history, err := repo.GetHistory("progesterone", 100, 0)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}

	if len(history) != goroutines {
		t.Errorf("Expected %d history entries, got %d", goroutines, len(history))
	}
}

// Benchmark tests
func BenchmarkInventoryRepository_DecrementForInjection(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	db, _ := database.Open(dbPath)
	defer db.Close()

	db.Exec("CREATE TABLE inventory_items (id INTEGER PRIMARY KEY AUTOINCREMENT, item_type TEXT UNIQUE NOT NULL CHECK(item_type IN ('progesterone', 'draw_needle', 'injection_needle', 'syringe', 'swab', 'gauze')), quantity REAL NOT NULL, unit TEXT NOT NULL, expiration_date TIMESTAMP, lot_number TEXT, low_stock_threshold REAL, notes TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);")
	db.Exec("CREATE TABLE inventory_history (id INTEGER PRIMARY KEY AUTOINCREMENT, item_type TEXT NOT NULL, change_amount REAL NOT NULL, quantity_before REAL NOT NULL, quantity_after REAL NOT NULL, reason TEXT NOT NULL, reference_id INTEGER, reference_type TEXT, performed_by INTEGER, timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP, notes TEXT);")

	// Create items with large quantities for benchmarking
	items := []string{"progesterone", "draw_needle", "injection_needle", "syringe", "swab"}
	for _, itemType := range items {
		db.Exec("INSERT INTO inventory_items (item_type, quantity, unit) VALUES (?, ?, ?)", itemType, 10000.0, "count")
	}

	repo := NewInventoryRepository(db)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		repo.DecrementForInjection(int64(i), 1, 1.0)
	}
}