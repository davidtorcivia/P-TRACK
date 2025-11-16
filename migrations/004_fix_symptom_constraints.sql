-- Fix symptom_logs constraints by recreating table
-- This migration properly removes CHECK constraints to allow custom locations

-- Step 1: Drop the old table
DROP TABLE IF EXISTS symptom_logs;

-- Step 2: Create new table without location/type constraints
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
);

-- Step 3: Create indexes
CREATE INDEX idx_symptom_logs_course ON symptom_logs(course_id);
CREATE INDEX idx_symptom_logs_timestamp ON symptom_logs(timestamp DESC);
CREATE INDEX idx_symptom_logs_course_timestamp ON symptom_logs(course_id, timestamp DESC);

-- Step 4: Create trigger
CREATE TRIGGER update_symptom_logs_timestamp
AFTER UPDATE ON symptom_logs
BEGIN
    UPDATE symptom_logs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
