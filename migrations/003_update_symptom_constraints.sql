-- Add new columns and update constraints for symptom logs
-- This migration removes restrictive CHECK constraints to allow custom locations

-- SQLite doesn't support DROP CONSTRAINT, so we need to recreate the table

-- Step 1: Drop the old symptom_logs table if it exists
DROP TABLE IF EXISTS symptom_logs;

-- Step 2: Create new table with updated schema (no CHECK constraints on location/type)
CREATE TABLE symptom_logs (
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

-- Step 3: Recreate indexes
CREATE INDEX idx_symptom_logs_course ON symptom_logs(course_id);
CREATE INDEX idx_symptom_logs_timestamp ON symptom_logs(timestamp DESC);
CREATE INDEX idx_symptom_logs_course_timestamp ON symptom_logs(course_id, timestamp DESC);

-- Step 4: Recreate trigger
CREATE TRIGGER update_symptom_logs_timestamp
AFTER UPDATE ON symptom_logs
BEGIN
    UPDATE symptom_logs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
