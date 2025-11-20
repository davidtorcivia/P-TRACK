# P-TRACK Technical Documentation

## Table of Contents
1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Technology Stack](#technology-stack)
4. [Project Structure](#project-structure)
5. [Database Schema](#database-schema)
6. [Core Components](#core-components)
7. [API Endpoints](#api-endpoints)
8. [Notification System](#notification-system)
9. [Inventory & Expiration System](#inventory--expiration-system)
10. [Security](#security)
11. [Development Workflow](#development-workflow)
12. [Testing](#testing)
13. [Deployment](#deployment)
14. [Troubleshooting](#troubleshooting)

---

## Overview

P-TRACK (Progesterone Injection Tracker) is a web application designed to help couples/families track progesterone injections, medications, symptoms, and medical supplies. Built with Go and SQLite, it provides a fast, lightweight, self-hosted solution with multi-user support.

### Key Features
- **Quick Injection Logging**: Fast, mobile-first interface for logging injections
- **Multi-User Support**: Couples/family members share data within an account
- **Inventory Management**: Automatic tracking with low-stock and expiration alerts
- **Notification System**: Real-time alerts for low stock and expiring items
- **Symptom & Medication Tracking**: Comprehensive health monitoring
- **PWA Support**: Installable as a mobile app with offline capabilities

---

## Architecture

### High-Level Architecture

```
┌─────────────────┐
│   Client        │
│  (Browser/PWA)  │
│                 │
│  HTMX + Alpine  │
│  Service Worker │
└────────┬────────┘
         │ HTTPS
         ▼
┌─────────────────┐
│  Nginx Proxy    │
│  (Optional)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Go HTTP Server │
│  - Chi Router   │
│  - Middleware   │
│  - Handlers     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Business Logic  │
│  - Repositories │
│  - Services     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  SQLite DB      │
│  (WAL Mode)     │
└─────────────────┘
```

###  Request Flow

1. **Client Request** → Browser/PWA sends HTMX request
2. **Middleware** → Auth check, rate limiting, CSRF validation
3. **Handler** → Processes request, validates input
4. **Repository** → Database operations
5. **Response** → HTML fragment or JSON returned to client
6. **HTMX** → Updates DOM with response

---

## Technology Stack

### Backend
- **Language**: Go 1.21+
- **Web Framework**: Chi Router
- **Database**: SQLite3 with WAL mode
- **Authentication**: JWT (golang-jwt/jwt)
- **Password Hashing**: bcrypt
- **Migrations**: Custom migration system

### Frontend
- **HTMX 1.9+**: Server-driven interactions
- **Alpine.js 3.x**: Minimal client-side reactivity
- **Pico CSS**: Classless semantic styling
- **Chart.js**: Data visualization
- **Service Worker**: PWA functionality

### Development
- **Build Tool**: Go standard toolchain
- **Testing**: Go testing package
- **Linting**: golangci-lint (optional)

---

## Project Structure

```
P-TRACK/
├── cmd/
│   └── server/
│       └── main.go                 # Application entry point
│
├── internal/
│   ├── auth/                       # Authentication logic
│   │   ├── jwt.go                  # JWT management
│   │   └── password.go             # Password hashing
│   │
│   ├── config/                     # Configuration
│   │   └── config.go
│   │
│   ├── database/                   # Database connection
│   │   └── database.go
│   │
│   ├── handlers/                   # HTTP request handlers
│   │   ├── auth_handlers.go        # Login, register, logout
│   │   ├── injection_handlers.go   # Injection logging
│   │   ├── inventory_handlers.go   # Inventory management
│   │   ├── notification_handlers.go # Notifications API
│   │   ├── medication_handlers.go  # Medication tracking
│   │   ├── symptom_handlers.go     # Symptom logging
│   │   ├── course_handlers.go      # Course management
│   │   ├── account_handlers.go     # Account & invitations
│   │   ├── settings_handlers.go    # Settings management
│   │   ├── export_handlers.go      # PDF/CSV export
│   │   └── web_handlers.go         # Web page handlers
│   │
│   ├── middleware/                 # HTTP middleware
│   │   ├── auth.go                 # JWT authentication
│   │   ├── security.go             # Security headers, CSRF
│   │   └── logging.go              # Request logging
│   │
│   ├── models/                     # Data models
│   │   └── models.go
│   │
│   ├── repository/                 # Database access layer
│   │   ├── user_repository.go
│   │   ├── account_repository.go
│   │   ├── course_repository.go
│   │   ├── injection_repository.go
│   │   ├── symptom_repository.go
│   │   ├── medication_repository.go
│   │   ├── inventory_repository.go
│   │   ├── notification_repository.go  # NEW
│   │   └── audit_repository.go
│   │
│   ├── services/                   # Business logic services
│   │   └── notification_service.go # NEW
│   │
│   └── web/                        # Web utilities
│       ├── templates.go
│       └── helpers.go
│
├── migrations/                     # Database migrations
│   ├── 001_initial_schema.sql
│   ├── 002_add_medication_time_windows.sql
│   ├── 003_update_symptom_constraints.sql
│   ├── 004_fix_symptom_constraints.sql
│   └── 005_add_accounts_multi_user.sql
│
├── static/                         # Static assets
│   ├── css/
│   ├── js/
│   ├── icons/
│   ├── sw.js                       # Service worker
│   └── manifest.json               # PWA manifest
│
├── templates/                      # HTML templates
│   ├── pages/
│   ├── components/
│   └── layouts/
│
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

---

## Database Schema

### Core Tables

#### `accounts`
- Multi-user support (couples/families)
- One-to-many with users via `account_members`

```sql
CREATE TABLE accounts (
    id INTEGER PRIMARY KEY,
    name TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

#### `users`
- Individual user accounts
- Linked to one account

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    email TEXT,
    is_active BOOLEAN DEFAULT 1,
    failed_login_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP,
    created_at TIMESTAMP,
    last_login TIMESTAMP
);
```

#### `account_members`
- Join table for users and accounts
- Roles: 'owner' or 'member'

```sql
CREATE TABLE account_members (
    account_id INTEGER NOT NULL REFERENCES accounts(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    role TEXT NOT NULL CHECK(role IN ('owner', 'member')),
    joined_at TIMESTAMP,
    invited_by INTEGER REFERENCES users(id),
    PRIMARY KEY (account_id, user_id)
);
```

#### `courses`
- Treatment cycles/periods
- Belongs to an account

```sql
CREATE TABLE courses (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    start_date DATE NOT NULL,
    expected_end_date DATE,
    actual_end_date DATE,
    is_active BOOLEAN DEFAULT 1,
    notes TEXT,
    account_id INTEGER NOT NULL REFERENCES accounts(id),
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    created_by INTEGER REFERENCES users(id)
);
```

#### `injections`
- Individual injection records
- Linked to courses (which belong to accounts)

```sql
CREATE TABLE injections (
    id INTEGER PRIMARY KEY,
    course_id INTEGER NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    administered_by INTEGER REFERENCES users(id),
    timestamp TIMESTAMP NOT NULL,
    side TEXT NOT NULL CHECK(side IN ('left', 'right')),
    site_x REAL,  -- Advanced mode coordinates
    site_y REAL,
    pain_level INTEGER CHECK(pain_level BETWEEN 1 AND 10),
    has_knots BOOLEAN,
    site_reaction TEXT,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

#### `inventory_items`
- Medical supplies tracking
- Belongs to an account

```sql
CREATE TABLE inventory_items (
    id INTEGER PRIMARY KEY,
    item_type TEXT NOT NULL CHECK(item_type IN (
        'progesterone', 'draw_needle', 'injection_needle',
        'syringe', 'swab', 'gauze'
    )),
    quantity REAL NOT NULL CHECK(quantity >= 0),
    unit TEXT NOT NULL CHECK(unit IN ('mL', 'count')),
    expiration_date DATE,              -- NEW: Used for expiration tracking
    lot_number TEXT,
    low_stock_threshold REAL,
    notes TEXT,
    account_id INTEGER NOT NULL REFERENCES accounts(id),
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    UNIQUE(item_type, account_id)
);
```

#### `notifications`
- User notifications for alerts

```sql
CREATE TABLE notifications (
    id INTEGER PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK(type IN (
        'injection_reminder', 'low_stock', 'missed_injection',
        'expiration_warning', 'system'
    )),
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    is_read BOOLEAN DEFAULT 0,
    scheduled_time TIMESTAMP,
    created_at TIMESTAMP
);
```

### Data Access Pattern
All user data is scoped by `account_id`:
1. User logs in → Get their `user_id`
2. Look up `account_id` from `account_members`
3. Filter all queries by `account_id`

This ensures multi-user accounts share data while maintaining security.

---

## Core Components

### 1. Authentication Flow

#### Registration
```
POST /api/auth/register
├── Validate username/password
├── Hash password (bcrypt cost 12)
├── Create account
├── Create user
├── Link user to account (as owner)
└── Return success
```

#### Login
```
POST /api/auth/login
├── Rate limiting (5 attempts per 15 min)
├── Find user by username
├── Verify password
├── Generate JWT (2-week expiry)
├── Set httpOnly cookie
└── Return user data + JWT
```

#### Middleware
```go
// RequireAuth middleware
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := extractTokenFromCookie(r)
        claims, err := m.jwtManager.ValidateToken(token)
        if err != nil {
            http.Error(w, "Unauthorized", 401)
            return
        }
        ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 2. Injection Logging

#### Quick Log Flow
```
1. User clicks "Log Injection"
2. Modal shows: [LEFT] [RIGHT] buttons
3. User selects side
4. HTMX POST /api/injections
   ├── Create injection record
   ├── Auto-decrement inventory (1mL progesterone, 1 needle, etc.)
   ├── Create audit log
   └── Return success HTML fragment
5. UI updates with new injection
```

#### Inventory Auto-Deduction
```go
func (r *InjectionRepository) Create(injection *Injection) error {
    tx, _ := r.db.Begin()
    defer tx.Rollback()

    // Insert injection
    result, _ := tx.Exec("INSERT INTO injections ...")
    injectionID, _ := result.LastInsertId()

    // Decrement inventory
    r.inventoryRepo.DecrementForInjection(injectionID, accountID, userID, 1.0)

    tx.Commit()
    return nil
}
```

### 3. Repository Pattern

All data access goes through repositories:

```go
// Example: InjectionRepository
type InjectionRepository struct {
    db *database.DB
}

// GetByID retrieves an injection by ID (with account check)
func (r *InjectionRepository) GetByID(id, accountID int64) (*Injection, error) {
    query := `
        SELECT i.* FROM injections i
        JOIN courses c ON i.course_id = c.id
        WHERE i.id = ? AND c.account_id = ?
    `
    // ... scan and return
}
```

**Key Principle**: Always filter by `account_id` to ensure data isolation.

---

## API Endpoints

### Authentication
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/register` | Register new user |
| POST | `/api/auth/login` | Login |
| POST | `/api/auth/logout` | Logout |
| GET | `/api/auth/me` | Get current user |

### Injections
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/injections` | List injections |
| POST | `/api/injections` | Create injection |
| GET | `/api/injections/{id}` | Get injection |
| PUT | `/api/injections/{id}` | Update injection |
| DELETE | `/api/injections/{id}` | Delete injection |
| GET | `/api/injections/stats` | Get statistics |

### Inventory
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/inventory` | List all inventory items |
| PUT | `/api/inventory/{itemType}` | Update inventory item |
| POST | `/api/inventory/{itemType}/adjust` | Manual adjustment |
| GET | `/api/inventory/alerts` | Get low stock & expiration alerts ⭐ |
| GET | `/api/inventory/{itemType}/history` | Get change history |

### Notifications ⭐ NEW
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/notifications` | List notifications |
| GET | `/api/notifications/unread-count` | Get unread count |
| PUT | `/api/notifications/{id}/read` | Mark as read |
| POST | `/api/notifications/mark-all-read` | Mark all as read |
| DELETE | `/api/notifications/{id}` | Delete notification |

### Courses
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/courses` | List courses |
| POST | `/api/courses` | Create course |
| GET | `/api/courses/active` | Get active course |
| POST | `/api/courses/{id}/activate` | Activate course |
| POST | `/api/courses/{id}/close` | Close course |

---

## Notification System

### Architecture

The notification system consists of three components:

1. **NotificationRepository** (`internal/repository/notification_repository.go`)
   - Database operations (CRUD)
   - Duplicate prevention
   - Specialized creators for low-stock and expiration alerts

2. **NotificationHandlers** (`internal/handlers/notification_handlers.go`)
   - HTTP API endpoints
   - JSON responses
   - User-scoped queries

3. **NotificationService** (`internal/services/notification_service.go`)
   - Business logic
   - Periodic checks
   - Batch notification creation

### How It Works

#### 1. Creating Notifications

```go
// In notification_repository.go
func (r *NotificationRepository) CreateLowStockNotification(
    userID sql.NullInt64,
    itemType string,
    quantity, threshold float64,
    severity string,
) error {
    // Check for duplicates (within last 24 hours)
    exists, _ := r.notificationExists(userID, "low_stock", itemType, 24)
    if exists {
        return nil // Don't create duplicate
    }

    // Create notification
    notification := &models.Notification{
        UserID:  userID,
        Type:    "low_stock",
        Title:   "Low Stock Alert",
        Message: fmt.Sprintf("%s is running low...", itemType),
    }
    return r.Create(notification)
}
```

#### 2. Checking Inventory

```go
// In notification_service.go
func (s *NotificationService) CheckAndCreateInventoryNotifications(accountID int64) error {
    // Get all users in account
    userIDs, _ := s.getUserIDsForAccount(accountID)

    // Check low stock
    lowStockItems, _ := s.inventoryRepo.ListLowStock(accountID)
    for _, item := range lowStockItems {
        for _, userID := range userIDs {
            s.notificationRepo.CreateLowStockNotification(...)
        }
    }

    // Check expirations
    items, _ := s.inventoryRepo.List(accountID)
    for _, item := range items {
        if item.ExpirationDate.Valid {
            daysUntil := time.Until(item.ExpirationDate.Time).Hours() / 24
            if daysUntil <= 30 {
                for _, userID := range userIDs {
                    s.notificationRepo.CreateExpirationNotification(...)
                }
            }
        }
    }
}
```

#### 3. Triggering Notifications

**Option A: On Inventory Change**
```go
// In inventory_handlers.go HandleUpdateInventory
func HandleUpdateInventory(db *database.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // ... update inventory ...

        // Trigger notification check
        notifService := services.NewNotificationService(db)
        notifService.CheckAndCreateInventoryNotifications(accountID)

        w.WriteHeader(http.StatusOK)
    }
}
```

**Option B: Background Worker (Recommended for Production)**
```go
// Add to main.go
go func() {
    ticker := time.NewTicker(24 * time.Hour)
    notifService := services.NewNotificationService(db)

    for range ticker.C {
        notifService.CheckAndCreateNotificationsForAllAccounts()
        notifService.CleanupOldNotifications(30) // Delete read notifications >30 days old
    }
}()
```

### Notification Types

| Type | Description | Severity |
|------|-------------|----------|
| `low_stock` | Inventory below threshold | warning/critical |
| `expiration_warning` | Item expiring within 30 days | warning |
| `expiration_warning` | Item expired | critical |
| `injection_reminder` | Reminder to log injection | info |
| `system` | System messages | info |

---

## Inventory & Expiration System

### Inventory Alerts Endpoint

**GET /api/inventory/alerts**

Returns both low stock AND expiration alerts:

```json
{
  "alerts": [
    {
      "item_type": "progesterone",
      "quantity": 2.5,
      "low_stock_threshold": 5.0,
      "unit": "mL",
      "severity": "warning",
      "alert_type": "low_stock",
      "message": "Progesterone is running low (2.5 mL remaining)"
    },
    {
      "item_type": "draw_needle",
      "quantity": 10,
      "unit": "count",
      "severity": "critical",
      "alert_type": "expiring",
      "expiration_date": "2025-12-01T00:00:00Z",
      "days_until_expiry": 5,
      "message": "Draw Needles expires in 5 days (on Dec 1, 2025)"
    },
    {
      "item_type": "syringe",
      "quantity": 8,
      "unit": "count",
      "severity": "critical",
      "alert_type": "expired",
      "expiration_date": "2025-11-15T00:00:00Z",
      "days_until_expiry": -4,
      "message": "Syringes expired on Nov 15, 2025 - please dispose and restock"
    }
  ],
  "count": 3
}
```

### Expiration Logic

```go
// In HandleGetInventoryAlerts
expirationRows, _ := db.Query(`
    SELECT item_type, quantity, unit, expiration_date
    FROM inventory_items
    WHERE account_id = ?
      AND expiration_date IS NOT NULL
      AND expiration_date <= date('now', '+30 days')  -- Within 30 days
    ORDER BY expiration_date ASC
`, accountID)

for expirationRows.Next() {
    // ... scan ...
    daysUntil := int(time.Until(expirationDate).Hours() / 24)

    if expirationDate.Before(now) {
        alert.AlertType = "expired"
        alert.Severity = "critical"
    } else if daysUntil <= 7 {
        alert.AlertType = "expiring"
        alert.Severity = "critical"  // Less than 7 days
    } else {
        alert.AlertType = "expiring"
        alert.Severity = "warning"   // 7-30 days
    }
}
```

---

## Security

### Authentication
- **Password Hashing**: bcrypt with cost factor 12
- **JWT**: HS256 signing, httpOnly cookies, 2-week expiry
- **Rate Limiting**: 5 login attempts per 15 minutes
- **Account Lockout**: After 5 failed attempts, lock for 15 minutes

### Authorization
- **Middleware**: All protected routes require valid JWT
- **Account Scoping**: All queries filtered by `account_id`
- **CSRF Protection**: CSRF tokens for state-changing operations

### Input Validation
- **SQL Injection**: All queries use prepared statements
- **XSS**: HTML escaped in templates
- **Content Security Policy**: Enabled via middleware

### Audit Logging
- All data modifications logged with:
  - User ID
  - Action type
  - Entity type/ID
  - Timestamp
  - IP address (optional)

---

## Development Workflow

### Setup
```bash
# Clone repository
git clone <repo-url>
cd P-TRACK

# Install dependencies
go mod download

# Set up environment
cp .env.example .env
# Edit .env with your values

# Run migrations (automatic on startup)

# Build
go build -o server ./cmd/server/

# Run
./server
```

### Running Locally
```bash
# Development mode
go run cmd/server/main.go

# Or using Make
make run

# Access at http://localhost:8080
```

### Database Migrations
Migrations run automatically on startup. To add a new migration:

1. Create `migrations/00X_description.sql`
2. Write UP migration SQL
3. Restart server (migrations auto-run)

### Testing
```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/repository -v

# Run with coverage
go test -cover ./...
```

---

## Testing

### Unit Tests
Test individual functions in isolation:

```go
func TestNotificationRepository_Create(t *testing.T) {
    db := setupTestDB(t)
    repo := NewNotificationRepository(db)

    notification := &models.Notification{...}
    err := repo.Create(notification)

    assert.NoError(t, err)
    assert.NotZero(t, notification.ID)
}
```

### Integration Tests
Test full request/response cycles:

```go
func TestHandleCreateInjection(t *testing.T) {
    db := setupTestDB(t)
    handler := HandleCreateInjection(db)

    req := httptest.NewRequest("POST", "/api/injections", body)
    w := httptest.NewRecorder()

    handler.ServeHTTP(w, req)

    assert.Equal(t, 200, w.Code)
}
```

---

## Deployment

### Docker Deployment

```bash
# Build image
docker build -t p-track .

# Run with Docker Compose
docker-compose up -d

# View logs
docker-compose logs -f

# Stop
docker-compose down
```

### Environment Variables
```env
# Required
JWT_SECRET=<long-random-string>
DATABASE_PATH=/app/data/tracker.db

# Optional
SERVER_PORT=8080
SESSION_DURATION=336h  # 2 weeks
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW=60s
```

### Production Checklist
- [ ] Set strong `JWT_SECRET`
- [ ] Enable HTTPS (Let's Encrypt)
- [ ] Configure backup strategy
- [ ] Set up monitoring (optional)
- [ ] Configure rate limits
- [ ] Enable audit logging
- [ ] Test disaster recovery

---

## Troubleshooting

### Common Issues

#### 1. "Database locked"
**Cause**: SQLite WAL mode not enabled or concurrent writes
**Solution**: Ensure WAL mode is enabled in `database.go`:
```go
_, err = db.Exec("PRAGMA journal_mode=WAL")
```

#### 2. "Unauthorized" on valid requests
**Cause**: JWT expired or invalid
**Solution**: Check JWT expiry and secret configuration

#### 3. Inventory not decrementing
**Cause**: Transaction rollback or missing inventory items
**Solution**: Check logs, ensure inventory items exist before injection

#### 4. Notifications not appearing
**Cause**: Notification service not running
**Solution**: Call `CheckAndCreateInventoryNotifications()` or set up background worker

### Debug Mode
Enable debug logging:
```go
log.SetFlags(log.LstdFlags | log.Lshortfile)
log.Println("Debug info:", data)
```

### Database Inspection
```bash
# Open SQLite database
sqlite3 data/tracker.db

# Common queries
SELECT * FROM users;
SELECT * FROM notifications WHERE is_read = 0;
SELECT * FROM inventory_items WHERE expiration_date < date('now');
```

---

## Key Files for New Developers

### Start Here
1. `cmd/server/main.go` - Application entry point, routing
2. `internal/models/models.go` - Data structures
3. `CLAUDE.md` - Product requirements document

### Common Tasks

**Adding a new API endpoint:**
1. Add route in `cmd/server/main.go`
2. Create handler in `internal/handlers/`
3. Add repository method if needed
4. Test with curl or browser

**Adding a new database table:**
1. Create migration file in `migrations/`
2. Add model in `internal/models/models.go`
3. Create repository in `internal/repository/`

**Modifying notification logic:**
1. Edit `internal/services/notification_service.go`
2. Update repository if needed
3. Test with sample data

---

## Support

### Documentation
- **Product Spec**: See `CLAUDE.md`
- **API Reference**: See this document
- **Deployment**: See `DEPLOYMENT_COMPLETE.md`

### Getting Help
- Check existing code for patterns
- Review test files for examples
- Search codebase with `grep -r "pattern" internal/`

---

**Last Updated**: November 2025
**Version**: 1.0.0 with Notification & Expiration System
