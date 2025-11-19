# Couples Tracking Implementation - Progress & Remaining Work

## ✅ COMPLETED - Foundational Architecture

### 1. Database Layer ✅
- **Migration Created**: `/migrations/005_add_accounts_multi_user.sql`
  - Created `accounts` table for family/couple grouping
  - Created `account_members` join table (users ↔ accounts mapping)
  - Created `account_invitations` table for onboarding partners
  - Added `account_id` to: courses, medications, inventory_items
  - Migrated existing users to have their own accounts
  - Added proper indexes for performance

### 2. Models ✅
- **File**: `/internal/models/models.go`
  - Added `Account`, `AccountMember`, `AccountInvitation` models
  - Added `AccountID` field to `Course`, `Medication`, `InventoryItem`

### 3. Authentication & JWT ✅
- **File**: `/internal/auth/jwt.go`
  - Updated `Claims` struct to include `AccountID` and `Role`
  - Updated `GenerateToken()` to accept and include account info
  - Updated `RefreshToken()` to preserve account info

### 4. Middleware ✅
- **File**: `/internal/middleware/auth.go`
  - Updated `UserContext` to include `AccountID` and `Role`
  - Added `GetAccountID(ctx)` helper function
  - Added `GetRole(ctx)` helper function
  - Auth middleware now extracts account info from JWT

### 5. Repositories ✅
- **AccountRepository**: `/internal/repository/account_repository.go` ✅
  - Complete CRUD for accounts
  - Member management (add, remove, update role)
  - Invitation system (create, validate, accept)
  - Token generation and hashing for secure invites

- **CourseRepository**: `/internal/repository/course_repository.go` ✅
  - ALL methods updated to filter by `account_id`
  - `Create()` now requires accountID
  - `GetByID()`, `Update()`, `Delete()`, `Activate()`, `Close()`, `Reopen()` all enforce account ownership
  - `List()`, `ListActive()`, `ListCompleted()` all filter by account
  - **SECURITY**: Prevents cross-account data access

- **MedicationRepository**: `/internal/repository/medication_repository.go` ✅
  - ALL medication methods updated to filter by `account_id`
  - `Create()`, `GetByID()`, `Update()`, `Delete()`, `HardDelete()` enforce account ownership
  - `List()`, `ListActive()` filter by account
  - **SECURITY**: Medications isolated per account

### 6. Auth Handlers ✅
- **File**: `/internal/handlers/auth_handlers.go`
  - **HandleLogin**: Now fetches user's account and includes accountID + role in JWT
  - **HandleRegister**: Creates an account automatically when new user registers
  - **CRITICAL**: Account creation is atomic with user creation (rollback on failure)

---

## ⚠️ REMAINING WORK

### 1. Complete Repository Updates 🔨
**Priority: HIGH** - Required for compilation

#### InjectionRepository & SymptomRepository
- These inherit account filtering via `JOIN` with courses table
- **Action needed**: Add helper methods that join with courses to filter by account
- Example query pattern:
  ```sql
  SELECT i.* FROM injections i
  JOIN courses c ON c.id = i.course_id
  WHERE c.account_id = ?
  ```

#### InventoryRepository
- **Action needed**: Add account filtering to all methods
- Similar pattern to MedicationRepository
- Update: `Create()`, `GetByItemType()`, `Update()`, `List()`, `GetHistory()`

### 2. Update ALL Data Handlers 🔨
**Priority: HIGH** - Required for compilation

#### Pattern to apply to EVERY handler:
```go
func SomeHandler(db *database.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := middleware.GetUserID(r.Context())
        accountID := middleware.GetAccountID(r.Context())  // ADD THIS
        if userID == 0 || accountID == 0 {                 // UPDATE THIS
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // Pass accountID to all repository method calls
        data, err := repo.GetByID(id, accountID)  // ADD accountID parameter
    }
}
```

#### Files needing updates:
- `/internal/handlers/course_handlers.go` - PARTIALLY DONE
  - **Remaining**: Lines 183, 215, 254, 297, 339, 350, 392, 403, 423, 467+
  - Add `accountID` parameter to ALL repository calls

- `/internal/handlers/injection_handlers.go`
  - Add accountID extraction
  - Pass to repository methods (via course validation)

- `/internal/handlers/symptom_handlers.go`
  - Add accountID extraction
  - Pass to repository methods (via course validation)

- `/internal/handlers/medication_handlers.go`
  - Add accountID extraction
  - Pass to ALL medication repository calls

