-- Enable foreign key constraints
PRAGMA foreign_keys = ON;

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    email TEXT COLLATE NOCASE,
    is_active BOOLEAN DEFAULT 1,
    failed_login_attempts INTEGER DEFAULT 0,
    locked_until TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP,
    CONSTRAINT chk_username_length CHECK (length(username) >= 3 AND length(username) <= 50),
    CONSTRAINT chk_email_format CHECK (email IS NULL OR email LIKE '%@%.%')
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);

-- Courses table
CREATE TABLE IF NOT EXISTS courses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    start_date DATE NOT NULL,
    expected_end_date DATE,
    actual_end_date DATE,
    is_active BOOLEAN DEFAULT 1,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT chk_dates CHECK (
        expected_end_date IS NULL OR expected_end_date >= start_date
    ),
    CONSTRAINT chk_actual_end CHECK (
        actual_end_date IS NULL OR actual_end_date >= start_date
    ),
    CONSTRAINT chk_name_length CHECK (length(trim(name)) > 0)
);

CREATE INDEX idx_courses_active ON courses(is_active);
CREATE INDEX idx_courses_created_by ON courses(created_by);

-- Injections table
CREATE TABLE IF NOT EXISTS injections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id INTEGER NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    administered_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    side TEXT NOT NULL CHECK(side IN ('left', 'right')),

    -- Advanced mode fields
    site_x REAL CHECK(site_x IS NULL OR (site_x >= 0 AND site_x <= 1)),
    site_y REAL CHECK(site_y IS NULL OR (site_y >= 0 AND site_y <= 1)),

    -- Optional details
    pain_level INTEGER CHECK(pain_level IS NULL OR (pain_level BETWEEN 1 AND 10)),
    has_knots BOOLEAN DEFAULT 0,
    site_reaction TEXT CHECK(site_reaction IS NULL OR site_reaction IN ('none', 'redness', 'swelling', 'bruising', 'other')),
    notes TEXT,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_injections_course ON injections(course_id);
CREATE INDEX idx_injections_timestamp ON injections(timestamp DESC);
CREATE INDEX idx_injections_side ON injections(side);
CREATE INDEX idx_injections_course_timestamp ON injections(course_id, timestamp DESC);

-- Symptom logs table
CREATE TABLE IF NOT EXISTS symptom_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id INTEGER NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    logged_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    pain_level INTEGER CHECK(pain_level IS NULL OR (pain_level BETWEEN 1 AND 10)),
    pain_location TEXT,
    pain_type TEXT,
    symptoms TEXT,  -- JSON array of symptoms
    notes TEXT,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_pain_location CHECK (
        pain_location IS NULL OR pain_location IN ('injection_site', 'abdomen', 'back', 'other')
    ),
    CONSTRAINT chk_pain_type CHECK (
        pain_type IS NULL OR pain_type IN ('sharp', 'dull', 'aching', 'cramping', 'other')
    )
);

CREATE INDEX idx_symptom_logs_course ON symptom_logs(course_id);
CREATE INDEX idx_symptom_logs_timestamp ON symptom_logs(timestamp DESC);
CREATE INDEX idx_symptom_logs_course_timestamp ON symptom_logs(course_id, timestamp DESC);

-- Medications table
CREATE TABLE IF NOT EXISTS medications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    dosage TEXT,
    frequency TEXT,
    start_date DATE,
    end_date DATE,
    is_active BOOLEAN DEFAULT 1,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_medication_name CHECK (length(trim(name)) > 0),
    CONSTRAINT chk_medication_dates CHECK (
        end_date IS NULL OR start_date IS NULL OR end_date >= start_date
    )
);

CREATE INDEX idx_medications_active ON medications(is_active);
CREATE INDEX idx_medications_name ON medications(name);

-- Medication logs table
CREATE TABLE IF NOT EXISTS medication_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    medication_id INTEGER NOT NULL REFERENCES medications(id) ON DELETE CASCADE,
    logged_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    taken BOOLEAN NOT NULL,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_medication_logs_med ON medication_logs(medication_id);
