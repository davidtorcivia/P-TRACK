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

// Create creates a new injection record (course_id must belong to account - verified by caller)
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

// GetByID retrieves an injection by ID and account (ensures data isolation via course)
func (r *InjectionRepository) GetByID(id int64, accountID int64) (*models.Injection, error) {
	query := `
		SELECT i.id, i.course_id, i.administered_by, i.timestamp, i.side, i.site_x, i.site_y, i.pain_level, i.has_knots, i.site_reaction, i.notes, i.created_at, i.updated_at
		FROM injections i
		JOIN courses c ON c.id = i.course_id
		WHERE i.id = ? AND c.account_id = ?
	`
	var injection models.Injection
	err := r.db.QueryRow(query, id, accountID).Scan(
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

// Update updates an injection record (only if it belongs to the account via course)
func (r *InjectionRepository) Update(injection *models.Injection, accountID int64) error {
	query := `
		UPDATE injections
		SET course_id = ?, administered_by = ?, timestamp = ?, side = ?, site_x = ?, site_y = ?, pain_level = ?, has_knots = ?, site_reaction = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		AND EXISTS (SELECT 1 FROM courses WHERE id = ? AND account_id = ?)
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
		injection.ID,
		injection.CourseID,
		accountID,
	)
	if err != nil {
		return fmt.Errorf("failed to update injection: %w", err)
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

// Delete deletes an injection (only if it belongs to the account via course)
func (r *InjectionRepository) Delete(id int64, accountID int64) error {
	query := `
		DELETE FROM injections
		WHERE id = ?
		AND EXISTS (SELECT 1 FROM courses WHERE id = injections.course_id AND account_id = ?)
	`
	result, err := r.db.Exec(query, id, accountID)
	if err != nil {
		return fmt.Errorf("failed to delete injection: %w", err)
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

// List retrieves all injections for an account with pagination
func (r *InjectionRepository) List(accountID int64, limit, offset int) ([]*models.Injection, error) {
	query := `
		SELECT i.id, i.course_id, i.administered_by, i.timestamp, i.side, i.site_x, i.site_y, i.pain_level, i.has_knots, i.site_reaction, i.notes, i.created_at, i.updated_at
		FROM injections i
		JOIN courses c ON c.id = i.course_id
		WHERE c.account_id = ?
		ORDER BY i.timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list injections: %w", err)
	}
	defer rows.Close()

	return r.scanInjections(rows)
}

// ListByCourse retrieves all injections for a specific course (course must belong to account)
func (r *InjectionRepository) ListByCourse(courseID int64, accountID int64, limit, offset int) ([]*models.Injection, error) {
	query := `
		SELECT i.id, i.course_id, i.administered_by, i.timestamp, i.side, i.site_x, i.site_y, i.pain_level, i.has_knots, i.site_reaction, i.notes, i.created_at, i.updated_at
		FROM injections i
		JOIN courses c ON c.id = i.course_id
		WHERE i.course_id = ? AND c.account_id = ?
		ORDER BY i.timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, courseID, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list injections by course: %w", err)
	}
	defer rows.Close()

	return r.scanInjections(rows)
}

// ListByDateRange retrieves injections within a date range for an account
func (r *InjectionRepository) ListByDateRange(accountID int64, startDate, endDate time.Time, limit, offset int) ([]*models.Injection, error) {
	query := `
		SELECT i.id, i.course_id, i.administered_by, i.timestamp, i.side, i.site_x, i.site_y, i.pain_level, i.has_knots, i.site_reaction, i.notes, i.created_at, i.updated_at
		FROM injections i
		JOIN courses c ON c.id = i.course_id
		WHERE c.account_id = ? AND i.timestamp BETWEEN ? AND ?
		ORDER BY i.timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, accountID, startDate, endDate, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list injections by date range: %w", err)
	}
	defer rows.Close()

	return r.scanInjections(rows)
}

// GetRecent retrieves the most recent injections for an account
func (r *InjectionRepository) GetRecent(accountID int64, count int) ([]*models.Injection, error) {
	query := `
		SELECT i.id, i.course_id, i.administered_by, i.timestamp, i.side, i.site_x, i.site_y, i.pain_level, i.has_knots, i.site_reaction, i.notes, i.created_at, i.updated_at
		FROM injections i
		JOIN courses c ON c.id = i.course_id
		WHERE c.account_id = ?
		ORDER BY i.timestamp DESC
		LIMIT ?
	`
	rows, err := r.db.Query(query, accountID, count)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent injections: %w", err)
	}
	defer rows.Close()

	return r.scanInjections(rows)
}

// GetLastBySide retrieves the most recent injection for a specific side for an account
func (r *InjectionRepository) GetLastBySide(accountID int64, side string) (*models.Injection, error) {
	query := `
		SELECT i.id, i.course_id, i.administered_by, i.timestamp, i.side, i.site_x, i.site_y, i.pain_level, i.has_knots, i.site_reaction, i.notes, i.created_at, i.updated_at
		FROM injections i
		JOIN courses c ON c.id = i.course_id
		WHERE c.account_id = ? AND i.side = ?
		ORDER BY i.timestamp DESC
		LIMIT 1
	`
	var injection models.Injection
	err := r.db.QueryRow(query, accountID, side).Scan(
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

// CountByCourse counts injections for a specific course (course must belong to account)
func (r *InjectionRepository) CountByCourse(courseID int64, accountID int64) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM injections i
		JOIN courses c ON c.id = i.course_id
		WHERE i.course_id = ? AND c.account_id = ?
	`
	var count int64
	err := r.db.QueryRow(query, courseID, accountID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count injections by course: %w", err)
	}
	return count, nil
}

// CountByDateRange counts injections within a date range for an account
func (r *InjectionRepository) CountByDateRange(accountID int64, startDate, endDate time.Time) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM injections i
		JOIN courses c ON c.id = i.course_id
		WHERE c.account_id = ? AND i.timestamp BETWEEN ? AND ?
	`
	var count int64
	err := r.db.QueryRow(query, accountID, startDate, endDate).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count injections by date range: %w", err)
	}
	return count, nil
}

// GetSiteHistory retrieves injection sites within the last N days for heat map visualization (for an account)
func (r *InjectionRepository) GetSiteHistory(accountID int64, side string, days int) ([]*models.Injection, error) {
	query := `
		SELECT i.id, i.course_id, i.administered_by, i.timestamp, i.side, i.site_x, i.site_y, i.pain_level, i.has_knots, i.site_reaction, i.notes, i.created_at, i.updated_at
		FROM injections i
		JOIN courses c ON c.id = i.course_id
		WHERE c.account_id = ? AND i.side = ? AND i.site_x IS NOT NULL AND i.site_y IS NOT NULL AND i.timestamp >= datetime('now', ? || ' days')
		ORDER BY i.timestamp DESC
	`
	rows, err := r.db.Query(query, accountID, side, fmt.Sprintf("-%d", days))
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
