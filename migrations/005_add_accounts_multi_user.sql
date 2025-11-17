-- ============================================
-- MIGRATION 005: MULTI-USER ACCOUNTS SUPPORT
-- ============================================
-- This migration adds support for couples/family accounts
-- where multiple users (husband/wife) can share the same data.
--
-- Key Changes:
-- 1. Creates accounts table (family/couple grouping)
-- 2. Creates account_members join table (users ↔ accounts)
-- 3. Creates account_invitations for onboarding second user
-- 4. Adds account_id to all data tables
-- 5. Migrates existing single users to have their own accounts
-- ============================================

-- Enable foreign key constraints
PRAGMA foreign_keys = ON;

-- ============================================
-- STEP 1: CREATE NEW TABLES
-- ============================================

-- Accounts table (represents a family/couple)
CREATE TABLE IF NOT EXISTS accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT,  -- Optional: "Smith Family", "John & Jane"
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Account members (join table: users ↔ accounts)
-- This allows multiple users to belong to one account
CREATE TABLE IF NOT EXISTS account_members (
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('owner', 'member')),
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    invited_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (account_id, user_id),
    CONSTRAINT chk_unique_user UNIQUE(user_id)  -- User can only be in ONE account
);

CREATE INDEX idx_account_members_user ON account_members(user_id);
CREATE INDEX idx_account_members_account ON account_members(account_id);

-- Account invitations (for onboarding second user)
CREATE TABLE IF NOT EXISTS account_invitations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    email TEXT NOT NULL COLLATE NOCASE,
    token_hash TEXT UNIQUE NOT NULL,
    invited_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('owner', 'member')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    accepted_at TIMESTAMP,
    accepted_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT chk_not_expired CHECK (accepted_at IS NULL OR accepted_at <= expires_at)
);

CREATE INDEX idx_invitations_token ON account_invitations(token_hash);
CREATE INDEX idx_invitations_account ON account_invitations(account_id);
CREATE INDEX idx_invitations_email ON account_invitations(email);
CREATE INDEX idx_invitations_expires ON account_invitations(expires_at);

-- ============================================
-- STEP 2: ADD account_id TO EXISTING TABLES
-- ============================================
-- SQLite doesn't support adding NOT NULL columns with foreign keys directly,
-- so we add them as nullable first, then backfill, then add constraints

-- Add account_id to courses
ALTER TABLE courses ADD COLUMN account_id INTEGER REFERENCES accounts(id) ON DELETE CASCADE;

-- Add account_id to medications
ALTER TABLE medications ADD COLUMN account_id INTEGER REFERENCES accounts(id) ON DELETE CASCADE;

-- Add account_id to inventory_items
ALTER TABLE inventory_items ADD COLUMN account_id INTEGER REFERENCES accounts(id) ON DELETE CASCADE;

-- Note: injections and symptom_logs inherit account via course.account_id (JOIN)
-- Note: medication_logs inherit account via medications.account_id (JOIN)

-- ============================================
-- STEP 3: CREATE INDEXES FOR PERFORMANCE
-- ============================================

CREATE INDEX idx_courses_account ON courses(account_id);
CREATE INDEX idx_courses_account_active ON courses(account_id, is_active);

CREATE INDEX idx_medications_account ON medications(account_id);
CREATE INDEX idx_medications_account_active ON medications(account_id, is_active);

CREATE INDEX idx_inventory_items_account ON inventory_items(account_id);

-- Create composite UNIQUE index for inventory items (each account has their own progesterone, needles, etc.)
CREATE UNIQUE INDEX idx_inventory_items_type_account ON inventory_items(item_type, account_id);

-- ============================================
-- STEP 4: MIGRATE EXISTING DATA
-- ============================================
-- For each existing user, create an account and link them as owner

-- Create an account for each existing user
INSERT INTO accounts (created_at)
SELECT created_at FROM users WHERE id NOT IN (SELECT user_id FROM account_members);

-- Link each user to their new account as owner
-- We'll match user.id with the corresponding account.id created above
INSERT INTO account_members (account_id, user_id, role, joined_at)
SELECT
    (SELECT id FROM accounts WHERE accounts.created_at = users.created_at LIMIT 1),
    users.id,
    'owner',
    users.created_at
FROM users
WHERE users.id NOT IN (SELECT user_id FROM account_members);

-- Update courses to belong to the user's account
UPDATE courses
SET account_id = (
    SELECT account_id
    FROM account_members
    WHERE user_id = courses.created_by
    LIMIT 1
)
WHERE account_id IS NULL AND created_by IS NOT NULL;

-- For courses without created_by, assign to first account (fallback)
UPDATE courses
SET account_id = (SELECT MIN(id) FROM accounts)
WHERE account_id IS NULL;

-- Update medications to belong to the first user who logged it
UPDATE medications
SET account_id = (
    SELECT am.account_id
    FROM medication_logs ml
    JOIN account_members am ON am.user_id = ml.logged_by
    WHERE ml.medication_id = medications.id
    ORDER BY ml.timestamp ASC
    LIMIT 1
)
WHERE account_id IS NULL;

-- For medications with no logs, assign to first account
UPDATE medications
SET account_id = (SELECT MIN(id) FROM accounts)
WHERE account_id IS NULL;

-- Update inventory_items to belong to accounts
-- Strategy: Assign inventory based on who first used it (from inventory_history)
UPDATE inventory_items
SET account_id = (
    SELECT am.account_id
    FROM inventory_history ih
    JOIN account_members am ON am.user_id = ih.performed_by
    WHERE ih.item_type = inventory_items.item_type
    ORDER BY ih.timestamp ASC
    LIMIT 1
)
WHERE account_id IS NULL;

-- For inventory with no history, assign to first account
UPDATE inventory_items
SET account_id = (SELECT MIN(id) FROM accounts)
WHERE account_id IS NULL;

-- ============================================
-- STEP 5: CREATE TRIGGERS
-- ============================================

-- Trigger for accounts.updated_at
CREATE TRIGGER IF NOT EXISTS update_accounts_timestamp
AFTER UPDATE ON accounts
BEGIN
    UPDATE accounts SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- ============================================
-- STEP 6: VALIDATION
-- ============================================
-- Verify migration succeeded

-- All users should have an account
-- SELECT COUNT(*) FROM users WHERE id NOT IN (SELECT user_id FROM account_members);
-- Expected: 0

-- All courses should have an account_id
-- SELECT COUNT(*) FROM courses WHERE account_id IS NULL;
-- Expected: 0

-- All medications should have an account_id
-- SELECT COUNT(*) FROM medications WHERE account_id IS NULL;
-- Expected: 0

-- All inventory_items should have an account_id
-- SELECT COUNT(*) FROM inventory_items WHERE account_id IS NULL;
-- Expected: 0
