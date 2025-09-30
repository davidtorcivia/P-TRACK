package repository

import (
	"database/sql"
	"fmt"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/models"
)

type InjectionRepository struct {
	db *database.DB
}

func NewInjectionRepository(db *database.DB) *InjectionRepository {
	return &InjectionRepository{db: db}
}

// Create creates a new injection record
func (r *InjectionRepository) Create(injection *models.Injection) error {
	query := `
		INSERT INTO injections (course_id, administered_by, timestamp, side, site_x, site_y, pain_level, has_knots, site_reaction, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	result, err := r.db.Exec(query,
		injection.CourseID,
		injection.AdministeredBy,
		injection.Timestamp,
		injection.Side,
		injection.SiteX,
		injection.SiteY,
		injection.PainLevel,
		injection.HasKnots,
		injection.SiteReaction,
		injection.Notes,
	)
	if err != nil {
		return fmt.Errorf("failed to create injection: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	injection.ID = id
	return nil
}

// GetByID retrieves an injection by ID
func (r *InjectionRepository) GetByID(id int64) (*models.Injection, error) {
	query := `
		SELECT id, course_id, administered_by, timestamp, side, site_x, site_y, pain_level, has_knots, site_reaction, notes, created_at, updated_at
		FROM injections
		WHERE id = ?
	`
	var injection models.Injection
	err := r.db.QueryRow(query, id).Scan(
		&injection.ID,
		&injection.CourseID,
		&injection.AdministeredBy,
		&injection.Timestamp,
		&injection.Side,
		&injection.SiteX,
		&injection.SiteY,
		&injection.PainLevel,
		&injection.HasKnots,
		&injection.SiteReaction,
		&injection.Notes,
		&injection.CreatedAt,
		&injection.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get injection: %w", err)
	}

	return &injection, nil
}

// Update updates an injection record
func (r *InjectionRepository) Update(injection *models.Injection) error {
	query := `
		UPDATE injections
		SET course_id = ?, administered_by = ?, timestamp = ?, side = ?, site_x = ?, site_y = ?, pain_level = ?, has_knots = ?, site_reaction = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	_, err := r.db.Exec(query,
		injection.CourseID,
		injection.AdministeredBy,
		injection.Timestamp,
		injection.Side,
		injection.SiteX,
		injection.SiteY,
		injection.PainLevel,
		injection.HasKnots,
		injection.SiteReaction,
		injection.Notes,
		injection.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update injection: %w", err)
	}
	return nil
}

// Delete deletes an injection
func (r *InjectionRepository) Delete(id int64) error {
	query := `DELETE FROM injections WHERE id = ?`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete injection: %w", err)
	}
	return nil
}

// List retrieves all injections with pagination
func (r *InjectionRepository) List(limit, offset int) ([]*models.Injection, error) {
	query := `
		SELECT id, course_id, administered_by, timestamp, side, site_x, site_y, pain_level, has_knots, site_reaction, notes, created_at, updated_at
		FROM injections
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list injections: %w", err)
	}
	defer rows.Close()

	return r.scanInjections(rows)
}

// ListByCourse retrieves all injections for a specific course
func (r *InjectionRepository) ListByCourse(courseID int64, limit, offset int) ([]*models.Injection, error) {
	query := `
		SELECT id, course_id, administered_by, timestamp, side, site_x, site_y, pain_level, has_knots, site_reaction, notes, created_at, updated_at
		FROM injections
		WHERE course_id = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, courseID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list injections by course: %w", err)
	}
	defer rows.Close()

	return r.scanInjections(rows)
}

// ListByDateRange retrieves injections within a date range
func (r *InjectionRepository) ListByDateRange(startDate, endDate time.Time, limit, offset int) ([]*models.Injection, error) {
	query := `
		SELECT id, course_id, administered_by, timestamp, side, site_x, site_y, pain_level, has_knots, site_reaction, notes, created_at, updated_at
		FROM injections
		WHERE timestamp BETWEEN ? AND ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, startDate, endDate, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list injections by date range: %w", err)
	}
	defer rows.Close()

	return r.scanInjections(rows)
}

// GetRecent retrieves the most recent injections
func (r *InjectionRepository) GetRecent(count int) ([]*models.Injection, error) {
	query := `
		SELECT id, course_id, administered_by, timestamp, side, site_x, site_y, pain_level, has_knots, site_reaction, notes, created_at, updated_at
		FROM injections
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := r.db.Query(query, count)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent injections: %w", err)
	}
	defer rows.Close()

	return r.scanInjections(rows)
}

// GetLastBySide retrieves the most recent injection for a specific side
func (r *InjectionRepository) GetLastBySide(side string) (*models.Injection, error) {
	query := `
		SELECT id, course_id, administered_by, timestamp, side, site_x, site_y, pain_level, has_knots, site_reaction, notes, created_at, updated_at
		FROM injections
		WHERE side = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`
	var injection models.Injection
	err := r.db.QueryRow(query, side).Scan(
		&injection.ID,
		&injection.CourseID,
		&injection.AdministeredBy,
		&injection.Timestamp,
		&injection.Side,
		&injection.SiteX,
		&injection.SiteY,
		&injection.PainLevel,
		&injection.HasKnots,
		&injection.SiteReaction,
		&injection.Notes,
		&injection.CreatedAt,
		&injection.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get last injection by side: %w", err)
	}

	return &injection, nil
}

// CountByCourse counts injections for a specific course
func (r *InjectionRepository) CountByCourse(courseID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM injections WHERE course_id = ?`
	var count int64
	err := r.db.QueryRow(query, courseID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count injections by course: %w", err)
	}
	return count, nil
}

// CountByDateRange counts injections within a date range
func (r *InjectionRepository) CountByDateRange(startDate, endDate time.Time) (int64, error) {
	query := `SELECT COUNT(*) FROM injections WHERE timestamp BETWEEN ? AND ?`
	var count int64
	err := r.db.QueryRow(query, startDate, endDate).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count injections by date range: %w", err)
	}
	return count, nil
}

// GetSiteHistory retrieves injection sites within the last N days for heat map visualization
func (r *InjectionRepository) GetSiteHistory(side string, days int) ([]*models.Injection, error) {
	query := `
		SELECT id, course_id, administered_by, timestamp, side, site_x, site_y, pain_level, has_knots, site_reaction, notes, created_at, updated_at
		FROM injections
		WHERE side = ? AND site_x IS NOT NULL AND site_y IS NOT NULL AND timestamp >= datetime('now', ? || ' days')
		ORDER BY timestamp DESC
	`
	rows, err := r.db.Query(query, side, fmt.Sprintf("-%d", days))
	if err != nil {
		return nil, fmt.Errorf("failed to get site history: %w", err)
	}
	defer rows.Close()

	return r.scanInjections(rows)
}

// scanInjections is a helper to scan multiple injection rows
func (r *InjectionRepository) scanInjections(rows *sql.Rows) ([]*models.Injection, error) {
	var injections []*models.Injection
	for rows.Next() {
		var injection models.Injection
		err := rows.Scan(
			&injection.ID,
			&injection.CourseID,
			&injection.AdministeredBy,
			&injection.Timestamp,
			&injection.Side,
			&injection.SiteX,
			&injection.SiteY,
			&injection.PainLevel,
			&injection.HasKnots,
			&injection.SiteReaction,
			&injection.Notes,
			&injection.CreatedAt,
			&injection.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan injection: %w", err)
		}
		injections = append(injections, &injection)
	}

	return injections, rows.Err()
}