- `/internal/handlers/inventory_handlers.go`
  - Add accountID extraction
  - Pass to ALL inventory repository calls

### 3. Create Account Management Handlers 📝
**Priority: MEDIUM** - New functionality

**File**: Create `/internal/handlers/account_handlers.go`

```go
// Endpoints needed:
// GET    /api/account - Get current user's account info
// PUT    /api/account/name - Update account name
// GET    /api/account/members - List all members
// DELETE /api/account/members/:id - Remove member (owner only)
```

Example implementation:
```go
func HandleGetAccount(db *database.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        accountID := middleware.GetAccountID(r.Context())
        accountRepo := repository.NewAccountRepository(db.DB)

        account, err := accountRepo.GetByID(accountID)
        if err != nil {
            http.Error(w, "Account not found", http.StatusNotFound)
            return
        }

        members, _ := accountRepo.GetMembers(accountID)

        respondJSON(w, http.StatusOK, map[string]interface{}{
            "account": account,
            "members": members,
        })
    }
}
```

### 4. Create Invitation Handlers 🎫
**Priority: MEDIUM** - Core couples feature

**File**: Create `/internal/handlers/invitation_handlers.go`

```go
// Endpoints needed:
// POST /api/account/invitations - Create invitation (returns token)
// POST /api/account/invitations/:token/accept - Accept invitation
// GET  /api/account/invitations - List pending invitations
// DELETE /api/account/invitations/:id - Cancel/revoke invitation
```

Example invitation flow:
```go
// 1. User A invites User B
POST /api/account/invitations
Body: {"email": "partner@example.com"}
Response: {"token": "abc123...", "expires_at": "..."}

// 2. User B registers with token
POST /api/auth/register?invite_token=abc123
Body: {"username": "partner", "password": "..."}
// Backend: Creates user, adds to inviter's account (not new account)

// 3. Or existing User B accepts
POST /api/account/invitations/abc123/accept
// Backend: Moves user from old account to new account
```

**IMPORTANT**: Update `HandleRegister` in auth_handlers.go to check for `invite_token` query param and join existing account instead of creating new one.

### 5. Update Router 🛣️
**Priority: HIGH** - Wire up new endpoints

**File**: `/cmd/server/main.go` or wherever routes are defined

```go
// Add account routes (protected by auth middleware)
r.Route("/api/account", func(r chi.Router) {
    r.Use(authMiddleware.RequireAuth)
    r.Get("/", handlers.HandleGetAccount(db))
    r.Put("/name", handlers.HandleUpdateAccountName(db))
    r.Get("/members", handlers.HandleGetMembers(db))
    r.Delete("/members/{id}", handlers.HandleRemoveMember(db))
})

// Add invitation routes
r.Route("/api/account/invitations", func(r chi.Router) {
    r.Use(authMiddleware.RequireAuth)
    r.Post("/", handlers.HandleCreateInvitation(db))
    r.Get("/", handlers.HandleListInvitations(db))
    r.Delete("/{id}", handlers.HandleRevokeInvitation(db))
    r.Post("/{token}/accept", handlers.HandleAcceptInvitation(db))
})
```

### 6. Testing 🧪
**Priority: HIGH** - Validate functionality and security

#### Unit Tests Needed:
- `account_repository_test.go` - Test all account CRUD operations
- `invitation_flow_test.go` - Test complete invitation acceptance
- `account_isolation_test.go` - **CRITICAL SECURITY TEST**

Example security test:
```go
func TestAccountIsolation(t *testing.T) {
    // Create two separate accounts
    account1 := createTestAccount(t)
    account2 := createTestAccount(t)

    // Create course in account 1
    course := createCourse(t, account1.ID)

    // Try to access course from account 2
    _, err := courseRepo.GetByID(course.ID, account2.ID)

    // MUST return ErrNotFound (security check)
    assert.Error(t, err)
    assert.Equal(t, repository.ErrNotFound, err)
}
```