CREATE INDEX idx_medication_logs_timestamp ON medication_logs(timestamp DESC);
CREATE INDEX idx_medication_logs_med_timestamp ON medication_logs(medication_id, timestamp DESC);

-- Inventory items table
CREATE TABLE IF NOT EXISTS inventory_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_type TEXT NOT NULL CHECK(item_type IN (
        'progesterone', 'draw_needle', 'injection_needle',
        'syringe', 'swab', 'gauze'
    )),
    quantity REAL NOT NULL CHECK(quantity >= 0),
    unit TEXT NOT NULL CHECK(unit IN ('mL', 'count')),
    expiration_date DATE,
    lot_number TEXT,
    low_stock_threshold REAL CHECK(low_stock_threshold IS NULL OR low_stock_threshold >= 0),
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_inventory_item_type UNIQUE(item_type)
);

CREATE INDEX idx_inventory_type ON inventory_items(item_type);
CREATE INDEX idx_inventory_expiration ON inventory_items(expiration_date);

-- Inventory history table
CREATE TABLE IF NOT EXISTS inventory_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_type TEXT NOT NULL,
    change_amount REAL NOT NULL,
    quantity_before REAL NOT NULL,
    quantity_after REAL NOT NULL,
    reason TEXT NOT NULL CHECK(reason IN ('injection', 'manual_adjustment', 'restock', 'expired', 'other')),
    reference_id INTEGER,  -- ID of injection if auto-deducted
    reference_type TEXT,   -- 'injection', 'symptom', etc.
    performed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    notes TEXT
);

CREATE INDEX idx_inventory_history_type ON inventory_history(item_type);
CREATE INDEX idx_inventory_history_timestamp ON inventory_history(timestamp DESC);
CREATE INDEX idx_inventory_history_reference ON inventory_history(reference_type, reference_id);

-- Settings table
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL
);

-- Notifications table
CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK(type IN ('injection_reminder', 'low_stock', 'missed_injection', 'expiration_warning', 'system')),
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    is_read BOOLEAN DEFAULT 0,
    scheduled_time TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_notifications_user ON notifications(user_id);
CREATE INDEX idx_notifications_read ON notifications(is_read);
CREATE INDEX idx_notifications_scheduled ON notifications(scheduled_time);

-- Audit log table for security and compliance
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id INTEGER,
    details TEXT,  -- JSON with additional details
    ip_address TEXT,
    user_agent TEXT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_logs_user ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);

-- Session tokens table for refresh tokens
CREATE TABLE IF NOT EXISTS session_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP,
    ip_address TEXT,
    user_agent TEXT,
    is_revoked BOOLEAN DEFAULT 0
);

CREATE INDEX idx_session_tokens_user ON session_tokens(user_id);
CREATE INDEX idx_session_tokens_hash ON session_tokens(token_hash);
CREATE INDEX idx_session_tokens_expires ON session_tokens(expires_at);

-- Password reset tokens table
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    used_at TIMESTAMP
);

CREATE INDEX idx_reset_tokens_hash ON password_reset_tokens(token_hash);
CREATE INDEX idx_reset_tokens_user ON password_reset_tokens(user_id);

-- Insert default settings
INSERT OR IGNORE INTO settings (key, value) VALUES
    ('advanced_mode_enabled', 'false'),
    ('heat_map_days', '14'),
    ('allow_registration', 'true'),
    ('require_email_verification', 'false'),
    ('injection_reminder_enabled', 'false'),
    ('low_stock_alert_enabled', 'true');

-- Create triggers for updated_at timestamps
CREATE TRIGGER IF NOT EXISTS update_courses_timestamp
AFTER UPDATE ON courses
BEGIN
    UPDATE courses SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_injections_timestamp
AFTER UPDATE ON injections
BEGIN
    UPDATE injections SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_symptom_logs_timestamp
AFTER UPDATE ON symptom_logs
BEGIN
    UPDATE symptom_logs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_medications_timestamp
AFTER UPDATE ON medications
BEGIN
    UPDATE medications SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_inventory_items_timestamp
AFTER UPDATE ON inventory_items
BEGIN
    UPDATE inventory_items SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;