package services

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/repository"
)

// NotificationService handles the creation and management of notifications
type NotificationService struct {
	db                  *database.DB
	notificationRepo    *repository.NotificationRepository
	inventoryRepo       *repository.InventoryRepository
	lowStockEnabled     bool
	expirationEnabled   bool
}

// NewNotificationService creates a new notification service
func NewNotificationService(db *database.DB) *NotificationService {
	return &NotificationService{
		db:                  db,
		notificationRepo:    repository.NewNotificationRepository(db),
		inventoryRepo:       repository.NewInventoryRepository(db),
		lowStockEnabled:     true,
		expirationEnabled:   true,
	}
}

// CheckAndCreateInventoryNotifications checks inventory and creates notifications for low stock and expiring items
// This should be called periodically (e.g., daily or when inventory changes)
func (s *NotificationService) CheckAndCreateInventoryNotifications(accountID int64) error {
	log.Printf("Checking inventory notifications for account %d", accountID)

	// Get all users in the account to send notifications
	userIDs, err := s.getUserIDsForAccount(accountID)
	if err != nil {
		return fmt.Errorf("failed to get users for account: %w", err)
	}

	if len(userIDs) == 0 {
		log.Printf("No users found for account %d", accountID)
		return nil
	}

	// Check low stock notifications
	if s.lowStockEnabled {
		if err := s.checkLowStockNotifications(accountID, userIDs); err != nil {
			log.Printf("Error checking low stock notifications: %v", err)
		}
	}

	// Check expiration notifications
	if s.expirationEnabled {
		if err := s.checkExpirationNotifications(accountID, userIDs); err != nil {
			log.Printf("Error checking expiration notifications: %v", err)
		}
	}

	return nil
}

// checkLowStockNotifications creates notifications for low stock items
func (s *NotificationService) checkLowStockNotifications(accountID int64, userIDs []int64) error {
	lowStockItems, err := s.inventoryRepo.ListLowStock(accountID)
	if err != nil {
		return fmt.Errorf("failed to list low stock items: %w", err)
	}

	for _, item := range lowStockItems {
		if !item.LowStockThreshold.Valid {
			continue
		}

		threshold := item.LowStockThreshold.Float64
		severity := "warning"
		if item.Quantity <= threshold/2 {
			severity = "critical"
		}

		// Create notification for each user in the account
		for _, userID := range userIDs {
			userIDSQL := sql.NullInt64{Int64: userID, Valid: true}
			err := s.notificationRepo.CreateLowStockNotification(
				userIDSQL,
				item.ItemType,
				item.Quantity,
				threshold,
				severity,
			)
			if err != nil {
				log.Printf("Failed to create low stock notification for user %d: %v", userID, err)
			}
		}
	}

	if len(lowStockItems) > 0 {
		log.Printf("Created low stock notifications for %d items", len(lowStockItems))
	}

	return nil
}

// checkExpirationNotifications creates notifications for expiring or expired items
func (s *NotificationService) checkExpirationNotifications(accountID int64, userIDs []int64) error {
	// Get all inventory items for the account
	items, err := s.inventoryRepo.List(accountID)
	if err != nil {
		return fmt.Errorf("failed to list inventory items: %w", err)
	}

	now := time.Now()
	warningDays := 30 // Warn 30 days before expiration

	for _, item := range items {
		if !item.ExpirationDate.Valid {
			continue
		}

		expirationDate := item.ExpirationDate.Time
		daysUntil := int(time.Until(expirationDate).Hours() / 24)

		// Check if expired or expiring within warning period
		isExpired := expirationDate.Before(now)
		isExpiring := !isExpired && daysUntil <= warningDays

		if isExpired || isExpiring {
			// Create notification for each user in the account
			for _, userID := range userIDs {
				userIDSQL := sql.NullInt64{Int64: userID, Valid: true}
				err := s.notificationRepo.CreateExpirationNotification(
					userIDSQL,
					item.ItemType,
					expirationDate,
					isExpired,
				)
				if err != nil {
					log.Printf("Failed to create expiration notification for user %d: %v", userID, err)
				}
			}
		}
	}

	return nil
}

// getUserIDsForAccount retrieves all user IDs for a given account
func (s *NotificationService) getUserIDsForAccount(accountID int64) ([]int64, error) {
	query := `
		SELECT user_id
		FROM account_members
		WHERE account_id = ?
	`
	rows, err := s.db.Query(query, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to query account members: %w", err)
	}
	defer rows.Close()

	var userIDs []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan user ID: %w", err)
		}
		userIDs = append(userIDs, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return userIDs, nil
}

// CheckAndCreateNotificationsForAllAccounts checks and creates notifications for all accounts
// This can be called by a background worker or cron job
func (s *NotificationService) CheckAndCreateNotificationsForAllAccounts() error {
	log.Println("Checking notifications for all accounts")

	// Get all account IDs
	query := "SELECT id FROM accounts"
	rows, err := s.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query accounts: %w", err)
	}
	defer rows.Close()

	accountCount := 0
	for rows.Next() {
		var accountID int64
		if err := rows.Scan(&accountID); err != nil {
			log.Printf("Failed to scan account ID: %v", err)
			continue
		}

		if err := s.CheckAndCreateInventoryNotifications(accountID); err != nil {
			log.Printf("Failed to check notifications for account %d: %v", accountID, err)
		}
		accountCount++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating accounts: %w", err)
	}

	log.Printf("Completed notification check for %d accounts", accountCount)
	return nil
}

// CleanupOldNotifications removes old read notifications (older than specified days)
func (s *NotificationService) CleanupOldNotifications(daysOld int) error {
	log.Printf("Cleaning up notifications older than %d days", daysOld)

	if err := s.notificationRepo.DeleteOldRead(daysOld); err != nil {
		return fmt.Errorf("failed to delete old notifications: %w", err)
	}

	log.Println("Cleanup completed successfully")
	return nil
}

// SetLowStockEnabled enables or disables low stock notifications
func (s *NotificationService) SetLowStockEnabled(enabled bool) {
	s.lowStockEnabled = enabled
}

// SetExpirationEnabled enables or disables expiration notifications
func (s *NotificationService) SetExpirationEnabled(enabled bool) {
	s.expirationEnabled = enabled
}
