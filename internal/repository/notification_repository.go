package repository

import (
	"database/sql"
	"fmt"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/models"
)

type NotificationRepository struct {
	db *database.DB
}

func NewNotificationRepository(db *database.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Create creates a new notification
func (r *NotificationRepository) Create(notification *models.Notification) error {
	query := `
		INSERT INTO notifications (user_id, type, title, message, is_read, scheduled_time, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.Exec(query,
		notification.UserID,
		notification.Type,
		notification.Title,
		notification.Message,
		notification.IsRead,
		notification.ScheduledTime,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	notification.ID = id
	notification.CreatedAt = time.Now()
	return nil
}

// GetByID retrieves a notification by ID
func (r *NotificationRepository) GetByID(id int64) (*models.Notification, error) {
	query := `
		SELECT id, user_id, type, title, message, is_read, scheduled_time, created_at
		FROM notifications
		WHERE id = ?
	`
	var n models.Notification
	err := r.db.QueryRow(query, id).Scan(
		&n.ID,
		&n.UserID,
		&n.Type,
		&n.Title,
		&n.Message,
		&n.IsRead,
		&n.ScheduledTime,
		&n.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	return &n, nil
}

// GetByUserID retrieves all notifications for a user
func (r *NotificationRepository) GetByUserID(userID int64, includeRead bool, limit, offset int) ([]*models.Notification, error) {
	query := `
		SELECT id, user_id, type, title, message, is_read, scheduled_time, created_at
		FROM notifications
		WHERE user_id = ? OR user_id IS NULL
	`
	args := []interface{}{userID}

	if !includeRead {
		query += " AND is_read = 0"
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer rows.Close()

	return r.scanNotifications(rows)
}

// CountUnread counts unread notifications for a user
func (r *NotificationRepository) CountUnread(userID int64) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM notifications
		WHERE (user_id = ? OR user_id IS NULL) AND is_read = 0
	`
	var count int64
	err := r.db.QueryRow(query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count unread notifications: %w", err)
	}
	return count, nil
}

// MarkAsRead marks a notification as read
func (r *NotificationRepository) MarkAsRead(id int64, userID int64) error {
	query := `
		UPDATE notifications
		SET is_read = 1
		WHERE id = ? AND (user_id = ? OR user_id IS NULL)
	`
	result, err := r.db.Exec(query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// MarkAllAsRead marks all notifications as read for a user
func (r *NotificationRepository) MarkAllAsRead(userID int64) error {
	query := `
		UPDATE notifications
		SET is_read = 1
		WHERE (user_id = ? OR user_id IS NULL) AND is_read = 0
	`
	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}

	return nil
}

// Delete deletes a notification
func (r *NotificationRepository) Delete(id int64, userID int64) error {
	query := `
		DELETE FROM notifications
		WHERE id = ? AND (user_id = ? OR user_id IS NULL)
	`
	result, err := r.db.Exec(query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteOldRead deletes old read notifications (older than specified days)
func (r *NotificationRepository) DeleteOldRead(daysOld int) error {
	query := `
		DELETE FROM notifications
		WHERE is_read = 1 AND created_at < datetime('now', ?)
	`
	_, err := r.db.Exec(query, fmt.Sprintf("-%d days", daysOld))
	if err != nil {
		return fmt.Errorf("failed to delete old notifications: %w", err)
	}

	return nil
}

// CreateLowStockNotification creates a low stock notification
func (r *NotificationRepository) CreateLowStockNotification(userID sql.NullInt64, itemType string, quantity float64, threshold float64, severity string) error {
	// Check if a similar notification already exists (within last 24 hours)
	exists, err := r.notificationExists(userID, "low_stock", itemType, 24)
	if err != nil {
		return err
	}
	if exists {
		return nil // Don't create duplicate notification
	}

	title := "Low Stock Alert"
	if severity == "critical" {
		title = "Critical: Stock Very Low"
	}

	message := fmt.Sprintf("%s is running low (%.1f remaining, threshold: %.1f). Please restock soon.",
		formatItemType(itemType), quantity, threshold)

	notification := &models.Notification{
		UserID:  userID,
		Type:    "low_stock",
		Title:   title,
		Message: message,
		IsRead:  false,
	}

	return r.Create(notification)
}

// CreateExpirationNotification creates an expiration warning notification
func (r *NotificationRepository) CreateExpirationNotification(userID sql.NullInt64, itemType string, expirationDate time.Time, isExpired bool) error {
	// Check if a similar notification already exists (within last 24 hours)
	exists, err := r.notificationExists(userID, "expiration_warning", itemType, 24)
	if err != nil {
		return err
	}
	if exists {
		return nil // Don't create duplicate notification
	}

	var title, message string
	if isExpired {
		title = "Expired Medication"
		message = fmt.Sprintf("%s expired on %s. Please dispose of it and restock.",
			formatItemType(itemType), expirationDate.Format("Jan 2, 2006"))
	} else {
		daysUntil := int(time.Until(expirationDate).Hours() / 24)
		title = "Medication Expiring Soon"
		message = fmt.Sprintf("%s will expire in %d days (on %s).",
			formatItemType(itemType), daysUntil, expirationDate.Format("Jan 2, 2006"))
	}

	notification := &models.Notification{
		UserID:  userID,
		Type:    "expiration_warning",
		Title:   title,
		Message: message,
		IsRead:  false,
	}

	return r.Create(notification)
}

// notificationExists checks if a similar notification already exists recently
func (r *NotificationRepository) notificationExists(userID sql.NullInt64, notifType, keyword string, hoursAgo int) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM notifications
			WHERE type = ?
			AND message LIKE ?
			AND created_at > datetime('now', ?)
			AND (user_id = ? OR (user_id IS NULL AND ? IS NULL))
		)
	`
	var exists bool
	err := r.db.QueryRow(query,
		notifType,
		"%"+keyword+"%",
		fmt.Sprintf("-%d hours", hoursAgo),
		userID,
		userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check notification existence: %w", err)
	}

	return exists, nil
}

// scanNotifications is a helper to scan multiple notification rows
func (r *NotificationRepository) scanNotifications(rows *sql.Rows) ([]*models.Notification, error) {
	var notifications []*models.Notification
	for rows.Next() {
		var n models.Notification
		err := rows.Scan(
			&n.ID,
			&n.UserID,
			&n.Type,
			&n.Title,
			&n.Message,
			&n.IsRead,
			&n.ScheduledTime,
			&n.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}
		notifications = append(notifications, &n)
	}

	return notifications, rows.Err()
}

// formatItemType converts item_type to human-readable format
func formatItemType(itemType string) string {
	switch itemType {
	case "progesterone":
		return "Progesterone"
	case "draw_needle":
		return "Draw Needles"
	case "injection_needle":
		return "Injection Needles"
	case "syringe":
		return "Syringes"
	case "swab":
		return "Alcohol Swabs"
	case "gauze":
		return "Gauze Pads"
	default:
		return itemType
	}
}
