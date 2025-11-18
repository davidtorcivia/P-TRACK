package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"injection-tracker/internal/database"
	"injection-tracker/internal/middleware"
	"injection-tracker/internal/models"
	"injection-tracker/internal/repository"

	"github.com/go-chi/chi/v5"
)

// ============================================
// REQUEST/RESPONSE TYPES
// ============================================

type UpdateAccountRequest struct {
	Name *string `json:"name,omitempty"`
}

type CreateInvitationRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"` // 'owner' or 'member'
}

type InvitationResponse struct {
	ID          int64     `json:"id"`
	Email       string    `json:"email"`
	Token       string    `json:"token,omitempty"` // Only included on creation
	InvitedBy   int64     `json:"invited_by"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
}

type AcceptInvitationRequest struct {
	Token string `json:"token"`
}

type UpdateMemberRoleRequest struct {
	Role string `json:"role"` // 'owner' or 'member'
}

// ============================================
// ACCOUNT MANAGEMENT HANDLERS
// ============================================

// HandleGetAccount returns the current user's account information
func HandleGetAccount(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		accountRepo := repository.NewAccountRepository(db.DB)
		account, err := accountRepo.GetByID(accountID)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Account not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to retrieve account", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(account)
	}
}

// HandleUpdateAccount updates the account name
func HandleUpdateAccount(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req UpdateAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == nil {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		accountRepo := repository.NewAccountRepository(db.DB)
		if err := accountRepo.UpdateName(accountID, *req.Name); err != nil {
			http.Error(w, "Failed to update account", http.StatusInternalServerError)
			return
		}

		// Return updated account
		account, err := accountRepo.GetByID(accountID)
		if err != nil {
			http.Error(w, "Failed to retrieve updated account", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(account)
	}
}

// HandleGetAccountMembers returns all members of the current user's account
func HandleGetAccountMembers(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		accountRepo := repository.NewAccountRepository(db.DB)
		members, err := accountRepo.GetMembers(accountID)
		if err != nil {
			http.Error(w, "Failed to retrieve members", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(members)
	}
}

// HandleRemoveAccountMember removes a member from the account (owner only)
func HandleRemoveAccountMember(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		role := middleware.GetRole(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Only owner can remove members
		if role != "owner" {
			http.Error(w, "Forbidden: only account owner can remove members", http.StatusForbidden)
			return
		}

		// Get member user ID from URL
		memberIDStr := chi.URLParam(r, "userID")
		memberID, err := strconv.ParseInt(memberIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		// Prevent removing yourself
		if memberID == userID {
			http.Error(w, "Cannot remove yourself from account", http.StatusBadRequest)
			return
		}

		accountRepo := repository.NewAccountRepository(db.DB)
		if err := accountRepo.RemoveMember(accountID, memberID); err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Member not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to remove member", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleUpdateMemberRole updates a member's role (owner only)
func HandleUpdateMemberRole(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		role := middleware.GetRole(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Only owner can update roles
		if role != "owner" {
			http.Error(w, "Forbidden: only account owner can update roles", http.StatusForbidden)
			return
		}

		memberIDStr := chi.URLParam(r, "userID")
		memberID, err := strconv.ParseInt(memberIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		var req UpdateMemberRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Role != "owner" && req.Role != "member" {
			http.Error(w, "role must be 'owner' or 'member'", http.StatusBadRequest)
			return
		}

		accountRepo := repository.NewAccountRepository(db.DB)
		if err := accountRepo.UpdateMemberRole(accountID, memberID, req.Role); err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Member not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to update member role", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ============================================
// INVITATION HANDLERS
// ============================================

// HandleCreateInvitation creates a new invitation (owner/member can invite)
func HandleCreateInvitation(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req CreateInvitationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Email == "" {
			http.Error(w, "email is required", http.StatusBadRequest)
			return
		}

		if req.Role == "" {
			req.Role = "member"
		}

		if req.Role != "owner" && req.Role != "member" {
			http.Error(w, "role must be 'owner' or 'member'", http.StatusBadRequest)
			return
		}

		// Check if email is already registered
		userRepo := repository.NewUserRepository(db)
		existingUser, err := userRepo.GetByUsername(req.Email)
		if err == nil && existingUser != nil {
			http.Error(w, "A user with this email already exists", http.StatusConflict)
			return
		}

		accountRepo := repository.NewAccountRepository(db.DB)

		// Set expiration to 7 days from now
		expiresAt := time.Now().Add(7 * 24 * time.Hour)

		token, err := accountRepo.CreateInvitation(accountID, req.Email, userID, expiresAt)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create invitation: %v", err), http.StatusInternalServerError)
			return
		}

		// Retrieve the created invitation
		invitation, err := accountRepo.GetInvitationByToken(token)
		if err != nil {
			http.Error(w, "Invitation created but failed to retrieve", http.StatusInternalServerError)
			return
		}

		response := InvitationResponse{
			ID:         invitation.ID,
			Email:      invitation.Email,
			Token:      token, // Return the plain token (not hashed)
			InvitedBy:  invitation.InvitedBy,
			Role:       invitation.Role,
			CreatedAt:  invitation.CreatedAt,
			ExpiresAt:  invitation.ExpiresAt,
			AcceptedAt: nil,
		}
		if invitation.AcceptedAt.Valid {
			t := invitation.AcceptedAt.Time
			response.AcceptedAt = &t
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}
}

// HandleGetInvitations returns all invitations for the account (pending and accepted)
func HandleGetInvitations(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get all invitations including accepted ones
		rows, err := db.DB.Query(`
			SELECT
				id, account_id, email, invited_by, role,
				created_at, expires_at, accepted_at, accepted_by
			FROM account_invitations
			WHERE account_id = ?
			ORDER BY created_at DESC
		`, accountID)
		if err != nil {
			http.Error(w, "Failed to retrieve invitations", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var invitations []*models.AccountInvitation
		for rows.Next() {
			var inv models.AccountInvitation
			err = rows.Scan(
				&inv.ID,
				&inv.AccountID,
				&inv.Email,
				&inv.InvitedBy,
				&inv.Role,
				&inv.CreatedAt,
				&inv.ExpiresAt,
				&inv.AcceptedAt,
				&inv.AcceptedBy,
			)
			if err != nil {
				continue
			}
			invitations = append(invitations, &inv)
		}

		responses := make([]InvitationResponse, 0, len(invitations))
		for _, inv := range invitations {
			resp := InvitationResponse{
				ID:         inv.ID,
				Email:      inv.Email,
				InvitedBy:  inv.InvitedBy,
				Role:       inv.Role,
				CreatedAt:  inv.CreatedAt,
				ExpiresAt:  inv.ExpiresAt,
				AcceptedAt: nil,
			}
			if inv.AcceptedAt.Valid {
				t := inv.AcceptedAt.Time
				resp.AcceptedAt = &t
			}
			responses = append(responses, resp)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responses)
	}
}

// HandleRevokeInvitation revokes/deletes an invitation
func HandleRevokeInvitation(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		accountID := middleware.GetAccountID(r.Context())
		if userID == 0 || accountID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		invIDStr := chi.URLParam(r, "id")
		invID, err := strconv.ParseInt(invIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid invitation ID", http.StatusBadRequest)
			return
		}

		accountRepo := repository.NewAccountRepository(db.DB)
		if err := accountRepo.DeleteInvitation(invID); err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Invitation not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to revoke invitation", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleAcceptInvitation accepts an invitation and adds the user to the account
// This is called during user registration with an invite token
func HandleAcceptInvitation(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract token from query parameter
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "token is required", http.StatusBadRequest)
			return
		}

		// User must be authenticated
		userID := middleware.GetUserID(r.Context())
		if userID == 0 {
			http.Error(w, "Unauthorized: must be logged in to accept invitation", http.StatusUnauthorized)
			return
		}

		accountRepo := repository.NewAccountRepository(db.DB)

		// Verify invitation exists and is valid
		invitation, err := accountRepo.GetInvitationByToken(token)
		if err != nil {
			if err == repository.ErrNotFound {
				http.Error(w, "Invalid or expired invitation", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to verify invitation", http.StatusInternalServerError)
			return
		}

		// Check if already accepted
		if invitation.AcceptedAt.Valid {
			http.Error(w, "Invitation has already been accepted", http.StatusConflict)
			return
		}

		// Check if expired
		if time.Now().After(invitation.ExpiresAt) {
			http.Error(w, "Invitation has expired", http.StatusGone)
			return
		}

		// Check if user is already in an account
		currentAccount, err := accountRepo.GetUserAccount(userID)
		if err == nil && currentAccount != nil {
			http.Error(w, "You are already a member of an account. Please contact support to switch accounts.", http.StatusConflict)
			return
		}

		// Accept the invitation
		if err := accountRepo.AcceptInvitation(invitation.ID, userID); err != nil {
			http.Error(w, fmt.Sprintf("Failed to accept invitation: %v", err), http.StatusInternalServerError)
			return
		}

		// Return success with account info
		account, err := accountRepo.GetByID(invitation.AccountID)
		if err != nil {
			http.Error(w, "Invitation accepted but failed to retrieve account", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Invitation accepted successfully",
			"account": account,
		})
	}
}
