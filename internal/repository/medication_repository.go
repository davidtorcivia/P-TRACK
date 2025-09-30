package repository

import (
	"database/sql"
	"fmt"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/models"
)

type MedicationRepository struct {
	db *database.DB
}

func NewMedicationRepository(db *database.DB) *MedicationRepository {
	return &MedicationRepository{db: db}
}

// Create creates a new medication
func (r *MedicationRepository) Create(medication *models.Medication) error {
	query := `
		INSERT INTO medications (name, dosage, frequency, start_date, end_date, is_active, notes, scheduled_time, time_window_minutes, reminder_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	result, err := r.db.Exec(query,
		medication.Name,
		medication.Dosage,
		medication.Frequency,
		medication.StartDate,
		medication.EndDate,
		medication.IsActive,
		medication.Notes,
		medication.ScheduledTime,
		medication.TimeWindowMinutes,
		medication.ReminderEnabled,
	)
	if err != nil {
		return fmt.Errorf("failed to create medication: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	medication.ID = id
	return nil
}

// GetByID retrieves a medication by ID
func (r *MedicationRepository) GetByID(id int64) (*models.Medication, error) {
	query := `
		SELECT id, name, dosage, frequency, start_date, end_date, is_active, notes, scheduled_time, time_window_minutes, reminder_enabled, created_at, updated_at
		FROM medications
		WHERE id = ?
	`
	var medication models.Medication
	err := r.db.QueryRow(query, id).Scan(
		&medication.ID,
		&medication.Name,
		&medication.Dosage,
		&medication.Frequency,
		&medication.StartDate,
		&medication.EndDate,
		&medication.IsActive,
		&medication.Notes,
		&medication.ScheduledTime,
		&medication.TimeWindowMinutes,
		&medication.ReminderEnabled,
		&medication.CreatedAt,
		&medication.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get medication: %w", err)
	}

	return &medication, nil
}

// Update updates a medication
func (r *MedicationRepository) Update(medication *models.Medication) error {
	query := `
		UPDATE medications
		SET name = ?, dosage = ?, frequency = ?, start_date = ?, end_date = ?, is_active = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	_, err := r.db.Exec(query,
		medication.Name,
		medication.Dosage,
		medication.Frequency,
		medication.StartDate,
		medication.EndDate,
		medication.IsActive,
		medication.Notes,
		medication.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update medication: %w", err)
	}
	return nil
}

// Delete deletes a medication (soft delete by setting is_active to false)
func (r *MedicationRepository) Delete(id int64) error {
	query := `UPDATE medications SET is_active = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete medication: %w", err)
	}
	return nil
}

// HardDelete permanently deletes a medication and all its logs
func (r *MedicationRepository) HardDelete(id int64) error {
	query := `DELETE FROM medications WHERE id = ?`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to hard delete medication: %w", err)
	}
	return nil
}

// List retrieves all medications
func (r *MedicationRepository) List() ([]*models.Medication, error) {
	query := `
		SELECT id, name, dosage, frequency, start_date, end_date, is_active, notes, scheduled_time, time_window_minutes, reminder_enabled, created_at, updated_at
		FROM medications
		ORDER BY name
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list medications: %w", err)
	}
	defer rows.Close()

	return r.scanMedications(rows)
}

