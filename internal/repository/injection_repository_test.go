package repository

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/models"
)

func setupInjectionTestDB(t *testing.T) *database.DB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create schema
	schema := `
		CREATE TABLE courses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			start_date DATE NOT NULL,
			expected_end_date DATE,
			actual_end_date DATE,
			is_active BOOLEAN DEFAULT 1,
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_by INTEGER
		);

		CREATE TABLE injections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			course_id INTEGER NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
			administered_by INTEGER,
			timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			side TEXT NOT NULL CHECK(side IN ('left', 'right')),
			site_x REAL,
			site_y REAL,
			pain_level INTEGER CHECK(pain_level BETWEEN 1 AND 10),
			has_knots BOOLEAN DEFAULT 0,
			site_reaction TEXT CHECK(site_reaction IN ('none', 'redness', 'swelling', 'bruising', 'other')),
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX idx_injections_course ON injections(course_id);
		CREATE INDEX idx_injections_timestamp ON injections(timestamp);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}

func createTestCourse(t *testing.T, db *database.DB) int64 {
	result, err := db.Exec(
		"INSERT INTO courses (name, start_date, is_active) VALUES (?, ?, ?)",
		"Test Course",
		time.Now(),
		true,
	)
	if err != nil {
		t.Fatalf("Failed to create test course: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get course ID: %v", err)
	}

	return id
}

func TestInjectionRepository_Create(t *testing.T) {
	db := setupInjectionTestDB(t)
	defer db.Close()

	courseID := createTestCourse(t, db)
	repo := NewInjectionRepository(db)

	tests := []struct {
		name        string
		injection   *models.Injection
		expectError bool
	}{
		{
			name: "Valid injection - left side",
			injection: &models.Injection{
				CourseID:       courseID,
				AdministeredBy: sql.NullInt64{Int64: 1, Valid: true},
				Timestamp:      time.Now(),
				Side:           "left",
			},
			expectError: false,
		},
		{
			name: "Valid injection - right side",
			injection: &models.Injection{
				CourseID:  courseID,
				Timestamp: time.Now(),
				Side:      "right",
			},
			expectError: false,
		},
		{
			name: "Injection with optional fields",
			injection: &models.Injection{
				CourseID:     courseID,
				Timestamp:    time.Now(),
				Side:         "left",
				SiteX:        sql.NullFloat64{Float64: 0.5, Valid: true},
				SiteY:        sql.NullFloat64{Float64: 0.3, Valid: true},
				PainLevel:    sql.NullInt64{Int64: 5, Valid: true},
				HasKnots:     true,
				SiteReaction: sql.NullString{String: "redness", Valid: true},
				Notes:        sql.NullString{String: "Test notes", Valid: true},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(tt.injection)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.injection.ID == 0 {
				t.Error("Expected non-zero ID after creation")
			}

			// Verify injection was created
			retrieved, err := repo.GetByID(tt.injection.ID)
			if err != nil {
				t.Errorf("Failed to retrieve created injection: %v", err)
				return
			}

			if retrieved.Side != tt.injection.Side {
				t.Errorf("Expected side %s, got %s", tt.injection.Side, retrieved.Side)
			}
		})
	}
}

func TestInjectionRepository_GetByID(t *testing.T) {
	db := setupInjectionTestDB(t)
	defer db.Close()

	courseID := createTestCourse(t, db)
	repo := NewInjectionRepository(db)

	// Create test injection
	injection := &models.Injection{
		CourseID:  courseID,
		Timestamp: time.Now(),
		Side:      "left",
	}
	if err := repo.Create(injection); err != nil {
		t.Fatalf("Failed to create test injection: %v", err)
	}

	tests := []struct {
		name        string
		id          int64
		expectError bool
	}{
		{
			name:        "Valid ID",
			id:          injection.ID,
			expectError: false,
		},
		{
			name:        "Non-existent ID",
			id:          99999,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrieved, err := repo.GetByID(tt.id)

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

			if retrieved.ID != tt.id {
				t.Errorf("Expected ID %d, got %d", tt.id, retrieved.ID)
			}
		})
	}
}

func TestInjectionRepository_Update(t *testing.T) {
	db := setupInjectionTestDB(t)
	defer db.Close()

	courseID := createTestCourse(t, db)
	repo := NewInjectionRepository(db)

	// Create test injection
	injection := &models.Injection{
		CourseID:  courseID,
		Timestamp: time.Now(),
		Side:      "left",
		PainLevel: sql.NullInt64{Int64: 3, Valid: true},
	}
	if err := repo.Create(injection); err != nil {
		t.Fatalf("Failed to create test injection: %v", err)
	}

	// Update injection
	injection.Side = "right"
	injection.PainLevel = sql.NullInt64{Int64: 7, Valid: true}
	injection.HasKnots = true
	injection.Notes = sql.NullString{String: "Updated notes", Valid: true}

	if err := repo.Update(injection); err != nil {
		t.Fatalf("Failed to update injection: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetByID(injection.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve injection: %v", err)
	}

	if retrieved.Side != "right" {
		t.Errorf("Expected side right, got %s", retrieved.Side)
	}

	if retrieved.PainLevel.Int64 != 7 {
		t.Errorf("Expected pain level 7, got %d", retrieved.PainLevel.Int64)
	}

	if !retrieved.HasKnots {
		t.Error("Expected HasKnots to be true")
	}
}

func TestInjectionRepository_Delete(t *testing.T) {
	db := setupInjectionTestDB(t)
	defer db.Close()

	courseID := createTestCourse(t, db)
	repo := NewInjectionRepository(db)

	// Create test injection
	injection := &models.Injection{
		CourseID:  courseID,
		Timestamp: time.Now(),
		Side:      "left",
	}
	if err := repo.Create(injection); err != nil {
		t.Fatalf("Failed to create test injection: %v", err)
	}

	// Delete injection
	if err := repo.Delete(injection.ID); err != nil {
		t.Fatalf("Failed to delete injection: %v", err)
	}

	// Verify deletion
	_, err := repo.GetByID(injection.ID)
	if err != ErrNotFound {
		t.Error("Expected injection to be deleted")
	}
}

func TestInjectionRepository_List(t *testing.T) {
	db := setupInjectionTestDB(t)
	defer db.Close()

	courseID := createTestCourse(t, db)
	repo := NewInjectionRepository(db)

	// Create multiple injections
	for i := 0; i < 15; i++ {
		injection := &models.Injection{
			CourseID:  courseID,
			Timestamp: time.Now().Add(time.Duration(-i) * time.Hour),
			Side:      "left",
		}
		if err := repo.Create(injection); err != nil {
			t.Fatalf("Failed to create injection: %v", err)
		}
	}

	// Test pagination
	list, err := repo.List(10, 0)
	if err != nil {
		t.Fatalf("Failed to list injections: %v", err)
	}

	if len(list) != 10 {
		t.Errorf("Expected 10 injections, got %d", len(list))
	}

	// Test offset
	list2, err := repo.List(10, 10)
	if err != nil {
		t.Fatalf("Failed to list injections with offset: %v", err)
	}

	if len(list2) != 5 {
		t.Errorf("Expected 5 injections with offset, got %d", len(list2))
	}
}

func TestInjectionRepository_ListByCourse(t *testing.T) {
	db := setupInjectionTestDB(t)
	defer db.Close()

	course1ID := createTestCourse(t, db)
	course2ID := createTestCourse(t, db)
	repo := NewInjectionRepository(db)

	// Create injections for course 1
	for i := 0; i < 5; i++ {
		injection := &models.Injection{
			CourseID:  course1ID,
			Timestamp: time.Now(),
			Side:      "left",
		}
		if err := repo.Create(injection); err != nil {
			t.Fatalf("Failed to create injection: %v", err)
		}
	}

	// Create injections for course 2
	for i := 0; i < 3; i++ {
		injection := &models.Injection{
			CourseID:  course2ID,
			Timestamp: time.Now(),
			Side:      "right",
		}
		if err := repo.Create(injection); err != nil {
			t.Fatalf("Failed to create injection: %v", err)
		}
	}

	// List injections for course 1
	list, err := repo.ListByCourse(course1ID, 100, 0)
	if err != nil {
		t.Fatalf("Failed to list injections by course: %v", err)
	}

	if len(list) != 5 {
		t.Errorf("Expected 5 injections for course 1, got %d", len(list))
	}

	for _, inj := range list {
		if inj.CourseID != course1ID {
			t.Error("Returned injection from wrong course")
		}
	}
}

func TestInjectionRepository_GetLastBySide(t *testing.T) {
	db := setupInjectionTestDB(t)
	defer db.Close()

	courseID := createTestCourse(t, db)
	repo := NewInjectionRepository(db)

	// Create injections with different times
	now := time.Now()
	for i := 0; i < 3; i++ {
		injection := &models.Injection{
			CourseID:  courseID,
			Timestamp: now.Add(time.Duration(-i) * time.Hour),
			Side:      "left",
		}
		if err := repo.Create(injection); err != nil {
			t.Fatalf("Failed to create injection: %v", err)
		}
	}

	// Get last left injection
	last, err := repo.GetLastBySide("left")
	if err != nil {
		t.Fatalf("Failed to get last injection: %v", err)
	}

	// Should be the most recent (first created)
	if last.Timestamp.Before(now.Add(-1 * time.Hour)) {
		t.Error("Returned injection is not the most recent")
	}

	// Test non-existent side
	_, err = repo.GetLastBySide("right")
	if err != ErrNotFound {
		t.Error("Expected ErrNotFound for non-existent side")
	}
}

func TestInjectionRepository_CountByCourse(t *testing.T) {
	db := setupInjectionTestDB(t)
	defer db.Close()

	courseID := createTestCourse(t, db)
	repo := NewInjectionRepository(db)

	// Create injections
	for i := 0; i < 7; i++ {
		injection := &models.Injection{
			CourseID:  courseID,
			Timestamp: time.Now(),
			Side:      "left",
		}
		if err := repo.Create(injection); err != nil {
			t.Fatalf("Failed to create injection: %v", err)
		}
	}

	count, err := repo.CountByCourse(courseID)
	if err != nil {
		t.Fatalf("Failed to count injections: %v", err)
	}

	if count != 7 {
		t.Errorf("Expected count 7, got %d", count)
	}
}

func TestInjectionRepository_GetSiteHistory(t *testing.T) {
	db := setupInjectionTestDB(t)
	defer db.Close()

	courseID := createTestCourse(t, db)
	repo := NewInjectionRepository(db)

	// Create injections with site coordinates
	for i := 0; i < 5; i++ {
		injection := &models.Injection{
			CourseID:  courseID,
			Timestamp: time.Now().Add(time.Duration(-i*24) * time.Hour),
			Side:      "left",
			SiteX:     sql.NullFloat64{Float64: float64(i) * 0.1, Valid: true},
			SiteY:     sql.NullFloat64{Float64: float64(i) * 0.1, Valid: true},
		}
		if err := repo.Create(injection); err != nil {
			t.Fatalf("Failed to create injection: %v", err)
		}
	}

	// Create old injection (outside history window)
	oldInjection := &models.Injection{
		CourseID:  courseID,
		Timestamp: time.Now().Add(-20 * 24 * time.Hour),
		Side:      "left",
		SiteX:     sql.NullFloat64{Float64: 0.9, Valid: true},
		SiteY:     sql.NullFloat64{Float64: 0.9, Valid: true},
	}
	if err := repo.Create(oldInjection); err != nil {
		t.Fatalf("Failed to create old injection: %v", err)
	}

	// Get site history for last 14 days
	history, err := repo.GetSiteHistory("left", 14)
	if err != nil {
		t.Fatalf("Failed to get site history: %v", err)
	}

	// Should only return injections within 14 days
	if len(history) != 5 {
		t.Errorf("Expected 5 injections in history, got %d", len(history))
	}

	// Verify all have site coordinates
	for _, inj := range history {
		if !inj.SiteX.Valid || !inj.SiteY.Valid {
			t.Error("Site history returned injection without coordinates")
		}
	}
}

// Benchmark tests
func BenchmarkInjectionRepository_Create(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	db, _ := database.Open(dbPath)
	defer db.Close()

	db.Exec("CREATE TABLE courses (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, start_date DATE NOT NULL, expected_end_date DATE, actual_end_date DATE, is_active BOOLEAN DEFAULT 1, notes TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, created_by INTEGER);")
	db.Exec("CREATE TABLE injections (id INTEGER PRIMARY KEY AUTOINCREMENT, course_id INTEGER NOT NULL REFERENCES courses(id) ON DELETE CASCADE, administered_by INTEGER, timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, side TEXT NOT NULL CHECK(side IN ('left', 'right')), site_x REAL, site_y REAL, pain_level INTEGER CHECK(pain_level BETWEEN 1 AND 10), has_knots BOOLEAN DEFAULT 0, site_reaction TEXT, notes TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);")

	result, _ := db.Exec("INSERT INTO courses (name, start_date, is_active) VALUES (?, ?, ?)", "Test Course", time.Now(), true)
	courseID, _ := result.LastInsertId()

	repo := NewInjectionRepository(db)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		injection := &models.Injection{
			CourseID:  courseID,
			Timestamp: time.Now(),
			Side:      "left",
		}
		repo.Create(injection)
	}
}