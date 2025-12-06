package models

import (
	"database/sql"
	"time"
)

// User represents a system user
type User struct {
	ID                  int64
	Username            string
	PasswordHash        string
	Email               sql.NullString
	AccountID           int64
	Role                string // "owner" or "member"
	IsActive            bool
	FailedLoginAttempts int
	LockedUntil         sql.NullTime
	CreatedAt           time.Time
	LastLogin           sql.NullTime
}

// Course represents a treatment cycle
type Course struct {
	ID              int64
	Name            string
	StartDate       time.Time
	ExpectedEndDate sql.NullTime
	ActualEndDate   sql.NullTime
	IsActive        bool
	Notes           sql.NullString
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CreatedBy       sql.NullInt64
	AccountID       int64 // Account this course belongs to

	// Computed fields (set by repository)
	InjectionCount int
	DurationDays   int
	AveragePerWeek float64
}

// FormattedStartDate returns the start date in a readable format
func (c *Course) FormattedStartDate() string {
	return c.StartDate.Format("Jan 2, 2006")
}

// FormattedEndDate returns the end date in a readable format
func (c *Course) FormattedEndDate() string {
	if c.ActualEndDate.Valid {
		return c.ActualEndDate.Time.Format("Jan 2, 2006")
	}
	return "Ongoing"
}

// FormattedExpectedEndDate returns the expected end date in a readable format
func (c *Course) FormattedExpectedEndDate() string {
	if c.ExpectedEndDate.Valid {
		return c.ExpectedEndDate.Time.Format("Jan 2, 2006")
	}
	return "Not set"
}

// DaysActive returns the number of days the course has been active
func (c *Course) DaysActive() int {
	endDate := time.Now()
	if c.ActualEndDate.Valid {
		endDate = c.ActualEndDate.Time
	}
	return int(endDate.Sub(c.StartDate).Hours() / 24)
}

// Injection represents an injection record
type Injection struct {
	ID             int64
	CourseID       int64
	AdministeredBy sql.NullInt64
	Timestamp      time.Time
	Side           string
	SiteX          sql.NullFloat64
	SiteY          sql.NullFloat64
	PainLevel      sql.NullInt64
	HasKnots       bool
	SiteReaction   sql.NullString
	Notes          sql.NullString
	AccountID      int64 // Account this injection belongs to
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// DateStr returns the date part of the timestamp for HTML date inputs
func (i *Injection) DateStr() string {
	return i.Timestamp.Format("2006-01-02")
}

// TimeStr returns the time part of the timestamp for HTML time inputs
func (i *Injection) TimeStr() string {
	return i.Timestamp.Format("15:04")
}

// SymptomLog represents a symptom log entry
type SymptomLog struct {
	ID           int64
	CourseID     int64
	LoggedBy     sql.NullInt64
	Timestamp    time.Time
	PainLevel    sql.NullInt64
	PainLocation sql.NullString
	PainType     sql.NullString
	Symptoms     sql.NullString // JSON array
	Notes        sql.NullString
	AccountID    int64 // Account this symptom log belongs to
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Medication represents a medication
type Medication struct {
	ID                int64
	Name              string
	Dosage            sql.NullString
	Frequency         sql.NullString
	StartDate         sql.NullTime
	EndDate           sql.NullTime
	IsActive          bool
	Notes             sql.NullString
	ScheduledTime     sql.NullString // HH:MM format (e.g., "08:00")
	TimeWindowMinutes sql.NullInt64  // Minutes before/after scheduled time
	ReminderEnabled   bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	AccountID         int64 // Account this medication belongs to

	// Computed fields (set by repository)
	TakenToday bool
}

// FormattedEndDate returns the end date in a readable format
func (m *Medication) FormattedEndDate() string {
	if m.EndDate.Valid {
		return m.EndDate.Time.Format("Jan 2, 2006")
	}
	return "Ongoing"
}

// MedicationLog represents a medication log entry
type MedicationLog struct {
	ID           int64
	MedicationID int64
	LoggedBy     sql.NullInt64
	Timestamp    time.Time
	Taken        bool
	Notes        sql.NullString
	CreatedAt    time.Time
}

// InventoryItem represents an inventory item
type InventoryItem struct {
	ID                int64
	ItemType          string
	Quantity          float64
	Unit              string
	ExpirationDate    sql.NullTime
	LotNumber         sql.NullString
	LowStockThreshold sql.NullFloat64
	Notes             sql.NullString
	CreatedAt         time.Time
	UpdatedAt         time.Time
	AccountID         int64 // Account this inventory belongs to
}

// InventoryHistory represents an inventory change record
type InventoryHistory struct {
	ID             int64
	ItemType       string
	ChangeAmount   float64
	QuantityBefore float64
	QuantityAfter  float64
	Reason         string
	ReferenceID    sql.NullInt64
	ReferenceType  sql.NullString
	PerformedBy    sql.NullInt64
	Timestamp      time.Time
	Notes          sql.NullString
}

// Notification represents a user notification
type Notification struct {
	ID            int64
	UserID        sql.NullInt64
	Type          string
	Title         string
	Message       string
	IsRead        bool
	ScheduledTime sql.NullTime
	CreatedAt     time.Time
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID         int64
	UserID     sql.NullInt64
	Action     string
	EntityType string
	EntityID   sql.NullInt64
	Details    sql.NullString
	IPAddress  sql.NullString
	UserAgent  sql.NullString
	Timestamp  time.Time
}

// Setting represents a system setting
type Setting struct {
	Key       string
	Value     string
	UpdatedAt time.Time
	UpdatedBy sql.NullInt64
}

// Account represents a family/couple account (multi-user support)
type Account struct {
	ID        int64
	Name      sql.NullString
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AccountMember represents a user's membership in an account
type AccountMember struct {
	AccountID int64
	UserID    int64
	Role      string // 'owner' or 'member'
	JoinedAt  time.Time
	InvitedBy sql.NullInt64

	// Computed fields (set by repository)
	Username string // Username of this member
}

// AccountInvitation represents an invitation to join an account
type AccountInvitation struct {
	ID         int64
	AccountID  int64
	Email      string
	TokenHash  string
	InvitedBy  int64
	Role       string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	AcceptedAt sql.NullTime
	AcceptedBy sql.NullInt64

	// Computed fields (set by repository)
	InviterUsername string
	IsExpired       bool
}

// IsExpiredCheck checks if the invitation has expired
func (i *AccountInvitation) IsExpiredCheck() bool {
	return time.Now().After(i.ExpiresAt)
}
