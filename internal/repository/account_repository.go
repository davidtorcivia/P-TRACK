package repository

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"injection-tracker/internal/models"
)

var (
	ErrAccountNotFound    = errors.New("account not found")
	ErrInvitationNotFound = errors.New("invitation not found")
	ErrInvitationExpired  = errors.New("invitation has expired")
	ErrInvitationUsed     = errors.New("invitation already used")
	ErrUserAlreadyInAccount = errors.New("user already belongs to an account")
)

type AccountRepository struct {
	db *sql.DB
}

func NewAccountRepository(db *sql.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// ==============================================
// ACCOUNT CRUD OPERATIONS
// ==============================================

// Create creates a new account and adds the creator as owner
func (r *AccountRepository) Create(name *string, ownerUserID int64) (int64, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create account
	var accountID int64
	if name != nil {
		err = tx.QueryRow(`
			INSERT INTO accounts (name, created_at, updated_at)
			VALUES (?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			RETURNING id
		`, *name).Scan(&accountID)
	} else {
		err = tx.QueryRow(`
			INSERT INTO accounts (created_at, updated_at)
			VALUES (CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			RETURNING id
		`).Scan(&accountID)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to create account: %w", err)
	}

	// Add owner as member
	_, err = tx.Exec(`
		INSERT INTO account_members (account_id, user_id, role, joined_at)
		VALUES (?, ?, 'owner', CURRENT_TIMESTAMP)
	`, accountID, ownerUserID)

	if err != nil {
		return 0, fmt.Errorf("failed to add owner to account: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return accountID, nil
}

// GetByID retrieves an account by ID
func (r *AccountRepository) GetByID(accountID int64) (*models.Account, error) {
	var account models.Account
	var name sql.NullString

	err := r.db.QueryRow(`
		SELECT id, name, created_at, updated_at
		FROM accounts
		WHERE id = ?
	`, accountID).Scan(&account.ID, &name, &account.CreatedAt, &account.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	account.Name = name
	return &account, nil
}

// GetUserAccount gets the account for a specific user
func (r *AccountRepository) GetUserAccount(userID int64) (*models.Account, error) {
	var account models.Account
	var name sql.NullString

	err := r.db.QueryRow(`
		SELECT a.id, a.name, a.created_at, a.updated_at
		FROM accounts a
		JOIN account_members am ON am.account_id = a.id
		WHERE am.user_id = ?
	`, userID).Scan(&account.ID, &name, &account.CreatedAt, &account.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user account: %w", err)
	}

	account.Name = name
	return &account, nil
}

// UpdateName updates an account's name
func (r *AccountRepository) UpdateName(accountID int64, name string) error {
	result, err := r.db.Exec(`
		UPDATE accounts
		SET name = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, accountID)

	if err != nil {
		return fmt.Errorf("failed to update account name: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrAccountNotFound
	}

	return nil
}

// Delete deletes an account and all associated data (CASCADE)
func (r *AccountRepository) Delete(accountID int64) error {
	result, err := r.db.Exec(`DELETE FROM accounts WHERE id = ?`, accountID)
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrAccountNotFound
	}

	return nil
}

// ==============================================
// ACCOUNT MEMBER OPERATIONS
// ==============================================

// GetMembers retrieves all members of an account with their usernames
func (r *AccountRepository) GetMembers(accountID int64) ([]*models.AccountMember, error) {
	rows, err := r.db.Query(`
		SELECT
			am.account_id,
			am.user_id,
			am.role,
			am.joined_at,
			am.invited_by,
			u.username
		FROM account_members am
		JOIN users u ON u.id = am.user_id
		WHERE am.account_id = ?
		ORDER BY am.joined_at ASC
	`, accountID)

	if err != nil {
		return nil, fmt.Errorf("failed to query members: %w", err)
	}
	defer rows.Close()

	var members []*models.AccountMember
	for rows.Next() {
		var member models.AccountMember
		var invitedBy sql.NullInt64

		err = rows.Scan(
			&member.AccountID,
			&member.UserID,
			&member.Role,
			&member.JoinedAt,
			&invitedBy,
			&member.Username,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}

		member.InvitedBy = invitedBy
		members = append(members, &member)
	}

	return members, nil
}

// GetMember retrieves a specific member's information
func (r *AccountRepository) GetMember(accountID, userID int64) (*models.AccountMember, error) {
	var member models.AccountMember
	var invitedBy sql.NullInt64

	err := r.db.QueryRow(`
		SELECT
			am.account_id,
			am.user_id,
			am.role,
			am.joined_at,
			am.invited_by,
			u.username
		FROM account_members am
		JOIN users u ON u.id = am.user_id
		WHERE am.account_id = ? AND am.user_id = ?
	`, accountID, userID).Scan(
		&member.AccountID,
		&member.UserID,
		&member.Role,
		&member.JoinedAt,
		&invitedBy,
		&member.Username,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("member not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get member: %w", err)
	}

	member.InvitedBy = invitedBy
	return &member, nil
}

// AddMember adds a user to an account
func (r *AccountRepository) AddMember(accountID, userID int64, role string, invitedBy int64) error {
	// Check if user already belongs to an account
	var existingAccountID int64
	err := r.db.QueryRow(`
		SELECT account_id FROM account_members WHERE user_id = ?
	`, userID).Scan(&existingAccountID)

	if err == nil {
		// User already has an account
		if existingAccountID == accountID {
			return nil // Already a member of this account
		}
		return ErrUserAlreadyInAccount
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing membership: %w", err)
	}

	// Add user to account
	_, err = r.db.Exec(`
		INSERT INTO account_members (account_id, user_id, role, joined_at, invited_by)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?)
	`, accountID, userID, role, invitedBy)

	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	return nil
}

// RemoveMember removes a user from an account
func (r *AccountRepository) RemoveMember(accountID, userID int64) error {
	result, err := r.db.Exec(`
		DELETE FROM account_members
		WHERE account_id = ? AND user_id = ?
	`, accountID, userID)

	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("member not found")
	}

	return nil
}

// UpdateMemberRole updates a member's role
func (r *AccountRepository) UpdateMemberRole(accountID, userID int64, role string) error {
	result, err := r.db.Exec(`
		UPDATE account_members
		SET role = ?
		WHERE account_id = ? AND user_id = ?
	`, role, accountID, userID)

	if err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("member not found")
	}

	return nil
}

// ==============================================
// INVITATION OPERATIONS
// ==============================================

// generateToken generates a secure random token for invitations
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// hashToken creates a SHA-256 hash of a token for database storage
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(hash[:])
}

// CreateInvitation creates an invitation and returns the token (not hashed)
func (r *AccountRepository) CreateInvitation(accountID int64, email string, invitedBy int64, expiresAt time.Time) (string, error) {
	// Generate token
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	tokenHash := hashToken(token)

	// Insert invitation
	_, err = r.db.Exec(`
		INSERT INTO account_invitations (
			account_id, email, token_hash, invited_by, role, created_at, expires_at
		) VALUES (?, ?, ?, ?, 'member', CURRENT_TIMESTAMP, ?)
	`, accountID, email, tokenHash, invitedBy, expiresAt)

	if err != nil {
		return "", fmt.Errorf("failed to create invitation: %w", err)
	}

	return token, nil
}

// GetInvitationByToken retrieves an invitation by its token
func (r *AccountRepository) GetInvitationByToken(token string) (*models.AccountInvitation, error) {
	tokenHash := hashToken(token)

	var invitation models.AccountInvitation
	var acceptedAt sql.NullTime
	var acceptedBy sql.NullInt64

	err := r.db.QueryRow(`
		SELECT
			i.id,
			i.account_id,
			i.email,
			i.token_hash,
			i.invited_by,
			i.role,
			i.created_at,
			i.expires_at,
			i.accepted_at,
			i.accepted_by,
			u.username
		FROM account_invitations i
		JOIN users u ON u.id = i.invited_by
		WHERE i.token_hash = ?
	`, tokenHash).Scan(
		&invitation.ID,
		&invitation.AccountID,
		&invitation.Email,
		&invitation.TokenHash,
		&invitation.InvitedBy,
		&invitation.Role,
		&invitation.CreatedAt,
		&invitation.ExpiresAt,
		&acceptedAt,
		&acceptedBy,
		&invitation.InviterUsername,
	)

	if err == sql.ErrNoRows {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get invitation: %w", err)
	}

	invitation.AcceptedAt = acceptedAt
	invitation.AcceptedBy = acceptedBy
	invitation.IsExpired = time.Now().After(invitation.ExpiresAt)

	return &invitation, nil
}

// GetPendingInvitations retrieves all pending (not accepted) invitations for an account
func (r *AccountRepository) GetPendingInvitations(accountID int64) ([]*models.AccountInvitation, error) {
	rows, err := r.db.Query(`
		SELECT
			i.id,
			i.account_id,
			i.email,
			i.token_hash,
			i.invited_by,
			i.role,
			i.created_at,
			i.expires_at,
			i.accepted_at,
			i.accepted_by,
			u.username
		FROM account_invitations i
		JOIN users u ON u.id = i.invited_by
		WHERE i.account_id = ? AND i.accepted_at IS NULL
		ORDER BY i.created_at DESC
	`, accountID)

	if err != nil {
		return nil, fmt.Errorf("failed to query invitations: %w", err)
	}
	defer rows.Close()

	var invitations []*models.AccountInvitation
	now := time.Now()

	for rows.Next() {
		var invitation models.AccountInvitation
		var acceptedAt sql.NullTime
		var acceptedBy sql.NullInt64

		err = rows.Scan(
			&invitation.ID,
			&invitation.AccountID,
			&invitation.Email,
			&invitation.TokenHash,
			&invitation.InvitedBy,
			&invitation.Role,
			&invitation.CreatedAt,
			&invitation.ExpiresAt,
			&acceptedAt,
			&acceptedBy,
			&invitation.InviterUsername,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan invitation: %w", err)
		}

		invitation.AcceptedAt = acceptedAt
		invitation.AcceptedBy = acceptedBy
		invitation.IsExpired = now.After(invitation.ExpiresAt)

		invitations = append(invitations, &invitation)
	}

	return invitations, nil
}

// AcceptInvitation marks an invitation as accepted and adds user to account
func (r *AccountRepository) AcceptInvitation(invitationID, userID int64) error {
	// Get the invitation details first
	var accountID int64
	var role string
	err := r.db.QueryRow(`
		SELECT account_id, role
		FROM account_invitations
		WHERE id = ?
	`, invitationID).Scan(&accountID, &role)

	if err != nil {
		return fmt.Errorf("failed to get invitation details: %w", err)
	}

	// Begin transaction
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update the invitation to mark it as accepted
	_, err = tx.Exec(`
		UPDATE account_invitations
		SET accepted_at = CURRENT_TIMESTAMP, accepted_by = ?
		WHERE id = ?
	`, userID, invitationID)
	if err != nil {
		return fmt.Errorf("failed to update invitation: %w", err)
	}

	// Add user to account_members
	_, err = tx.Exec(`
		INSERT INTO account_members (account_id, user_id, role, joined_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, accountID, userID, role)
	if err != nil {
		return fmt.Errorf("failed to add user to account members: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteInvitation deletes an invitation (e.g., revoke before acceptance)
func (r *AccountRepository) DeleteInvitation(invitationID int64) error {
	result, err := r.db.Exec(`DELETE FROM account_invitations WHERE id = ?`, invitationID)
	if err != nil {
		return fmt.Errorf("failed to delete invitation: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrInvitationNotFound
	}

	return nil
}

// ValidateInvitation checks if an invitation is valid (not expired, not used)
func (r *AccountRepository) ValidateInvitation(invitation *models.AccountInvitation) error {
	if invitation.AcceptedAt.Valid {
		return ErrInvitationUsed
	}

	if time.Now().After(invitation.ExpiresAt) {
		return ErrInvitationExpired
	}

	return nil
}
