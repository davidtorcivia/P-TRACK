# Progesterone Injection Tracker - Design Document

## 1. Project Overview

### 1.1 Purpose
A mobile-first web application for tracking progesterone injections, medication inventory, and related symptoms. Designed for family use with shared patient data and multi-user access.

### 1.2 Goals
- Enable quick, frictionless injection logging (side + timestamp)
- Track injection sites with visual heat map to avoid overuse
- Monitor pain, symptoms, and medication inventory
- Provide insights through calendar views and charts
- Support multiple family members managing shared patient data
- Ensure medical data privacy and security

### 1.3 Non-Goals
- Native mobile apps (PWA only)
- Multi-tenancy for unrelated users
- Integration with external medical systems
- HIPAA compliance (personal use only)

---

## 2. User Roles & Data Model

### 2.1 User Roles
- **Account Holder**: Can create account, manage users, view/edit all data
- **Family Member**: Can view/edit shared patient data, manage inventory
- All users within a family account have equal permissions (simplified model)

### 2.2 Core Entities
- **User**: Individual account (husband/wife)
- **Patient**: The person receiving injections (implicit - one per family)
- **Course**: A treatment cycle/period (e.g., "IVF Cycle 1", "Pregnancy Week 1-12")
- **Injection**: Individual injection record
- **Symptom Log**: Daily symptoms, pain, notes
- **Medication**: Pill/supplement tracking
- **Inventory**: Medical supplies tracking

---

## 3. Features & User Stories

### 3.1 Authentication
- **FR-1.1**: User can register with username/password
- **FR-1.2**: User can log in and receive JWT token (2 week expiry)
- **FR-1.3**: User can reset password via email (if SMTP configured)
- **FR-1.4**: Session automatically expires after 2 weeks
- **FR-1.5**: User can log out manually

### 3.2 Quick Injection Logging (PRIMARY FEATURE)
- **FR-2.1**: User sees large, prominent "Log Injection" button on home screen
- **FR-2.2**: Tapping opens quick-log modal with two buttons: "Left" | "Right"
- **FR-2.3**: Selecting side immediately logs injection with current timestamp
- **FR-2.4**: Optional "More Details" accordion reveals additional fields:
  - Pain level (1-10 slider)
  - Knots/hardness checkbox
  - Site reaction (dropdown: none, redness, swelling, bruising)
  - Notes (text field)
  - Administered by (auto-fills current user, can change)
- **FR-2.5**: Injection is auto-assigned to active course
- **FR-2.6**: Inventory is automatically decremented (1mL progesterone, 1 draw needle, 1 injection needle, 1 syringe, 1 swab)

### 3.3 Advanced Injection Site Tracking
- **FR-3.1**: User can toggle "Advanced Mode" in settings
- **FR-3.2**: In advanced mode, quick-log shows anatomical diagram (left/right buttocks)
- **FR-3.3**: User taps specific location on diagram to record injection site
- **FR-3.4**: Past injection sites shown as colored dots with opacity fade over configurable days (default 14)
- **FR-3.5**: Heat map indicates areas used most recently (darker = more recent)
- **FR-3.6**: Recommended injection site highlighted based on rotation pattern

### 3.4 Symptom & Pain Tracking
- **FR-4.1**: User can log daily symptoms separately from injections
- **FR-4.2**: Symptom log includes:
  - Pain level (1-10 slider)
  - Location (dropdown: injection site, abdomen, back, other)
  - Type (dropdown: sharp, dull, aching, cramping)
  - Other symptoms (multi-select: nausea, fatigue, headache, mood changes, custom)
  - Notes
- **FR-4.3**: Symptoms are timestamped and associated with active course

### 3.5 Medication/Pill Tracking
- **FR-5.1**: User can add medications with name, dosage, frequency
- **FR-5.2**: User can log when medication is taken
- **FR-5.3**: Medication adherence shown as calendar/checklist
- **FR-5.4**: Missed doses highlighted

