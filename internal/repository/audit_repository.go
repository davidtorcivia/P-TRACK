package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/models"
)

type AuditRepository struct {
	db *database.DB
}

func NewAuditRepository(db *database.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Log creates a new audit log entry
func (r *AuditRepository) Log(entry *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (user_id, action, entity_type, entity_id, details, ip_address, user_agent, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	result, err := r.db.Exec(
		query,
		entry.UserID,
		entry.Action,
		entry.EntityType,
		entry.EntityID,
		entry.Details,
		entry.IPAddress,
		entry.UserAgent,
	)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	entry.ID = id
	return nil
}

// LogWithDetails logs an action with structured details
func (r *AuditRepository) LogWithDetails(userID sql.NullInt64, action, entityType string, entityID sql.NullInt64, details map[string]interface{}, ipAddress, userAgent string) error {
	var detailsJSON sql.NullString
	if details != nil {
		jsonBytes, err := json.Marshal(details)
		if err != nil {
			return fmt.Errorf("failed to marshal details: %w", err)
		}
		detailsJSON = sql.NullString{String: string(jsonBytes), Valid: true}
	}

	entry := &models.AuditLog{
		UserID:     userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Details:    detailsJSON,
		IPAddress:  sql.NullString{String: ipAddress, Valid: ipAddress != ""},
		UserAgent:  sql.NullString{String: userAgent, Valid: userAgent != ""},
	}

	return r.Log(entry)
}

// GetByUser retrieves audit logs for a specific user
func (r *AuditRepository) GetByUser(userID int64, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, user_id, action, entity_type, entity_id, details, ip_address, user_agent, timestamp
		FROM audit_logs
		WHERE user_id = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit logs by user: %w", err)
	}
	defer rows.Close()

	return r.scanAuditLogs(rows)
}

// GetByAction retrieves audit logs for a specific action
func (r *AuditRepository) GetByAction(action string, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, user_id, action, entity_type, entity_id, details, ip_address, user_agent, timestamp
		FROM audit_logs
		WHERE action = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, action, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit logs by action: %w", err)
	}
	defer rows.Close()

	return r.scanAuditLogs(rows)
}

// GetByEntity retrieves audit logs for a specific entity
func (r *AuditRepository) GetByEntity(entityType string, entityID int64, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, user_id, action, entity_type, entity_id, details, ip_address, user_agent, timestamp
		FROM audit_logs
		WHERE entity_type = ? AND entity_id = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, entityType, entityID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit logs by entity: %w", err)
	}
	defer rows.Close()

	return r.scanAuditLogs(rows)
}

// GetByDateRange retrieves audit logs within a date range
func (r *AuditRepository) GetByDateRange(startDate, endDate time.Time, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, user_id, action, entity_type, entity_id, details, ip_address, user_agent, timestamp
		FROM audit_logs
		WHERE timestamp BETWEEN ? AND ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, startDate, endDate, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit logs by date range: %w", err)
	}
	defer rows.Close()

	return r.scanAuditLogs(rows)
}

// GetRecentFailedLogins retrieves recent failed login attempts
func (r *AuditRepository) GetRecentFailedLogins(minutes int, limit int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, user_id, action, entity_type, entity_id, details, ip_address, user_agent, timestamp
		FROM audit_logs
		WHERE action = 'login_failed'
		  AND timestamp >= datetime('now', '-' || ? || ' minutes')
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := r.db.Query(query, minutes, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent failed logins: %w", err)
	}
	defer rows.Close()

	return r.scanAuditLogs(rows)
}

// CountFailedLoginsByIP counts failed login attempts by IP address within a time window
func (r *AuditRepository) CountFailedLoginsByIP(ipAddress string, minutes int) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE action = 'login_failed'
		  AND ip_address = ?
		  AND timestamp >= datetime('now', '-' || ? || ' minutes')
	`
	var count int
	err := r.db.QueryRow(query, ipAddress, minutes).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count failed logins by IP: %w", err)
	}
	return count, nil
}

// scanAuditLogs scans rows into audit log structs
func (r *AuditRepository) scanAuditLogs(rows *sql.Rows) ([]*models.AuditLog, error) {
	var logs []*models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.Action,
			&log.EntityType,
			&log.EntityID,
			&log.Details,
			&log.IPAddress,
			&log.UserAgent,
			&log.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, &log)
	}

	return logs, rows.Err()
}

// DeleteOldLogs deletes audit logs older than specified days (for maintenance)
func (r *AuditRepository) DeleteOldLogs(days int) (int64, error) {
	query := `
		DELETE FROM audit_logs
		WHERE timestamp < datetime('now', '-' || ? || ' days')
	`
	result, err := r.db.Exec(query, days)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old audit logs: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}