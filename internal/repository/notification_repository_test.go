package repository

import (
	"database/sql"
	"testing"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/models"
)

func setupTestDBForNotifications(t *testing.T) *database.DB {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.RunMigrations(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create test user and account
	_, err = db.Exec(`
		INSERT INTO accounts (id, name) VALUES (1, 'Test Account')
	`)
	if err != nil {
		t.Fatalf("Failed to create test account: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (id, username, password_hash, email)
		VALUES (1, 'testuser', 'hash', 'test@example.com')
	`)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO account_members (account_id, user_id, role)
		VALUES (1, 1, 'owner')
	`)
	if err != nil {
		t.Fatalf("Failed to create account member: %v", err)
	}

	return db
}

func TestNotificationRepository_Create(t *testing.T) {
	db := setupTestDBForNotifications(t)
	defer db.Close()

	repo := NewNotificationRepository(db)

	notification := &models.Notification{
		UserID:  sql.NullInt64{Int64: 1, Valid: true},
		Type:    "low_stock",
		Title:   "Test Notification",
		Message: "This is a test notification",
		IsRead:  false,
	}

	err := repo.Create(notification)
	if err != nil {
		t.Fatalf("Failed to create notification: %v", err)
	}

	if notification.ID == 0 {
		t.Fatal("Expected notification ID to be set after creation")
	}
}

func TestNotificationRepository_GetByID(t *testing.T) {
	db := setupTestDBForNotifications(t)
	defer db.Close()

	repo := NewNotificationRepository(db)

	// Create a notification
	notification := &models.Notification{
		UserID:  sql.NullInt64{Int64: 1, Valid: true},
		Type:    "low_stock",
		Title:   "Test Notification",
		Message: "This is a test notification",
		IsRead:  false,
	}

	err := repo.Create(notification)
	if err != nil {
		t.Fatalf("Failed to create notification: %v", err)
	}

	// Retrieve it
	retrieved, err := repo.GetByID(notification.ID)
	if err != nil {
		t.Fatalf("Failed to get notification: %v", err)
	}

	if retrieved.Title != notification.Title {
		t.Errorf("Expected title %s, got %s", notification.Title, retrieved.Title)
	}
	if retrieved.Message != notification.Message {
		t.Errorf("Expected message %s, got %s", notification.Message, retrieved.Message)
	}
	if retrieved.IsRead != notification.IsRead {
		t.Errorf("Expected is_read %v, got %v", notification.IsRead, retrieved.IsRead)
	}
}

func TestNotificationRepository_GetByUserID(t *testing.T) {
	db := setupTestDBForNotifications(t)
	defer db.Close()

	repo := NewNotificationRepository(db)

	// Create multiple notifications
	for i := 0; i < 3; i++ {
		notification := &models.Notification{
			UserID:  sql.NullInt64{Int64: 1, Valid: true},
			Type:    "low_stock",
			Title:   "Test Notification",
			Message: "This is a test notification",
			IsRead:  i%2 == 0, // Alternate between read and unread
		}
		err := repo.Create(notification)
		if err != nil {
			t.Fatalf("Failed to create notification: %v", err)
		}
	}

	// Get all notifications
	notifications, err := repo.GetByUserID(1, true, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get notifications: %v", err)
	}

	if len(notifications) != 3 {
		t.Errorf("Expected 3 notifications, got %d", len(notifications))
	}

	// Get only unread notifications
	unreadNotifications, err := repo.GetByUserID(1, false, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get unread notifications: %v", err)
	}

	if len(unreadNotifications) != 1 {
		t.Errorf("Expected 1 unread notification, got %d", len(unreadNotifications))
	}
}

func TestNotificationRepository_CountUnread(t *testing.T) {
	db := setupTestDBForNotifications(t)
	defer db.Close()

	repo := NewNotificationRepository(db)

	// Create notifications
	for i := 0; i < 5; i++ {
		notification := &models.Notification{
			UserID:  sql.NullInt64{Int64: 1, Valid: true},
			Type:    "low_stock",
			Title:   "Test Notification",
			Message: "This is a test notification",
			IsRead:  i < 2, // First 2 are read, last 3 are unread
		}
		err := repo.Create(notification)
		if err != nil {
			t.Fatalf("Failed to create notification: %v", err)
		}
	}

	count, err := repo.CountUnread(1)
	if err != nil {
		t.Fatalf("Failed to count unread notifications: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 unread notifications, got %d", count)
	}
}

func TestNotificationRepository_MarkAsRead(t *testing.T) {
	db := setupTestDBForNotifications(t)
	defer db.Close()

	repo := NewNotificationRepository(db)

	notification := &models.Notification{
		UserID:  sql.NullInt64{Int64: 1, Valid: true},
		Type:    "low_stock",
		Title:   "Test Notification",
		Message: "This is a test notification",
		IsRead:  false,
	}

	err := repo.Create(notification)
	if err != nil {
		t.Fatalf("Failed to create notification: %v", err)
	}

	err = repo.MarkAsRead(notification.ID, 1)
	if err != nil {
		t.Fatalf("Failed to mark notification as read: %v", err)
	}

	// Verify it's marked as read
	retrieved, err := repo.GetByID(notification.ID)
	if err != nil {
		t.Fatalf("Failed to get notification: %v", err)
	}

	if !retrieved.IsRead {
		t.Error("Expected notification to be marked as read")
	}
}

func TestNotificationRepository_MarkAllAsRead(t *testing.T) {
	db := setupTestDBForNotifications(t)
	defer db.Close()

	repo := NewNotificationRepository(db)

	// Create multiple unread notifications
	for i := 0; i < 3; i++ {
		notification := &models.Notification{
			UserID:  sql.NullInt64{Int64: 1, Valid: true},
			Type:    "low_stock",
			Title:   "Test Notification",
			Message: "This is a test notification",
			IsRead:  false,
		}
		err := repo.Create(notification)
		if err != nil {
			t.Fatalf("Failed to create notification: %v", err)
		}
	}

	err := repo.MarkAllAsRead(1)
	if err != nil {
		t.Fatalf("Failed to mark all notifications as read: %v", err)
	}

	// Verify all are marked as read
	count, err := repo.CountUnread(1)
	if err != nil {
		t.Fatalf("Failed to count unread notifications: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 unread notifications, got %d", count)
	}
}

func TestNotificationRepository_Delete(t *testing.T) {
	db := setupTestDBForNotifications(t)
	defer db.Close()

	repo := NewNotificationRepository(db)

	notification := &models.Notification{
		UserID:  sql.NullInt64{Int64: 1, Valid: true},
		Type:    "low_stock",
		Title:   "Test Notification",
		Message: "This is a test notification",
		IsRead:  false,
	}

	err := repo.Create(notification)
	if err != nil {
		t.Fatalf("Failed to create notification: %v", err)
	}

	err = repo.Delete(notification.ID, 1)
	if err != nil {
		t.Fatalf("Failed to delete notification: %v", err)
	}

	// Verify it's deleted
	_, err = repo.GetByID(notification.ID)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestNotificationRepository_CreateLowStockNotification(t *testing.T) {
	db := setupTestDBForNotifications(t)
	defer db.Close()

	repo := NewNotificationRepository(db)

	err := repo.CreateLowStockNotification(
		sql.NullInt64{Int64: 1, Valid: true},
		"progesterone",
		2.5,
		5.0,
		"warning",
	)
	if err != nil {
		t.Fatalf("Failed to create low stock notification: %v", err)
	}

	// Verify notification was created
	notifications, err := repo.GetByUserID(1, true, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get notifications: %v", err)
	}

	if len(notifications) != 1 {
		t.Errorf("Expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].Type != "low_stock" {
		t.Errorf("Expected type 'low_stock', got %s", notifications[0].Type)
	}
}

func TestNotificationRepository_CreateExpirationNotification(t *testing.T) {
	db := setupTestDBForNotifications(t)
	defer db.Close()

	repo := NewNotificationRepository(db)

	expirationDate := time.Now().AddDate(0, 0, 7) // 7 days from now

	err := repo.CreateExpirationNotification(
		sql.NullInt64{Int64: 1, Valid: true},
		"progesterone",
		expirationDate,
		false,
	)
	if err != nil {
		t.Fatalf("Failed to create expiration notification: %v", err)
	}

	// Verify notification was created
	notifications, err := repo.GetByUserID(1, true, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get notifications: %v", err)
	}

	if len(notifications) != 1 {
		t.Errorf("Expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].Type != "expiration_warning" {
		t.Errorf("Expected type 'expiration_warning', got %s", notifications[0].Type)
	}
}

func TestNotificationRepository_NoDuplicateNotifications(t *testing.T) {
	db := setupTestDBForNotifications(t)
	defer db.Close()

	repo := NewNotificationRepository(db)

	// Create first low stock notification
	err := repo.CreateLowStockNotification(
		sql.NullInt64{Int64: 1, Valid: true},
		"progesterone",
		2.5,
		5.0,
		"warning",
	)
	if err != nil {
		t.Fatalf("Failed to create first low stock notification: %v", err)
	}

	// Try to create duplicate notification (should not create)
	err = repo.CreateLowStockNotification(
		sql.NullInt64{Int64: 1, Valid: true},
		"progesterone",
		2.5,
		5.0,
		"warning",
	)
	if err != nil {
		t.Fatalf("Failed to check duplicate notification: %v", err)
	}

	// Verify only one notification exists
	notifications, err := repo.GetByUserID(1, true, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get notifications: %v", err)
	}

	if len(notifications) != 1 {
		t.Errorf("Expected 1 notification (no duplicate), got %d", len(notifications))
	}
}
