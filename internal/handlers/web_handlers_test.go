package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/middleware"
	"injection-tracker/internal/models"
)

func TestHandleDashboard(t *testing.T) {
	// Setup test database
	db := setupTestDB(t)
	defer db.Close()

	// Create test data
	user := createTestUser(t, db)
	course := createTestCourse(t, db, user.ID)
	injection := createTestInjection(t, db, course.ID, user.ID)

	// Create CSRF protection
	csrf := middleware.NewCSRFProtection("test-secret")

	// Create handler
	handler := HandleDashboard(db, csrf)

	// Create request with authentication
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req = addTestAuthCookie(req, user.ID)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check that response contains injection data
	responseBody := rr.Body.String()
	if !contains(responseBody, injection.Side) {
		t.Errorf("Response should contain injection side: %s", injection.Side)
	}

	if !contains(responseBody, course.Name) {
		t.Errorf("Response should contain course name: %s", course.Name)
	}
}

func TestHandleGetRecentActivity(t *testing.T) {
	// Setup test database
	db := setupTestDB(t)
	defer db.Close()

	// Create test data
	user := createTestUser(t, db)
	course := createTestCourse(t, db, user.ID)
	_ = createTestInjection(t, db, course.ID, user.ID)
	_ = createTestSymptom(t, db, course.ID, user.ID)

	// Create handler
	handler := HandleGetRecentActivity(db)

	// Create request with authentication
	req := httptest.NewRequest("GET", "/api/dashboard/recent", nil)
	req = addTestAuthCookie(req, user.ID)
	req.Header.Set("HX-Request", "true")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check that response contains activity data
	responseBody := rr.Body.String()
	if !contains(responseBody, "💉 Injection") {
		t.Errorf("Response should contain injection activity")
	}

	if !contains(responseBody, "📝 Symptom Log") {
		t.Errorf("Response should contain symptom activity")
	}
}

// Helper functions for testing

func setupTestDB(t *testing.T) *database.DB {
	// Create a simple in-memory database for testing
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create the tables directly instead of running migrations
	createTestTables(t, db)

	return db
}

func createTestTables(t *testing.T, db *database.DB) {
	// Create users table
	_, err := db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			email TEXT,
			is_active BOOLEAN DEFAULT 1,
			failed_login_attempts INTEGER DEFAULT 0,
			locked_until DATETIME,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	// Create courses table
	_, err = db.Exec(`
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
			created_by INTEGER,
			FOREIGN KEY (created_by) REFERENCES users(id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create courses table: %v", err)
	}

	// Create injections table
	_, err = db.Exec(`
		CREATE TABLE injections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			course_id INTEGER NOT NULL,
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
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (course_id) REFERENCES courses(id) ON DELETE CASCADE,
			FOREIGN KEY (administered_by) REFERENCES users(id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create injections table: %v", err)
	}

	// Create symptom_logs table
	_, err = db.Exec(`
		CREATE TABLE symptom_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			course_id INTEGER NOT NULL,
			logged_by INTEGER,
			timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			pain_level INTEGER CHECK(pain_level BETWEEN 1 AND 10),
			pain_location TEXT,
			pain_type TEXT,
			symptoms TEXT,
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (course_id) REFERENCES courses(id) ON DELETE CASCADE,
			FOREIGN KEY (logged_by) REFERENCES users(id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create symptom_logs table: %v", err)
	}
}

func createTestUser(t *testing.T, db *database.DB) *models.User {
	result, err := db.Exec(`
		INSERT INTO users (username, password_hash, is_active, created_at)
		VALUES (?, ?, ?, ?)
	`, "testuser", "$2a$12$hash", true, time.Now())
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get user ID: %v", err)
	}

	return &models.User{
		ID:       id,
		Username: "testuser",
		IsActive: true,
	}
}

func createTestCourse(t *testing.T, db *database.DB, userID int64) *models.Course {
	result, err := db.Exec(`
		INSERT INTO courses (name, start_date, is_active, created_at, created_by)
		VALUES (?, ?, ?, ?, ?)
	`, "Test Course", time.Now(), true, time.Now(), userID)
	if err != nil {
		t.Fatalf("Failed to create test course: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get course ID: %v", err)
	}

	return &models.Course{
		ID:        id,
		Name:      "Test Course",
		StartDate: time.Now(),
		IsActive:  true,
		CreatedBy: sql.NullInt64{Int64: userID, Valid: true},
	}
}

func createTestInjection(t *testing.T, db *database.DB, courseID, userID int64) *models.Injection {
	result, err := db.Exec(`
		INSERT INTO injections (course_id, administered_by, timestamp, side, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, courseID, userID, time.Now().Add(-2*time.Hour), "left", time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create test injection: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get injection ID: %v", err)
	}

	return &models.Injection{
		ID:             id,
		CourseID:       courseID,
		AdministeredBy: sql.NullInt64{Int64: userID, Valid: true},
		Timestamp:      time.Now().Add(-2 * time.Hour),
		Side:           "left",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func createTestSymptom(t *testing.T, db *database.DB, courseID, userID int64) *models.SymptomLog {
	symptomsJSON := `["nausea", "fatigue"]`
	result, err := db.Exec(`
		INSERT INTO symptom_logs (course_id, logged_by, timestamp, pain_level, pain_location, pain_type, symptoms, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, courseID, userID, time.Now().Add(-1*time.Hour), 5, "abdomen", "aching", symptomsJSON, "Feeling unwell", time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create test symptom: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get symptom ID: %v", err)
	}

	return &models.SymptomLog{
		ID:           id,
		CourseID:     courseID,
		LoggedBy:     sql.NullInt64{Int64: userID, Valid: true},
		Timestamp:    time.Now().Add(-1 * time.Hour),
		PainLevel:    sql.NullInt64{Int64: 5, Valid: true},
		PainLocation: sql.NullString{String: "abdomen", Valid: true},
		PainType:     sql.NullString{String: "aching", Valid: true},
		Symptoms:     sql.NullString{String: symptomsJSON, Valid: true},
		Notes:        sql.NullString{String: "Feeling unwell", Valid: true},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func addTestAuthCookie(req *http.Request, userID int64) *http.Request {
	// Create a simple test JWT token (in real app, use proper JWT library)
	token := createTestJWT(userID)
	cookie := &http.Cookie{
		Name:  "auth_token",
		Value: token,
		Path:  "/",
	}
	req.AddCookie(cookie)
	return req
}

func createTestJWT(userID int64) string {
	// This is a mock JWT for testing - in real app use proper JWT library
	claims := map[string]interface{}{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	claimsJSON, _ := json.Marshal(claims)
	return string(claimsJSON)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

