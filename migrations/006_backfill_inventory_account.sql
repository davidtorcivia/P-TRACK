-- Migration 006: backfill account scoping for inventory data.
--
-- Earlier builds created inventory_items without an account_id and never
-- recorded account ownership on inventory_history. This migration repairs that
-- so per-account scoping in the handlers/repositories matches the data.
--
-- It is safe and idempotent: it only touches rows whose account_id is NULL and
-- only adds the inventory_history.account_id column once (the migration runner
-- records applied migrations, so this file runs exactly once per database).

-- 1) Assign orphaned inventory items to the oldest existing account.
--    (Single-family deployments have exactly one account; this maps the
--     pre-existing inventory to it. If no account exists yet, this is a no-op.)
UPDATE inventory_items
SET account_id = (SELECT id FROM accounts ORDER BY id LIMIT 1)
WHERE account_id IS NULL
  AND EXISTS (SELECT 1 FROM accounts);

-- 2) Add account_id to inventory_history for precise per-account scoping.
ALTER TABLE inventory_history ADD COLUMN account_id INTEGER REFERENCES accounts(id) ON DELETE CASCADE;

-- 3) Backfill history rows from their matching inventory item (by item_type).
UPDATE inventory_history
SET account_id = (
    SELECT ii.account_id
    FROM inventory_items ii
    WHERE ii.item_type = inventory_history.item_type
    ORDER BY ii.account_id
    LIMIT 1
)
WHERE account_id IS NULL;

-- 4) Index for the new scoping column.
CREATE INDEX IF NOT EXISTS idx_inventory_history_account ON inventory_history(account_id);