### 3.6 Inventory Management
- **FR-6.1**: System tracks inventory for:
  - Progesterone (vials, mL remaining)
  - Draw needles (count)
  - Injection needles (count)
  - Syringes (count)
  - Alcohol swabs (count)
  - Gauze pads (count)
- **FR-6.2**: Inventory auto-decrements on injection logging
- **FR-6.3**: User can manually adjust inventory (with reason/note)
- **FR-6.4**: Low stock alerts (configurable thresholds)
- **FR-6.5**: Expiration date tracking with warnings
- **FR-6.6**: Inventory history log (all changes tracked)

### 3.7 Course Management
- **FR-7.1**: User can create new course with name, start date, expected end date
- **FR-7.2**: One course is "active" at a time
- **FR-7.3**: All injections/symptoms logged during active course period
- **FR-7.4**: User can close course (marks complete)
- **FR-7.5**: User can reopen/edit past courses
- **FR-7.6**: User can delete entire course (removes all associated data)

### 3.8 Data Visualization
- **FR-8.1**: Calendar view shows:
  - Injection dates (color-coded by side)
  - Symptom severity (color intensity)
  - Missed injections (if scheduled)
- **FR-8.2**: Charts/graphs:
  - Injection frequency over time
  - Pain trends (line graph)
  - Side alternation pattern
  - Symptom frequency
- **FR-8.3**: Export to PDF:
  - Summary report with date range
  - Injection log table
  - Charts included
  - Formatted for medical professionals

### 3.9 Notifications & Reminders
- **FR-9.1**: User can set injection reminder (recurring, specific time)
- **FR-9.2**: PWA push notifications for reminders (if permission granted)
- **FR-9.3**: In-app badge for missed injections
- **FR-9.4**: Low inventory notifications

---

## 4. Technical Architecture

### 4.1 Tech Stack
- **Frontend**: 
  - HTMX 1.9+ (server-driven interactions)
  - Alpine.js 3.x (minimal client-side reactivity)
  - Pico CSS (classless semantic styling)
  - Chart.js or Apache ECharts (data visualization)
  - Go html/template (server-side rendering)
  - Service Worker for PWA functionality
  
- **Backend**:
  - Go 1.21+
  - Chi router (or Gin)
  - SQLite with WAL mode
  - JWT for authentication (golang-jwt/jwt)
  - SMTP support via gomail or similar
  - go-wkhtmltopdf or similar for PDF generation
  
- **Deployment**:
  - Docker + Docker Compose
  - Nginx reverse proxy
  - Let's Encrypt for SSL

### 4.2 Architecture Diagram
```
┌─────────────────────────────────────────────────┐
│                   Browser/PWA                    │
│  ┌────────────┐  ┌──────────┐  ┌──────────────┐ │
│  │HTMX+Alpine │  │ Service  │  │ IndexedDB    │ │
│  │  (~30kb)   │  │ Worker   │  │ (offline)    │ │
│  └────────────┘  └──────────┘  └──────────────┘ │
└─────────────────────────────────────────────────┘
                      ↓ HTTPS/JWT
┌─────────────────────────────────────────────────┐
│              Docker Container                    │
│  ┌────────────────────────────────────────────┐ │
│  │         Nginx (Reverse Proxy)              │ │
│  └────────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────────┐ │
│  │           Go HTTP Server                   │ │
│  │  ┌──────────────┐  ┌──────────────────┐   │ │
│  │  │ Auth Handler │  │  HTMX Handlers   │   │ │
│  │  │              │  │  (HTML fragments)│   │ │
│  │  └──────────────┘  └──────────────────┘   │ │
│  │  ┌──────────────┐  ┌──────────────────┐   │ │
│  │  │ JWT Middleware│ │  Business Logic  │   │ │
│  │  │              │  │  html/template   │   │ │
│  │  └──────────────┘  └──────────────────┘   │ │
│  └────────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────────┐ │
│  │            SQLite Database                 │ │
│  │              (WAL Mode)                    │ │
│  └────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

### 4.3 Database Schema

```sql
-- Users table
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    email TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP
);

