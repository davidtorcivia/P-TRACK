package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/middleware"
	"injection-tracker/internal/models"
	"injection-tracker/internal/repository"

	"github.com/go-chi/chi/v5"
)

// NotificationResponse represents the API response for a notification
type NotificationResponse struct {
	ID            int64      `json:"id"`
	Type          string     `json:"type"`
	Title         string     `json:"title"`
	Message       string     `json:"message"`
	IsRead        bool       `json:"is_read"`
	ScheduledTime *time.Time `json:"scheduled_time,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	TimeAgo       string     `json:"time_ago"` // Human-readable time
}

// NotificationsListResponse represents the response for listing notifications
type NotificationsListResponse struct {
	Notifications []*NotificationResponse `json:"notifications"`
	UnreadCount   int64                   `json:"unread_count"`
	Total         int                     `json:"total"`
}

// HandleGetNotifications returns user notifications
func HandleGetNotifications(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse query parameters
		includeRead := r.URL.Query().Get("include_read") == "true"
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")

		limit := 50 // default
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		offset := 0
		if offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		repo := repository.NewNotificationRepository(db)

		// Get notifications
		notifications, err := repo.GetByUserID(userID, includeRead, limit, offset)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get notifications: %v", err), http.StatusInternalServerError)
			return
		}

		// Get unread count
		unreadCount, err := repo.CountUnread(userID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to count unread notifications: %v", err), http.StatusInternalServerError)
			return
		}

		// Convert to response format
		responseNotifications := make([]*NotificationResponse, 0, len(notifications))
		for _, n := range notifications {
			responseNotifications = append(responseNotifications, notificationToResponse(n))
		}

		response := NotificationsListResponse{
			Notifications: responseNotifications,
			UnreadCount:   unreadCount,
			Total:         len(responseNotifications),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode notifications response: %v", err)
		}
	}
}

// HandleMarkNotificationRead marks a notification as read
func HandleMarkNotificationRead(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid notification ID", http.StatusBadRequest)
			return
		}

		repo := repository.NewNotificationRepository(db)
		if err := repo.MarkAsRead(id, userID); err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Notification not found", http.StatusNotFound)
				return
			}
			http.Error(w, fmt.Sprintf("Failed to mark notification as read: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleMarkAllNotificationsRead marks all notifications as read
func HandleMarkAllNotificationsRead(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		repo := repository.NewNotificationRepository(db)
		if err := repo.MarkAllAsRead(userID); err != nil {
			http.Error(w, fmt.Sprintf("Failed to mark all notifications as read: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleDeleteNotification deletes a notification
func HandleDeleteNotification(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid notification ID", http.StatusBadRequest)
			return
		}

		repo := repository.NewNotificationRepository(db)
		if err := repo.Delete(id, userID); err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Notification not found", http.StatusNotFound)
				return
			}
			http.Error(w, fmt.Sprintf("Failed to delete notification: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleGetUnreadCount returns the count of unread notifications
func HandleGetUnreadCount(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		repo := repository.NewNotificationRepository(db)
		count, err := repo.CountUnread(userID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to count unread notifications: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]int64{"count": count}); err != nil {
			log.Printf("Failed to encode count response: %v", err)
		}
	}
}

// notificationToResponse converts a notification model to API response
func notificationToResponse(n *models.Notification) *NotificationResponse {
	var scheduledTime *time.Time
	if n.ScheduledTime.Valid {
		scheduledTime = &n.ScheduledTime.Time
	}

	return &NotificationResponse{
		ID:            n.ID,
		Type:          n.Type,
		Title:         n.Title,
		Message:       n.Message,
		IsRead:        n.IsRead,
		ScheduledTime: scheduledTime,
		CreatedAt:     n.CreatedAt,
		TimeAgo:       formatTimeAgo(n.CreatedAt),
	}
}
