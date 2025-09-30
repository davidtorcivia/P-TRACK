package repository

import (
	"database/sql"
	"fmt"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/models"
)

type UserRepository struct {
	db *database.DB
}

func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(user *models.User) error {
	query := `
		INSERT INTO users (username, password_hash, email, is_active, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	result, err := r.db.Exec(query, user.Username, user.PasswordHash, user.Email, user.IsActive)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	user.ID = id
	return nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(id int64) (*models.User, error) {
	query := `
		SELECT id, username, password_hash, email, is_active,
		       failed_login_attempts, locked_until, created_at, last_login
		FROM users
		WHERE id = ?
	`
	var user models.User
	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.IsActive,
		&user.FailedLoginAttempts,
		&user.LockedUntil,
		&user.CreatedAt,
		&user.LastLogin,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetByUsername retrieves a user by username
func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	query := `
		SELECT id, username, password_hash, email, is_active,
		       failed_login_attempts, locked_until, created_at, last_login
		FROM users
		WHERE LOWER(username) = LOWER(?)
	`
	var user models.User
	err := r.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.IsActive,
		&user.FailedLoginAttempts,
		&user.LockedUntil,
		&user.CreatedAt,
		&user.LastLogin,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	return &user, nil
}

// UpdateLastLogin updates the last login timestamp
func (r *UserRepository) UpdateLastLogin(id int64) error {
	query := `UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}
	return nil
}

// IncrementFailedLogins increments the failed login counter
func (r *UserRepository) IncrementFailedLogins(id int64) error {
	query := `
		UPDATE users
		SET failed_login_attempts = failed_login_attempts + 1
		WHERE id = ?
	`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to increment failed logins: %w", err)
	}
	return nil
}

// ResetFailedLogins resets the failed login counter
func (r *UserRepository) ResetFailedLogins(id int64) error {
	query := `
		UPDATE users
		SET failed_login_attempts = 0, locked_until = NULL
		WHERE id = ?
	`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to reset failed logins: %w", err)
	}
	return nil
}

// LockAccount locks an account until the specified time
func (r *UserRepository) LockAccount(id int64, until time.Time) error {
	query := `UPDATE users SET locked_until = ? WHERE id = ?`
	_, err := r.db.Exec(query, until, id)
	if err != nil {
		return fmt.Errorf("failed to lock account: %w", err)
	}
	return nil
}

// IsAccountLocked checks if an account is currently locked
func (r *UserRepository) IsAccountLocked(id int64) (bool, error) {
	query := `
		SELECT locked_until
		FROM users
		WHERE id = ?
	`
	var lockedUntil sql.NullTime
	err := r.db.QueryRow(query, id).Scan(&lockedUntil)
	if err != nil {
		return false, fmt.Errorf("failed to check account lock: %w", err)
	}

	if !lockedUntil.Valid {
		return false, nil
	}

	return time.Now().Before(lockedUntil.Time), nil
}

// Update updates a user's information
func (r *UserRepository) Update(user *models.User) error {
	query := `
		UPDATE users
		SET username = ?, email = ?, is_active = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, user.Username, user.Email, user.IsActive, user.ID)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

// UpdatePassword updates a user's password hash
func (r *UserRepository) UpdatePassword(id int64, passwordHash string) error {
	query := `UPDATE users SET password_hash = ? WHERE id = ?`
	_, err := r.db.Exec(query, passwordHash, id)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return nil
}

// Delete deletes a user (soft delete by setting is_active to false)
func (r *UserRepository) Delete(id int64) error {
	query := `UPDATE users SET is_active = 0 WHERE id = ?`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

// List retrieves all users
func (r *UserRepository) List() ([]*models.User, error) {
	query := `
		SELECT id, username, password_hash, email, is_active,
		       failed_login_attempts, locked_until, created_at, last_login
		FROM users
		WHERE is_active = 1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.PasswordHash,
			&user.Email,
			&user.IsActive,
			&user.FailedLoginAttempts,
			&user.LockedUntil,
			&user.CreatedAt,
			&user.LastLogin,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &user)
	}

	return users, rows.Err()
}

var ErrNotFound = fmt.Errorf("not found")