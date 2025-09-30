package repository

import (
	"database/sql"
	"fmt"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/models"
)

type SymptomRepository struct {
	db *database.DB
}

func NewSymptomRepository(db *database.DB) *SymptomRepository {
	return &SymptomRepository{db: db}
}

// Create creates a new symptom log entry
func (r *SymptomRepository) Create(symptom *models.SymptomLog) error {
	query := `
		INSERT INTO symptom_logs (course_id, logged_by, timestamp, pain_level, pain_location, pain_type, symptoms, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	result, err := r.db.Exec(query,
		symptom.CourseID,
		symptom.LoggedBy,
		symptom.Timestamp,
		symptom.PainLevel,
		symptom.PainLocation,
		symptom.PainType,
		symptom.Symptoms,
		symptom.Notes,
	)
	if err != nil {
		return fmt.Errorf("failed to create symptom log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	symptom.ID = id
	return nil
}

// GetByID retrieves a symptom log by ID
func (r *SymptomRepository) GetByID(id int64) (*models.SymptomLog, error) {
	query := `
		SELECT id, course_id, logged_by, timestamp, pain_level, pain_location, pain_type, symptoms, notes, created_at, updated_at
		FROM symptom_logs
		WHERE id = ?
	`
	var symptom models.SymptomLog
	err := r.db.QueryRow(query, id).Scan(
		&symptom.ID,
		&symptom.CourseID,
		&symptom.LoggedBy,
		&symptom.Timestamp,
		&symptom.PainLevel,
		&symptom.PainLocation,
		&symptom.PainType,
		&symptom.Symptoms,
		&symptom.Notes,
		&symptom.CreatedAt,
		&symptom.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get symptom log: %w", err)
	}

	return &symptom, nil
}

// Update updates a symptom log entry
func (r *SymptomRepository) Update(symptom *models.SymptomLog) error {
	query := `
		UPDATE symptom_logs
		SET course_id = ?, logged_by = ?, timestamp = ?, pain_level = ?, pain_location = ?, pain_type = ?, symptoms = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	_, err := r.db.Exec(query,
		symptom.CourseID,
		symptom.LoggedBy,
		symptom.Timestamp,
		symptom.PainLevel,
		symptom.PainLocation,
		symptom.PainType,
		symptom.Symptoms,
		symptom.Notes,
		symptom.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update symptom log: %w", err)
	}
	return nil
}

// Delete deletes a symptom log
func (r *SymptomRepository) Delete(id int64) error {
	query := `DELETE FROM symptom_logs WHERE id = ?`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete symptom log: %w", err)
	}
	return nil
}

// List retrieves all symptom logs with pagination
func (r *SymptomRepository) List(limit, offset int) ([]*models.SymptomLog, error) {
	query := `
		SELECT id, course_id, logged_by, timestamp, pain_level, pain_location, pain_type, symptoms, notes, created_at, updated_at
		FROM symptom_logs
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list symptom logs: %w", err)
	}
	defer rows.Close()

	return r.scanSymptomLogs(rows)
}

// ListByCourse retrieves all symptom logs for a specific course
func (r *SymptomRepository) ListByCourse(courseID int64, limit, offset int) ([]*models.SymptomLog, error) {
	query := `
		SELECT id, course_id, logged_by, timestamp, pain_level, pain_location, pain_type, symptoms, notes, created_at, updated_at
		FROM symptom_logs
		WHERE course_id = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, courseID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list symptom logs by course: %w", err)
	}
	defer rows.Close()

	return r.scanSymptomLogs(rows)
}

// ListByDateRange retrieves symptom logs within a date range
func (r *SymptomRepository) ListByDateRange(startDate, endDate time.Time, limit, offset int) ([]*models.SymptomLog, error) {
	query := `
		SELECT id, course_id, logged_by, timestamp, pain_level, pain_location, pain_type, symptoms, notes, created_at, updated_at
		FROM symptom_logs
		WHERE timestamp BETWEEN ? AND ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, startDate, endDate, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list symptom logs by date range: %w", err)
	}
	defer rows.Close()

	return r.scanSymptomLogs(rows)
}

// GetRecent retrieves the most recent symptom logs
func (r *SymptomRepository) GetRecent(count int) ([]*models.SymptomLog, error) {
	query := `
		SELECT id, course_id, logged_by, timestamp, pain_level, pain_location, pain_type, symptoms, notes, created_at, updated_at
		FROM symptom_logs
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := r.db.Query(query, count)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent symptom logs: %w", err)
	}
	defer rows.Close()

	return r.scanSymptomLogs(rows)
}

// CountByCourse counts symptom logs for a specific course
func (r *SymptomRepository) CountByCourse(courseID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM symptom_logs WHERE course_id = ?`
	var count int64
	err := r.db.QueryRow(query, courseID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count symptom logs by course: %w", err)
	}
	return count, nil
}

// CountByDateRange counts symptom logs within a date range
func (r *SymptomRepository) CountByDateRange(startDate, endDate time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM symptom_logs WHERE timestamp BETWEEN ? AND ?`
	var count int64
	err := r.db.QueryRow(query, startDate, endDate).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count symptom logs by date range: %w", err)
	}
	return count, nil
}

// GetAveragePainLevel calculates the average pain level for a course
func (r *SymptomRepository) GetAveragePainLevel(courseID int64) (float64, error) {
	query := `SELECT AVG(pain_level) FROM symptom_logs WHERE course_id = ? AND pain_level IS NOT NULL`
	var avg sql.NullFloat64
	err := r.db.QueryRow(query, courseID).Scan(&avg)
	if err != nil {
		return 0, fmt.Errorf("failed to get average pain level: %w", err)
	}
	if !avg.Valid {
		return 0, nil
	}
	return avg.Float64, nil
}

// scanSymptomLogs is a helper to scan multiple symptom log rows
func (r *SymptomRepository) scanSymptomLogs(rows *sql.Rows) ([]*models.SymptomLog, error) {
	var symptoms []*models.SymptomLog
	for rows.Next() {
		var symptom models.SymptomLog
		err := rows.Scan(
			&symptom.ID,
			&symptom.CourseID,
			&symptom.LoggedBy,
			&symptom.Timestamp,
			&symptom.PainLevel,
			&symptom.PainLocation,
			&symptom.PainType,
			&symptom.Symptoms,
			&symptom.Notes,
			&symptom.CreatedAt,
			&symptom.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan symptom log: %w", err)
		}
		symptoms = append(symptoms, &symptom)
	}

	return symptoms, rows.Err()
}