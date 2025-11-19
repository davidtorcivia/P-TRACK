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

// Create creates a new symptom log entry (course_id must belong to account - verified by caller)
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

// GetByID retrieves a symptom log by ID and account (ensures data isolation via course)
func (r *SymptomRepository) GetByID(id int64, accountID int64) (*models.SymptomLog, error) {
	query := `
		SELECT s.id, s.course_id, s.logged_by, s.timestamp, s.pain_level, s.pain_location, s.pain_type, s.symptoms, s.notes, s.created_at, s.updated_at
		FROM symptom_logs s
		JOIN courses c ON c.id = s.course_id
		WHERE s.id = ? AND c.account_id = ?
	`
	var symptom models.SymptomLog
	err := r.db.QueryRow(query, id, accountID).Scan(
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

// Update updates a symptom log entry (only if it belongs to the account via course)
func (r *SymptomRepository) Update(symptom *models.SymptomLog, accountID int64) error {
	query := `
		UPDATE symptom_logs
		SET course_id = ?, logged_by = ?, timestamp = ?, pain_level = ?, pain_location = ?, pain_type = ?, symptoms = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		AND EXISTS (SELECT 1 FROM courses WHERE id = ? AND account_id = ?)
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
		symptom.ID,
		symptom.CourseID,
		accountID,
	)
	if err != nil {
		return fmt.Errorf("failed to update symptom log: %w", err)
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

// Delete deletes a symptom log (only if it belongs to the account via course)
func (r *SymptomRepository) Delete(id int64, accountID int64) error {
	query := `
		DELETE FROM symptom_logs
		WHERE id = ?
		AND EXISTS (SELECT 1 FROM courses WHERE id = symptom_logs.course_id AND account_id = ?)
	`
	result, err := r.db.Exec(query, id, accountID)
	if err != nil {
		return fmt.Errorf("failed to delete symptom log: %w", err)
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

// List retrieves all symptom logs for an account with pagination
func (r *SymptomRepository) List(accountID int64, limit, offset int) ([]*models.SymptomLog, error) {
	query := `
		SELECT s.id, s.course_id, s.logged_by, s.timestamp, s.pain_level, s.pain_location, s.pain_type, s.symptoms, s.notes, s.created_at, s.updated_at
		FROM symptom_logs s
		JOIN courses c ON c.id = s.course_id
		WHERE c.account_id = ?
		ORDER BY s.timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list symptom logs: %w", err)
	}
	defer rows.Close()

	return r.scanSymptomLogs(rows)
}

// ListByCourse retrieves all symptom logs for a specific course (course must belong to account)
func (r *SymptomRepository) ListByCourse(courseID int64, accountID int64, limit, offset int) ([]*models.SymptomLog, error) {
	query := `
		SELECT s.id, s.course_id, s.logged_by, s.timestamp, s.pain_level, s.pain_location, s.pain_type, s.symptoms, s.notes, s.created_at, s.updated_at
		FROM symptom_logs s
		JOIN courses c ON c.id = s.course_id
		WHERE s.course_id = ? AND c.account_id = ?
		ORDER BY s.timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, courseID, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list symptom logs by course: %w", err)
	}
	defer rows.Close()

	return r.scanSymptomLogs(rows)
}

// ListByDateRange retrieves symptom logs within a date range for an account
func (r *SymptomRepository) ListByDateRange(accountID int64, startDate, endDate time.Time, limit, offset int) ([]*models.SymptomLog, error) {
	query := `
		SELECT s.id, s.course_id, s.logged_by, s.timestamp, s.pain_level, s.pain_location, s.pain_type, s.symptoms, s.notes, s.created_at, s.updated_at
		FROM symptom_logs s
		JOIN courses c ON c.id = s.course_id
		WHERE c.account_id = ? AND s.timestamp BETWEEN ? AND ?
		ORDER BY s.timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, accountID, startDate, endDate, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list symptom logs by date range: %w", err)
	}
	defer rows.Close()

	return r.scanSymptomLogs(rows)
}

// GetRecent retrieves the most recent symptom logs for an account
func (r *SymptomRepository) GetRecent(accountID int64, count int) ([]*models.SymptomLog, error) {
	query := `
		SELECT s.id, s.course_id, s.logged_by, s.timestamp, s.pain_level, s.pain_location, s.pain_type, s.symptoms, s.notes, s.created_at, s.updated_at
		FROM symptom_logs s
		JOIN courses c ON c.id = s.course_id
		WHERE c.account_id = ?
		ORDER BY s.timestamp DESC
		LIMIT ?
	`
	rows, err := r.db.Query(query, accountID, count)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent symptom logs: %w", err)
	}
	defer rows.Close()

	return r.scanSymptomLogs(rows)
}

// CountByCourse counts symptom logs for a specific course (course must belong to account)
func (r *SymptomRepository) CountByCourse(courseID int64, accountID int64) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM symptom_logs s
		JOIN courses c ON c.id = s.course_id
		WHERE s.course_id = ? AND c.account_id = ?
	`
	var count int64
	err := r.db.QueryRow(query, courseID, accountID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count symptom logs by course: %w", err)
	}
	return count, nil
}

// CountByDateRange counts symptom logs within a date range for an account
func (r *SymptomRepository) CountByDateRange(accountID int64, startDate, endDate time.Time) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM symptom_logs s
		JOIN courses c ON c.id = s.course_id
		WHERE c.account_id = ? AND s.timestamp BETWEEN ? AND ?
	`
	var count int64
	err := r.db.QueryRow(query, accountID, startDate, endDate).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count symptom logs by date range: %w", err)
	}
	return count, nil
}

// GetAveragePainLevel calculates the average pain level for a course (course must belong to account)
func (r *SymptomRepository) GetAveragePainLevel(courseID int64, accountID int64) (float64, error) {
	query := `
		SELECT AVG(s.pain_level)
		FROM symptom_logs s
		JOIN courses c ON c.id = s.course_id
		WHERE s.course_id = ? AND c.account_id = ? AND s.pain_level IS NOT NULL
	`
	var avg sql.NullFloat64
	err := r.db.QueryRow(query, courseID, accountID).Scan(&avg)
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