-- Courses table
CREATE TABLE courses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    start_date DATE NOT NULL,
    expected_end_date DATE,
    actual_end_date DATE,
    is_active BOOLEAN DEFAULT 1,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER REFERENCES users(id)
);

-- Injections table
CREATE TABLE injections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id INTEGER NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    administered_by INTEGER REFERENCES users(id),
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    side TEXT NOT NULL CHECK(side IN ('left', 'right')),
    
    -- Advanced mode fields
    site_x REAL,  -- X coordinate on diagram (0-1)
    site_y REAL,  -- Y coordinate on diagram (0-1)
    
    -- Optional details
    pain_level INTEGER CHECK(pain_level BETWEEN 1 AND 10),
    has_knots BOOLEAN DEFAULT 0,
    site_reaction TEXT CHECK(site_reaction IN ('none', 'redness', 'swelling', 'bruising', 'other')),
    notes TEXT,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_injections_course ON injections(course_id);
CREATE INDEX idx_injections_timestamp ON injections(timestamp);

-- Symptom logs table
CREATE TABLE symptom_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id INTEGER NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    logged_by INTEGER REFERENCES users(id),
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    pain_level INTEGER CHECK(pain_level BETWEEN 1 AND 10),
    pain_location TEXT,
    pain_type TEXT,
    symptoms TEXT,  -- JSON array of symptoms
    notes TEXT,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_symptom_logs_course ON symptom_logs(course_id);
CREATE INDEX idx_symptom_logs_timestamp ON symptom_logs(timestamp);

-- Medications table
CREATE TABLE medications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    dosage TEXT,
    frequency TEXT,
    start_date DATE,
    end_date DATE,
    is_active BOOLEAN DEFAULT 1,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Medication logs table
CREATE TABLE medication_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    medication_id INTEGER NOT NULL REFERENCES medications(id) ON DELETE CASCADE,
    logged_by INTEGER REFERENCES users(id),
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    taken BOOLEAN NOT NULL,
    notes TEXT
);

CREATE INDEX idx_medication_logs_med ON medication_logs(medication_id);
CREATE INDEX idx_medication_logs_timestamp ON medication_logs(timestamp);

-- Inventory items table
CREATE TABLE inventory_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_type TEXT NOT NULL CHECK(item_type IN (
        'progesterone', 'draw_needle', 'injection_needle', 
        'syringe', 'swab', 'gauze'
    )),
    quantity REAL NOT NULL,  -- Use REAL for mL tracking
    unit TEXT NOT NULL,  -- 'mL', 'count'
    expiration_date DATE,
    lot_number TEXT,
    low_stock_threshold REAL,
    notes TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Inventory history table
CREATE TABLE inventory_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_type TEXT NOT NULL,
    change_amount REAL NOT NULL,  -- Negative for deduction
    reason TEXT NOT NULL,  -- 'injection', 'manual_adjustment', 'restock'
    reference_id INTEGER,  -- ID of injection if auto-deducted
    performed_by INTEGER REFERENCES users(id),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    notes TEXT
);

CREATE INDEX idx_inventory_history_type ON inventory_history(item_type);
CREATE INDEX idx_inventory_history_timestamp ON inventory_history(timestamp);

-- Settings table
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Notifications table
CREATE TABLE notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    type TEXT NOT NULL,  -- 'injection_reminder', 'low_stock', 'missed_injection'
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    is_read BOOLEAN DEFAULT 0,
    scheduled_time TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_notifications_user ON notifications(user_id);
