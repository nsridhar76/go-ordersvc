-- Add composite indexes that cover both WHERE and ORDER BY created_at DESC
-- to eliminate filesort on paginated list queries (ADR-0002).

-- Covers: WHERE status = $1 AND deleted_at IS NULL ORDER BY created_at DESC
-- Replaces: idx_orders_status (status-only)
CREATE INDEX IF NOT EXISTS idx_orders_status_created ON orders(status, created_at DESC) WHERE deleted_at IS NULL;

-- Covers: WHERE customer_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC
-- Replaces: idx_orders_customer_id (customer-only)
CREATE INDEX IF NOT EXISTS idx_orders_customer_created ON orders(customer_id, created_at DESC) WHERE deleted_at IS NULL;

-- Covers: WHERE customer_id = $1 AND status = $2 AND deleted_at IS NULL ORDER BY created_at DESC
-- Replaces: idx_orders_customer_status (customer+status without sort)
CREATE INDEX IF NOT EXISTS idx_orders_customer_status_created ON orders(customer_id, status, created_at DESC) WHERE deleted_at IS NULL;

-- Drop redundant indexes (subsets of the new composites)
DROP INDEX IF EXISTS idx_orders_status;
DROP INDEX IF EXISTS idx_orders_customer_id;
DROP INDEX IF EXISTS idx_orders_customer_status;
