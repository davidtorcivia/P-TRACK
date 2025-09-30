package repository

import (
	"database/sql"
	"fmt"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/models"
)

type CourseRepository struct {
	db *database.DB
}

func NewCourseRepository(db *database.DB) *CourseRepository {
	return &CourseRepository{db: db}
}

// Create creates a new course
func (r *CourseRepository) Create(course *models.Course) error {
	query := `
		INSERT INTO courses (name, start_date, expected_end_date, is_active, notes, created_at, updated_at, created_by)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)
	`
	result, err := r.db.Exec(query,
		course.Name,
		course.StartDate,
		course.ExpectedEndDate,
		course.IsActive,
		course.Notes,
		course.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("failed to create course: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	course.ID = id
	return nil
}

// GetByID retrieves a course by ID
func (r *CourseRepository) GetByID(id int64) (*models.Course, error) {
	query := `
		SELECT id, name, start_date, expected_end_date, actual_end_date, is_active, notes, created_at, updated_at, created_by
		FROM courses
		WHERE id = ?
	`
	var course models.Course
	err := r.db.QueryRow(query, id).Scan(
		&course.ID,
		&course.Name,
		&course.StartDate,
		&course.ExpectedEndDate,
		&course.ActualEndDate,
		&course.IsActive,
		&course.Notes,
		&course.CreatedAt,
		&course.UpdatedAt,
		&course.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get course: %w", err)
	}

	return &course, nil
}

// GetActiveCourse retrieves the currently active course
func (r *CourseRepository) GetActiveCourse() (*models.Course, error) {
	query := `
		SELECT id, name, start_date, expected_end_date, actual_end_date, is_active, notes, created_at, updated_at, created_by
		FROM courses
		WHERE is_active = 1
		ORDER BY start_date DESC
		LIMIT 1
	`
	var course models.Course
	err := r.db.QueryRow(query).Scan(
		&course.ID,
		&course.Name,
		&course.StartDate,
		&course.ExpectedEndDate,
		&course.ActualEndDate,
		&course.IsActive,
		&course.Notes,
		&course.CreatedAt,
		&course.UpdatedAt,
		&course.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active course: %w", err)
	}

	return &course, nil
}

// Update updates a course
func (r *CourseRepository) Update(course *models.Course) error {
	query := `
		UPDATE courses
		SET name = ?, start_date = ?, expected_end_date = ?, actual_end_date = ?, is_active = ?, notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	_, err := r.db.Exec(query,
		course.Name,
		course.StartDate,
		course.ExpectedEndDate,
		course.ActualEndDate,
		course.IsActive,
		course.Notes,
		course.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update course: %w", err)
	}
	return nil
}

// Activate sets a course as active and deactivates all other courses
func (r *CourseRepository) Activate(id int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Deactivate all courses
	query := `UPDATE courses SET is_active = 0, updated_at = CURRENT_TIMESTAMP`
	_, err = tx.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to deactivate courses: %w", err)
	}

	// Activate the specified course
	query = `UPDATE courses SET is_active = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err = tx.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to activate course: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Close closes a course by setting the actual end date and deactivating it
func (r *CourseRepository) Close(id int64, endDate time.Time) error {
	query := `
		UPDATE courses
		SET actual_end_date = ?, is_active = 0, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`
	_, err := r.db.Exec(query, endDate, id)
	if err != nil {
		return fmt.Errorf("failed to close course: %w", err)
	}
	return nil
}

// Reopen reopens a closed course by clearing the actual end date and activating it
func (r *CourseRepository) Reopen(id int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Deactivate all courses
	query := `UPDATE courses SET is_active = 0, updated_at = CURRENT_TIMESTAMP`
	_, err = tx.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to deactivate courses: %w", err)
	}

	// Reopen and activate the specified course
	query = `UPDATE courses SET actual_end_date = NULL, is_active = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err = tx.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to reopen course: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Delete deletes a course (will cascade delete all related data)
func (r *CourseRepository) Delete(id int64) error {
	query := `DELETE FROM courses WHERE id = ?`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete course: %w", err)
	}
	return nil
}

// List retrieves all courses
func (r *CourseRepository) List() ([]*models.Course, error) {
	query := `
		SELECT id, name, start_date, expected_end_date, actual_end_date, is_active, notes, created_at, updated_at, created_by
		FROM courses
		ORDER BY start_date DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list courses: %w", err)
	}
	defer rows.Close()

	return r.scanCourses(rows)
}

// ListActive retrieves all active courses
func (r *CourseRepository) ListActive() ([]*models.Course, error) {
	query := `
		SELECT id, name, start_date, expected_end_date, actual_end_date, is_active, notes, created_at, updated_at, created_by
		FROM courses
		WHERE is_active = 1
		ORDER BY start_date DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list active courses: %w", err)
	}
	defer rows.Close()

	return r.scanCourses(rows)
}

// ListCompleted retrieves all completed courses
func (r *CourseRepository) ListCompleted() ([]*models.Course, error) {
	query := `
		SELECT id, name, start_date, expected_end_date, actual_end_date, is_active, notes, created_at, updated_at, created_by
		FROM courses
		WHERE is_active = 0 AND actual_end_date IS NOT NULL
		ORDER BY actual_end_date DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list completed courses: %w", err)
	}
	defer rows.Close()

	return r.scanCourses(rows)
}

// scanCourses is a helper to scan multiple course rows
func (r *CourseRepository) scanCourses(rows *sql.Rows) ([]*models.Course, error) {
	var courses []*models.Course
	for rows.Next() {
		var course models.Course
		err := rows.Scan(
			&course.ID,
			&course.Name,
			&course.StartDate,
			&course.ExpectedEndDate,
			&course.ActualEndDate,
			&course.IsActive,
			&course.Notes,
			&course.CreatedAt,
			&course.UpdatedAt,
			&course.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan course: %w", err)
		}
		courses = append(courses, &course)
	}

	return courses, rows.Err()
}