CREATE INDEX idx_notifications_read ON notifications(is_read);
```

---

## 5. API Design

### 5.1 Authentication Endpoints

```
POST   /api/auth/register
POST   /api/auth/login
POST   /api/auth/logout
POST   /api/auth/refresh
POST   /api/auth/forgot-password
POST   /api/auth/reset-password
GET    /api/auth/me
```

### 5.2 Course Endpoints

```
GET    /api/courses
POST   /api/courses
GET    /api/courses/:id
PUT    /api/courses/:id
DELETE /api/courses/:id
GET    /api/courses/active
POST   /api/courses/:id/activate
POST   /api/courses/:id/close
```

### 5.3 Injection Endpoints

```
GET    /api/injections
POST   /api/injections
GET    /api/injections/:id
PUT    /api/injections/:id
DELETE /api/injections/:id
GET    /api/injections/recent
GET    /api/injections/stats
```

### 5.4 Symptom Endpoints

```
GET    /api/symptoms
POST   /api/symptoms
GET    /api/symptoms/:id
PUT    /api/symptoms/:id
DELETE /api/symptoms/:id
```

### 5.5 Medication Endpoints

```
GET    /api/medications
POST   /api/medications
GET    /api/medications/:id
PUT    /api/medications/:id
DELETE /api/medications/:id
POST   /api/medications/:id/log
GET    /api/medications/:id/logs
```

### 5.6 Inventory Endpoints

```
GET    /api/inventory
PUT    /api/inventory/:itemType
GET    /api/inventory/:itemType/history
POST   /api/inventory/:itemType/adjust
GET    /api/inventory/alerts
```

### 5.7 Export Endpoints

```
GET    /api/export/pdf?start_date=X&end_date=Y&course_id=Z
GET    /api/export/csv?start_date=X&end_date=Y
```

### 5.8 Settings Endpoints

```
GET    /api/settings
PUT    /api/settings
```

---

## 6. Security Considerations

### 6.1 Authentication & Authorization
- **Password Storage**: bcrypt with cost factor 12+
- **JWT**: HS256 signing, 2-week expiry, stored in httpOnly cookie
- **Refresh Tokens**: Separate refresh token with longer expiry, stored securely
- **Rate Limiting**: Login attempts limited (5 per 15 minutes per IP)
- **CSRF Protection**: CSRF tokens for state-changing operations

### 6.2 Data Protection
- **Encryption at Rest**: SQLite database file encrypted (SQLCipher option)
- **Encryption in Transit**: HTTPS only (redirect HTTP to HTTPS)
- **Input Validation**: All user input sanitized and validated
- **SQL Injection Prevention**: Prepared statements only
- **XSS Prevention**: Content Security Policy headers

### 6.3 Access Control
- All API endpoints require valid JWT (except auth endpoints)
- User can only access data within their family account
- Session invalidation on logout

### 6.4 Audit Logging
- All data modifications logged with user ID and timestamp
- Inventory changes tracked in history table
- Failed login attempts logged

### 6.5 Backup & Recovery
- Automated SQLite backups (daily)
- Backup retention policy (30 days)
- Export functionality for user-initiated backups

---

## 7. UI/UX Flow

### 7.1 Home Screen (Mobile-First)
```
┌─────────────────────────────────┐
│  ≡  Injection Tracker     [🔔] │
├─────────────────────────────────┤
│                                 │
│   Active Course: IVF Cycle 1    │
│   Last injection: 18 hours ago  │
│                                 │
│  ┌───────────────────────────┐  │
│  │    LOG INJECTION NOW      │  │
│  │       [Large Button]      │  │
│  └───────────────────────────┘  │
│                                 │
│  Next injection due: 6 hours    │
│                                 │
│  ───────────────────────────    │
│                                 │
│  Quick Actions:                 │
│  • Log Symptoms                 │
│  • Log Medication               │
│  • View Calendar                │
│  • Check Inventory              │
│                                 │
│  ───────────────────────────    │
│                                 │
│  Recent Activity:               │
│  ✓ Injection (Left) - Today     │
│  ✓ Prenatal Vitamin - Today     │
│  ⚠ Low Stock: Draw Needles      │
│                                 │
└─────────────────────────────────┘
```

### 7.2 Quick Log Modal
```
┌─────────────────────────────────┐
│  Log Injection                ✕ │
├─────────────────────────────────┤
│                                 │
│   Which side?                   │
│                                 │
│  ┌──────────┐    ┌──────────┐  │
│  │   LEFT   │    │  RIGHT   │  │
│  │   [L]    │    │   [R]    │  │
│  └──────────┘    └──────────┘  │
│                                 │
│  ▼ More Details (optional)      │
│  │                              │
│  │ Pain Level: ●●●○○○○○○○       │
│  │                              │
│  │ □ Knots/Hardness             │
│  │                              │
│  │ Site Reaction: [None ▼]     │
│  │                              │
│  │ Notes: ___________________   │
│  │                              │
│  └─────────────────────────────│
│                                 │
│         [CANCEL]  [SAVE]        │
└─────────────────────────────────┘
```

### 7.3 Advanced Mode Injection Site
```
┌─────────────────────────────────┐
│  Log Injection Site           ✕ │
├─────────────────────────────────┤
│                                 │
│   Tap location on diagram:      │
│                                 │
│       LEFT          RIGHT       │
│   ┌────────┐    ┌────────┐     │
│   │   🟥   │    │  🟧    │     │
│   │  🟨    │    │    🟩  │     │
│   │        │    │        │     │
│   └────────┘    └────────────┘ │
│                                 │
│   🟥 0-3 days   🟧 4-7 days     │
│   🟨 8-11 days  🟩 12+ days     │
│   ⭐ Recommended site            │
│                                 │
│         [CANCEL]  [NEXT]        │
└─────────────────────────────────┘
```

### 7.4 Navigation Structure
```
Main Navigation (Hamburger Menu):
├── Home
├── Courses
│   ├── Active Course
│   ├── Past Courses
│   └── Create New Course
├── Injections
│   ├── Log Injection
│   ├── Injection History
│   └── Site Map (if advanced mode)
├── Symptoms
│   ├── Log Symptom
│   └── Symptom History
├── Medications
│   ├── Active Medications
│   └── Medication Log
├── Inventory
│   ├── Current Stock
│   ├── Adjust Inventory
│   └── History
├── Calendar
├── Reports & Charts
├── Settings
│   ├── Profile
│   ├── Reminders
│   ├── Advanced Mode Toggle
│   └── Export Data
└── Logout
```

---

## 8. Deployment

### 8.1 Docker Compose Configuration
```yaml
version: '3.8'

