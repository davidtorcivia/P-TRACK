# Couples Tracking Implementation - Remaining Work

## COMPLETED ✓
1. Database migration (005_add_accounts_multi_user.sql)
2. Models updated with Account, AccountMember, AccountInvitation
3. JWT Claims enhanced with AccountID and Role
4. Auth middleware updated to extract AccountID
5. Auth handlers (login/register) create/fetch accounts
6. Account Repository fully implemented
7. Course Repository - ALL methods filter by account_id
8. Medication Repository - ALL methods filter by account_id
9. Injection Repository - ALL methods filter via courses.account_id JOIN
10. Symptom Repository - ALL methods filter via courses.account_id JOIN
11. All Course handlers updated with accountID
12. All Medication handlers updated with accountID
13. All Web handlers updated with accountID
14. Application compiles successfully

## REMAINING WORK
### 1. InventoryRepository (IN PROGRESS)
- Need to add account_id parameter to all methods
- Update all queries to include WHERE account_id = ?
- Add account_id to all SELECT/Scan operations

### 2. Update Injection Handlers  
- Add accountID extraction from context
- Pass accountID to all injectionRepo method calls
- Files: internal/handlers/injection_handlers.go

### 3. Update Symptom Handlers
- Add accountID extraction from context  
- Pass accountID to all symptomRepo method calls
- Files: internal/handlers/symptom_handlers.go

### 4. Update Inventory Handlers
- Add accountID extraction from context
- Pass accountID to all inventoryRepo method calls  
- Files: internal/handlers/inventory_handlers.go

### 5. Create Account Management Handlers
- GET /api/account - get current user's account info
- PUT /api/account - update account name
- GET /api/account/members - list account members
- POST /api/account/leave - leave account (if not owner)
- DELETE /api/account/members/:userID - remove member (owner only)
- PUT /api/account/members/:userID/role - update member role

### 6. Create Invitation Handlers  
- POST /api/account/invitations - create invitation (generates token)
- GET /api/account/invitations - list pending invitations
- DELETE /api/account/invitations/:id - revoke invitation
- POST /api/auth/accept-invitation?token=XXX - accept invitation (add user to account)

### 7. Update Router
- Register new account management endpoints
- Register invitation endpoints
- Ensure all endpoints use auth middleware

### 8. Testing
- Test account isolation (users can't access other account's data)
- Test invitation flow end-to-end
- Test that both users in couple can see/edit same data
- Integration tests for multi-user scenarios

### 9. Final Steps
- Update documentation
- Test in development environment
- Create PR with comprehensive description

## SECURITY NOTES
- All repositories enforce account filtering at DB layer
- JWT tokens include accountID - validated on every request
- Invitation tokens are SHA-256 hashed in database
- Users can only belong to ONE account (enforced by UNIQUE constraint)
- Owner role required for sensitive operations (remove members, delete account)

## ESTIMATED TIME REMAINING
- Inventory Repository: 30 min
- Handler updates (3 files): 1-2 hours
- Account/Invitation handlers: 2-3 hours
- Router updates: 30 min
- Testing: 1-2 hours
- **Total: 5-8 hours**