#### Integration Tests:
- Full registration flow (creates account)
- Full login flow (loads account, includes in JWT)
- Invitation flow (create → accept → shared access)
- Data isolation (User A cannot see User B's data)

---

## 📋 QUICK FIX CHECKLIST

To get the app compiling and working quickly:

### Step 1: Fix Course Handlers (15 min)
```bash
# Search for all courseRepo calls in course_handlers.go
# Add accountID parameter to each call
# Pattern: courseRepo.MethodName(id) → courseRepo.MethodName(id, accountID)
```

Specific lines to fix in `/internal/handlers/course_handlers.go`:
- Line ~183: `GetActiveCourse()` → `GetActiveCourse(accountID)`
- Line ~215: `GetByID(id)` → `GetByID(id, accountID)`
- Line ~254: `GetByID(id)` → `GetByID(id, accountID)`
- Line ~297: `Update(course)` → `Update(course, accountID)`
- Line ~339: `GetByID(id)` → `GetByID(id, accountID)`
- Line ~350: `Delete(id)` → `Delete(id, accountID)`
- Line ~392: `GetByID(id)` → `GetByID(id, accountID)`
- Line ~403: `Activate(id)` → `Activate(id, accountID)`
- Line ~423: `GetByID(id)` → `GetByID(id, accountID)`
- Line ~467: `GetByID(id)` → `GetByID(id, accountID)`
- Line ~478: `Close(id, endDate)` → `Close(id, accountID, endDate)`

### Step 2: Apply Migration (5 min)
```bash
# The migration will run automatically on app startup
# Or manually: sqlite3 data/tracker.db < migrations/005_add_accounts_multi_user.sql
```

### Step 3: Test Basic Flow (10 min)
```bash
# 1. Register new user → should create account automatically
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser", "password": "password123"}'

# 2. Login → should receive JWT with accountID
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser", "password": "password123"}'

# 3. Create course → should belong to user's account
curl -X POST http://localhost:8080/api/courses \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "Test Course", "start_date": "2025-01-01"}'
```

---

## 🎯 MVP (Minimum Viable Product) Scope

To get couples tracking working ASAP, focus on:

1. ✅ **DONE**: Database migration, models, JWT, auth
2. 🔨 **FIX**: Course handlers (add accountID to all calls)
3. 📝 **CREATE**: Basic invitation handler
4. 🧪 **TEST**: Registration → Login → Create Course → Invite Partner → Partner Accepts → Both See Same Data

**Estimated time to MVP**: 2-3 hours of focused work

---

## 🔐 Security Checklist

Before deploying to production:

- [ ] ALL repository methods filter by `account_id`
- [ ] ALL handlers extract `accountID` from context
- [ ] NO handler allows accessing data from other accounts
- [ ] Invitation tokens are hashed in database
- [ ] Invitation tokens expire after 24-48 hours
- [ ] Invitation tokens are single-use
- [ ] Failed authorization attempts are logged
- [ ] Account deletion cascades properly
- [ ] User can only be in ONE account at a time
- [ ] Integration tests validate account isolation

---

## 📊 Current Status Summary

| Component | Status | Completion |
|-----------|--------|------------|
| Database Schema | ✅ Complete | 100% |
| Models | ✅ Complete | 100% |
| JWT & Middleware | ✅ Complete | 100% |
| AccountRepository | ✅ Complete | 100% |
| CourseRepository | ✅ Complete | 100% |
| MedicationRepository | ✅ Complete | 100% |
| Auth Handlers | ✅ Complete | 100% |
| Course Handlers | ⚠️ Partial | 30% |
| Other Data Handlers | ❌ Not Started | 0% |
| Account Handlers | ❌ Not Started | 0% |
| Invitation Handlers | ❌ Not Started | 0% |
| Router Updates | ❌ Not Started | 0% |
| Tests | ❌ Not Started | 0% |
| **OVERALL** | **⚠️ In Progress** | **~40%** |

---

## 💡 Implementation Tips

1. **Start with compilation**: Fix all course_handlers.go errors first
2. **Test incrementally**: After each handler update, test that endpoint
3. **Security first**: Always validate accountID matches before data access
4. **Use existing patterns**: Copy from CourseRepository/MedicationRepository updates
5. **Commit often**: Don't try to do everything in one commit

---

## 📞 Questions & Design Decisions Needed

1. **Account Switching**: Should users be able to switch between accounts? (Recommend: NO for v1)
2. **Max Members**: Limit accounts to 2 members (couples only) or allow more? (Recommend: 2 for v1)
3. **Invitation Expiry**: How long should invitation tokens be valid? (Recommend: 48 hours)
4. **Email**: Send actual emails or just provide token to copy-paste? (Recommend: Copy-paste for v1, email later)
5. **Account Deletion**: What happens when owner leaves? Transfer ownership or delete account? (Recommend: Delete account)
6. **Existing Data**: When user accepts invite, what happens to their old data? (Recommend: Orphaned/deleted)

---

**Last Updated**: 2025-11-17
**Author**: Claude Code Implementation
**Status**: Foundation Complete - Handlers Need Updates