services:
  app:
    build: .
    container_name: injection-tracker
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
      - ./backups:/app/backups
    environment:
      - JWT_SECRET=${JWT_SECRET}
      - SMTP_HOST=${SMTP_HOST}
      - SMTP_PORT=${SMTP_PORT}
      - SMTP_USERNAME=${SMTP_USERNAME}
      - SMTP_PASSWORD=${SMTP_PASSWORD}
      - DATABASE_PATH=/app/data/tracker.db
    restart: unless-stopped
    
  nginx:
    image: nginx:alpine
    container_name: injection-tracker-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
      - ./ssl:/etc/nginx/ssl
    depends_on:
      - app
    restart: unless-stopped
```

### 8.2 Environment Variables
```
# Required
JWT_SECRET=<generated-secret>
DATABASE_PATH=/app/data/tracker.db

# Optional (for email)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
SMTP_FROM=your-email@gmail.com

# Optional
SESSION_DURATION=336h  # 2 weeks
BACKUP_ENABLED=true
BACKUP_SCHEDULE=0 2 * * *  # Daily at 2 AM
```

### 8.3 Initial Setup Script
```bash
#!/bin/bash
# setup.sh

# Generate JWT secret
JWT_SECRET=$(openssl rand -base64 32)

# Create directories
mkdir -p data backups ssl

# Create .env file
cat > .env <<EOF
JWT_SECRET=${JWT_SECRET}
DATABASE_PATH=/app/data/tracker.db
EOF

# Build and start
docker-compose up -d

