-- Add time window columns to medications table
ALTER TABLE medications ADD COLUMN scheduled_time TEXT; -- Format: "HH:MM" (e.g., "08:00")
ALTER TABLE medications ADD COLUMN time_window_minutes INTEGER DEFAULT 60; -- Default 1-hour window
ALTER TABLE medications ADD COLUMN reminder_enabled BOOLEAN DEFAULT 0;

-- Comments:
-- scheduled_time: The ideal time to take the medication (e.g., "08:00" for 8:00 AM)
-- time_window_minutes: How many minutes before/after scheduled_time is acceptable
-- reminder_enabled: Whether to show reminders for this medication