// ListActive retrieves all active medications
func (r *MedicationRepository) ListActive() ([]*models.Medication, error) {
	query := `
		SELECT id, name, dosage, frequency, start_date, end_date, is_active, notes, scheduled_time, time_window_minutes, reminder_enabled, created_at, updated_at
		FROM medications
		WHERE is_active = 1
		ORDER BY name
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list active medications: %w", err)
	}
	defer rows.Close()

	return r.scanMedications(rows)
}

// CreateLog creates a new medication log entry
func (r *MedicationRepository) CreateLog(log *models.MedicationLog) error {
	query := `
		INSERT INTO medication_logs (medication_id, logged_by, timestamp, taken, notes, created_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	result, err := r.db.Exec(query,
		log.MedicationID,
		log.LoggedBy,
		log.Timestamp,
		log.Taken,
		log.Notes,
	)
	if err != nil {
		return fmt.Errorf("failed to create medication log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	log.ID = id
	return nil
}

// GetLogByID retrieves a medication log by ID
func (r *MedicationRepository) GetLogByID(id int64) (*models.MedicationLog, error) {
	query := `
		SELECT id, medication_id, logged_by, timestamp, taken, notes, created_at
		FROM medication_logs
		WHERE id = ?
	`
	var log models.MedicationLog
	err := r.db.QueryRow(query, id).Scan(
		&log.ID,
		&log.MedicationID,
		&log.LoggedBy,
		&log.Timestamp,
		&log.Taken,
		&log.Notes,
		&log.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get medication log: %w", err)
	}

	return &log, nil
}

// UpdateLog updates a medication log entry
func (r *MedicationRepository) UpdateLog(log *models.MedicationLog) error {
	query := `
		UPDATE medication_logs
		SET medication_id = ?, logged_by = ?, timestamp = ?, taken = ?, notes = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query,
		log.MedicationID,
		log.LoggedBy,
		log.Timestamp,
		log.Taken,
		log.Notes,
		log.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update medication log: %w", err)
	}
	return nil
}

// DeleteLog deletes a medication log
func (r *MedicationRepository) DeleteLog(id int64) error {
	query := `DELETE FROM medication_logs WHERE id = ?`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete medication log: %w", err)
	}
	return nil
}

// ListLogs retrieves medication logs for a specific medication with pagination
func (r *MedicationRepository) ListLogs(medicationID int64, limit, offset int) ([]*models.MedicationLog, error) {
	query := `
		SELECT id, medication_id, logged_by, timestamp, taken, notes, created_at
		FROM medication_logs
		WHERE medication_id = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, medicationID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list medication logs: %w", err)
	}
	defer rows.Close()

	return r.scanMedicationLogs(rows)
}

// ListLogsByDateRange retrieves medication logs within a date range
func (r *MedicationRepository) ListLogsByDateRange(medicationID int64, startDate, endDate time.Time, limit, offset int) ([]*models.MedicationLog, error) {
	query := `
		SELECT id, medication_id, logged_by, timestamp, taken, notes, created_at
		FROM medication_logs
		WHERE medication_id = ? AND timestamp BETWEEN ? AND ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, medicationID, startDate, endDate, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list medication logs by date range: %w", err)
	}
	defer rows.Close()

	return r.scanMedicationLogs(rows)
}

// GetRecentLogs retrieves the most recent medication logs for a medication
func (r *MedicationRepository) GetRecentLogs(medicationID int64, count int) ([]*models.MedicationLog, error) {
	query := `
		SELECT id, medication_id, logged_by, timestamp, taken, notes, created_at
		FROM medication_logs
		WHERE medication_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := r.db.Query(query, medicationID, count)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent medication logs: %w", err)
	}
	defer rows.Close()

	return r.scanMedicationLogs(rows)
}

// CountLogs counts medication logs for a specific medication
func (r *MedicationRepository) CountLogs(medicationID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM medication_logs WHERE medication_id = ?`
	var count int64
	err := r.db.QueryRow(query, medicationID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count medication logs: %w", err)
	}
	return count, nil
}

// GetAdherenceRate calculates the adherence rate for a medication over a date range
func (r *MedicationRepository) GetAdherenceRate(medicationID int64, startDate, endDate time.Time) (float64, error) {
	query := `
		SELECT
			COUNT(CASE WHEN taken = 1 THEN 1 END) * 100.0 / COUNT(*) AS adherence_rate
		FROM medication_logs
		WHERE medication_id = ? AND timestamp BETWEEN ? AND ?
	`
	var rate sql.NullFloat64
	err := r.db.QueryRow(query, medicationID, startDate, endDate).Scan(&rate)
	if err != nil {
		return 0, fmt.Errorf("failed to get adherence rate: %w", err)
	}
	if !rate.Valid {
		return 0, nil
	}
	return rate.Float64, nil
}

// scanMedications is a helper to scan multiple medication rows
func (r *MedicationRepository) scanMedications(rows *sql.Rows) ([]*models.Medication, error) {
	var medications []*models.Medication
	for rows.Next() {
		var medication models.Medication
		err := rows.Scan(
			&medication.ID,
			&medication.Name,
			&medication.Dosage,
			&medication.Frequency,
			&medication.StartDate,
			&medication.EndDate,
			&medication.IsActive,
			&medication.Notes,
			&medication.ScheduledTime,
			&medication.TimeWindowMinutes,
			&medication.ReminderEnabled,
			&medication.CreatedAt,
			&medication.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan medication: %w", err)
		}
		medications = append(medications, &medication)
	}

	return medications, rows.Err()
}

// scanMedicationLogs is a helper to scan multiple medication log rows
func (r *MedicationRepository) scanMedicationLogs(rows *sql.Rows) ([]*models.MedicationLog, error) {
	var logs []*models.MedicationLog
	for rows.Next() {
		var log models.MedicationLog
		err := rows.Scan(
			&log.ID,
			&log.MedicationID,
			&log.LoggedBy,
			&log.Timestamp,
			&log.Taken,
			&log.Notes,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan medication log: %w", err)
		}
		logs = append(logs, &log)
	}

	return logs, rows.Err()
}