echo "Application started on http://localhost:8080"
echo "JWT_SECRET has been generated"
echo "Please configure SMTP settings in .env if needed"
```

---

## 9. PWA Configuration

### 9.1 manifest.json
```json
{
  "name": "Injection Tracker",
  "short_name": "InjTracker",
  "description": "Track progesterone injections and medications",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#ffffff",
  "theme_color": "#6366f1",
  "orientation": "portrait",
  "icons": [
    {
      "src": "/icons/icon-192.png",
      "sizes": "192x192",
      "type": "image/png"
    },
    {
      "src": "/icons/icon-512.png",
      "sizes": "512x512",
      "type": "image/png"
    }
  ]
}
```

### 9.2 Service Worker Features
- Offline page caching
- API response caching (with expiry)
- Background sync for offline data submission
- Push notification support
- Update notification when new version available

---

## 10. Testing Strategy

### 10.1 Backend Testing
- Unit tests for all business logic (Go testing package)
- Integration tests for API endpoints
- Database migration tests
- JWT authentication/authorization tests
- Inventory auto-deduction logic tests

### 10.2 Frontend Testing
- Component tests (React Testing Library)
- E2E tests for critical flows (Playwright/Cypress)
- PWA functionality tests
- Mobile responsiveness tests

### 10.3 Security Testing
- Penetration testing checklist
- SQL injection prevention validation
- XSS prevention validation
- Authentication bypass attempts
- Rate limiting verification

---

## 11. Future Enhancements (Out of Scope for v1)

### 11.1 Phase 2 Features
- Multi-patient support (tracking multiple people)
- Photo attachments for injection sites
- Integration with calendar apps
- Voice input for quick logging
- Biometric authentication (Face ID / Fingerprint)

### 11.2 Phase 3 Features
- Data analytics and ML predictions
- Shared view with healthcare providers
- Import from other tracking apps
- Apple Health / Google Fit integration
- Wearable device integration for automated symptom detection

---

## 12. Success Metrics

### 12.1 User Experience Metrics
- Time to log injection < 5 seconds (quick mode)
- App load time < 2 seconds
- Offline functionality success rate > 95%
- User error rate < 2%

### 12.2 Technical Metrics
- API response time < 200ms (p95)
- Database query performance < 50ms
- Docker container memory usage < 512MB
- Backup success rate 100%

### 12.3 Adoption Metrics
- Daily active usage during treatment course
- Feature utilization (which features are used most)
- Data export frequency (for medical appointments)

---

## 13. Project Timeline Estimate

### Phase 1: Foundation (2-3 weeks)
- Database schema and migrations
- Authentication system
- Basic API endpoints
- Docker setup

### Phase 2: Core Features (3-4 weeks)
- Quick injection logging
- Inventory management
- Course management
- Basic UI/UX

### Phase 3: Advanced Features (2-3 weeks)
- Advanced injection site tracking
- Symptom and medication logging
- Calendar and charts
- PWA configuration

### Phase 4: Polish & Testing (1-2 weeks)
- UI/UX refinement
- Testing and bug fixes
- Documentation
- Deployment automation

**Total Estimated Time: 8-12 weeks**

---

## 14. Open Questions / Decisions Needed

1. **Database Encryption**: Use SQLCipher for encryption at rest?
2. **Image Storage**: If adding photo attachments later, use filesystem or blob storage?
3. **Backup Strategy**: Automated backup to external location (S3, Backblaze)?
4. **Multi-User Limits**: Maximum number of users per family account?
5. **Data Retention**: Any automatic purging of old data after X years?
6. **Analytics**: Self-hosted analytics (Plausible) or none?
7. **Error Reporting**: Sentry or similar for production error tracking?

---

## Appendix A: Technology Justification

### Why Go?
- Fast compilation and execution
- Excellent standard library for HTTP servers
- Built-in concurrency if needed later
- Easy deployment (single binary)
- Strong typing and error handling

### Why SQLite?
- Zero configuration
- Perfect for single-family use case
- WAL mode supports concurrent reads
- Easy backups (single file)
- Sufficient performance for this use case
- No separate database server needed

### Why JWT?
- Stateless authentication
- Works well with PWA architecture
- Suitable for self-hosted deployment
- Simple to implement and validate

### Why PWA over Native App?
- Single codebase for all platforms
- No app store approval process
- Easier deployment and updates
- Sufficient for family use case
- Can be packaged as native app later if needed