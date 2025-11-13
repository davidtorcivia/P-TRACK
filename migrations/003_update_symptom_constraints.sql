-- Add new columns and update constraints for symptom logs
-- This migration removes restrictive CHECK constraints to allow custom locations

-- SQLite doesn't support DROP CONSTRAINT, so we need to recreate the table

-- Step 1: Create new table with updated schema (no CHECK constraints)
CREATE TABLE IF NOT EXISTS symptom_logs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id INTEGER NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    logged_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    pain_level INTEGER CHECK(pain_level IS NULL OR (pain_level BETWEEN 1 AND 10)),
    pain_location TEXT,
    pain_type TEXT,
    has_knots BOOLEAN DEFAULT 0,
    symptoms TEXT,  -- JSON array of symptoms
    dissipated_at TIMESTAMP,  -- When symptoms went away
    notes TEXT,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    -- Removed pain_location CHECK constraint to allow custom locations
    -- Removed pain_type CHECK constraint to allow custom types
);

-- Step 2: Copy existing data
INSERT INTO symptom_logs_new (
    id, course_id, logged_by, timestamp,
    pain_level, pain_location, pain_type, symptoms, notes,
    created_at, updated_at
)
SELECT
    id, course_id, logged_by, timestamp,
    pain_level,
    CASE
        WHEN pain_location = 'injection_site' THEN 'injection_site_left'
        ELSE pain_location
    END,
    pain_type, symptoms, notes,
    created_at, updated_at
FROM symptom_logs;

-- Step 3: Drop old table
DROP TABLE symptom_logs;

-- Step 4: Rename new table
ALTER TABLE symptom_logs_new RENAME TO symptom_logs;

-- Step 5: Recreate indexes
CREATE INDEX idx_symptom_logs_course ON symptom_logs(course_id);
CREATE INDEX idx_symptom_logs_timestamp ON symptom_logs(timestamp DESC);
CREATE INDEX idx_symptom_logs_course_timestamp ON symptom_logs(course_id, timestamp DESC);

-- Step 6: Recreate trigger
CREATE TRIGGER update_symptom_logs_timestamp
AFTER UPDATE ON symptom_logs
BEGIN
    UPDATE symptom_logs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
