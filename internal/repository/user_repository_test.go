package repository

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/models"
)

func setupTestDB(t *testing.T) *database.DB {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create users table
	schema := `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			email TEXT,
			is_active BOOLEAN DEFAULT 1,
			failed_login_attempts INTEGER DEFAULT 0,
			locked_until TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}

func TestUserRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	tests := []struct {
		name        string
		user        *models.User
		expectError bool
	}{
		{
			name: "Valid user with email",
			user: &models.User{
				Username:     "testuser",
				PasswordHash: "hashedpassword123",
				Email:        sql.NullString{String: "test@example.com", Valid: true},
				IsActive:     true,
			},
			expectError: false,
		},
		{
			name: "Valid user without email",
			user: &models.User{
				Username:     "testuser2",
				PasswordHash: "hashedpassword456",
				IsActive:     true,
			},
			expectError: false,
		},
		{
			name: "Duplicate username",
			user: &models.User{
				Username:     "testuser",
				PasswordHash: "hashedpassword789",
				IsActive:     true,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(tt.user)

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

			if tt.user.ID == 0 {
				t.Error("Expected non-zero ID after creation")
			}

			// Verify user was created
			retrieved, err := repo.GetByID(tt.user.ID)
			if err != nil {
				t.Errorf("Failed to retrieve created user: %v", err)
				return
			}

			if retrieved.Username != tt.user.Username {
				t.Errorf("Expected username %s, got %s", tt.user.Username, retrieved.Username)
			}
		})
	}
}

func TestUserRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	// Create test user
	user := &models.User{
		Username:     "testuser",
		PasswordHash: "hashedpassword",
		Email:        sql.NullString{String: "test@example.com", Valid: true},
		IsActive:     true,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	tests := []struct {
		name        string
		id          int64
		expectError bool
	}{
		{
			name:        "Valid ID",
			id:          user.ID,
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

func TestUserRepository_GetByUsername(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	// Create test user
	user := &models.User{
		Username:     "TestUser",
		PasswordHash: "hashedpassword",
		IsActive:     true,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	tests := []struct {
		name        string
		username    string
		expectError bool
	}{
		{
			name:        "Exact match",
			username:    "TestUser",
			expectError: false,
		},
		{
			name:        "Case insensitive - lowercase",
			username:    "testuser",
			expectError: false,
		},
		{
			name:        "Case insensitive - uppercase",
			username:    "TESTUSER",
			expectError: false,
		},
		{
			name:        "Non-existent user",
			username:    "nonexistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrieved, err := repo.GetByUsername(tt.username)

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

			if retrieved.Username != user.Username {
				t.Errorf("Expected username %s, got %s", user.Username, retrieved.Username)
			}
		})
	}
}

func TestUserRepository_UpdateLastLogin(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	// Create test user
	user := &models.User{
		Username:     "testuser",
		PasswordHash: "hashedpassword",
		IsActive:     true,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Update last login
	if err := repo.UpdateLastLogin(user.ID); err != nil {
		t.Fatalf("Failed to update last login: %v", err)
	}

	// Verify last login was updated
	retrieved, err := repo.GetByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}

	if !retrieved.LastLogin.Valid {
		t.Error("Expected LastLogin to be set")
	}

	if time.Since(retrieved.LastLogin.Time) > 5*time.Second {
		t.Error("LastLogin timestamp is too old")
	}
}

func TestUserRepository_FailedLoginAttempts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	// Create test user
	user := &models.User{
		Username:     "testuser",
		PasswordHash: "hashedpassword",
		IsActive:     true,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Increment failed logins
	for i := 1; i <= 3; i++ {
		if err := repo.IncrementFailedLogins(user.ID); err != nil {
			t.Fatalf("Failed to increment failed logins: %v", err)
		}

		retrieved, err := repo.GetByID(user.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve user: %v", err)
		}

		if retrieved.FailedLoginAttempts != i {
			t.Errorf("Expected %d failed attempts, got %d", i, retrieved.FailedLoginAttempts)
		}
	}

	// Reset failed logins
	if err := repo.ResetFailedLogins(user.ID); err != nil {
		t.Fatalf("Failed to reset failed logins: %v", err)
	}

	retrieved, err := repo.GetByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}

	if retrieved.FailedLoginAttempts != 0 {
		t.Errorf("Expected 0 failed attempts after reset, got %d", retrieved.FailedLoginAttempts)
	}
}

func TestUserRepository_LockAccount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	// Create test user
	user := &models.User{
		Username:     "testuser",
		PasswordHash: "hashedpassword",
		IsActive:     true,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Lock account
	lockUntil := time.Now().Add(15 * time.Minute)
	if err := repo.LockAccount(user.ID, lockUntil); err != nil {
		t.Fatalf("Failed to lock account: %v", err)
	}

	// Check if account is locked
	isLocked, err := repo.IsAccountLocked(user.ID)
	if err != nil {
		t.Fatalf("Failed to check account lock: %v", err)
	}

	if !isLocked {
		t.Error("Expected account to be locked")
	}

	// Verify locked_until time
	retrieved, err := repo.GetByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}

	if !retrieved.LockedUntil.Valid {
		t.Error("Expected LockedUntil to be set")
	}

	if retrieved.LockedUntil.Time.Sub(lockUntil).Abs() > 1*time.Second {
		t.Error("LockedUntil time doesn't match expected value")
	}
}

func TestUserRepository_IsAccountLocked_Expired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	// Create test user
	user := &models.User{
		Username:     "testuser",
		PasswordHash: "hashedpassword",
		IsActive:     true,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Lock account with past time
	lockUntil := time.Now().Add(-1 * time.Hour)
	if err := repo.LockAccount(user.ID, lockUntil); err != nil {
		t.Fatalf("Failed to lock account: %v", err)
	}

	// Check if account is locked
	isLocked, err := repo.IsAccountLocked(user.ID)
	if err != nil {
		t.Fatalf("Failed to check account lock: %v", err)
	}

	if isLocked {
		t.Error("Expected account to NOT be locked (lock expired)")
	}
}

func TestUserRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	// Create test user
	user := &models.User{
		Username:     "testuser",
		PasswordHash: "hashedpassword",
		Email:        sql.NullString{String: "old@example.com", Valid: true},
		IsActive:     true,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Update user
	user.Username = "updateduser"
	user.Email = sql.NullString{String: "new@example.com", Valid: true}
	user.IsActive = false

	if err := repo.Update(user); err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}

	if retrieved.Username != "updateduser" {
		t.Errorf("Expected username updateduser, got %s", retrieved.Username)
	}

	if retrieved.Email.String != "new@example.com" {
		t.Errorf("Expected email new@example.com, got %s", retrieved.Email.String)
	}

	if retrieved.IsActive {
		t.Error("Expected IsActive to be false")
	}
}

func TestUserRepository_UpdatePassword(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	// Create test user
	user := &models.User{
		Username:     "testuser",
		PasswordHash: "oldpassword",
		IsActive:     true,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Update password
	newHash := "newpasswordhash"
	if err := repo.UpdatePassword(user.ID, newHash); err != nil {
		t.Fatalf("Failed to update password: %v", err)
	}

	// Verify password was updated
	retrieved, err := repo.GetByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}

	if retrieved.PasswordHash != newHash {
		t.Errorf("Expected password hash %s, got %s", newHash, retrieved.PasswordHash)
	}
}

func TestUserRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	// Create test user
	user := &models.User{
		Username:     "testuser",
		PasswordHash: "hashedpassword",
		IsActive:     true,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Delete user (soft delete)
	if err := repo.Delete(user.ID); err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	// Verify user is marked inactive
	retrieved, err := repo.GetByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}

	if retrieved.IsActive {
		t.Error("Expected user to be inactive after deletion")
	}
}

func TestUserRepository_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	// Create multiple users
	users := []*models.User{
		{Username: "user1", PasswordHash: "hash1", IsActive: true},
		{Username: "user2", PasswordHash: "hash2", IsActive: true},
		{Username: "user3", PasswordHash: "hash3", IsActive: false}, // Inactive
	}

	for _, user := range users {
		if err := repo.Create(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
	}

	// List users (should only return active users)
	list, err := repo.List()
	if err != nil {
		t.Fatalf("Failed to list users: %v", err)
	}

	// Should only return 2 active users
	if len(list) != 2 {
		t.Errorf("Expected 2 users, got %d", len(list))
	}

	// Verify all returned users are active
	for _, user := range list {
		if !user.IsActive {
			t.Error("List returned inactive user")
		}
	}
}

// Test concurrent operations
func TestUserRepository_ConcurrentOperations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	// Create base user
	user := &models.User{
		Username:     "testuser",
		PasswordHash: "hashedpassword",
		IsActive:     true,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Test concurrent reads
	const goroutines = 50
	done := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			_, err := repo.GetByID(user.ID)
			if err != nil {
				t.Errorf("Concurrent read failed: %v", err)
			}
			done <- true
		}()
	}

	for i := 0; i < goroutines; i++ {
		<-done
	}
}

// Benchmark tests
func BenchmarkUserRepository_Create(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	db, _ := database.Open(dbPath)
	defer db.Close()

	schema := `CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT UNIQUE NOT NULL, password_hash TEXT NOT NULL, email TEXT, is_active BOOLEAN DEFAULT 1, failed_login_attempts INTEGER DEFAULT 0, locked_until TIMESTAMP, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, last_login TIMESTAMP);`
	_, _ = db.Exec(schema)

	repo := NewUserRepository(db)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		user := &models.User{
			Username:     "user" + string(rune(i)),
			PasswordHash: "hash",
			IsActive:     true,
		}
		_ = repo.Create(user)
	}
}

func BenchmarkUserRepository_GetByID(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	db, _ := database.Open(dbPath)
	defer db.Close()

	schema := `CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT UNIQUE NOT NULL, password_hash TEXT NOT NULL, email TEXT, is_active BOOLEAN DEFAULT 1, failed_login_attempts INTEGER DEFAULT 0, locked_until TIMESTAMP, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, last_login TIMESTAMP);`
	_, _ = db.Exec(schema)

	repo := NewUserRepository(db)
	user := &models.User{Username: "testuser", PasswordHash: "hash", IsActive: true}
	_ = repo.Create(user)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repo.GetByID(user.ID)
	}
}
