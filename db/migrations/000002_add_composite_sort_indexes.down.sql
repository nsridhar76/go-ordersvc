-- Restore original single/two-column indexes
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_orders_customer_status ON orders(customer_id, status) WHERE deleted_at IS NULL;

-- Drop composite sort indexes
DROP INDEX IF EXISTS idx_orders_status_created;
DROP INDEX IF EXISTS idx_orders_customer_created;
DROP INDEX IF EXISTS idx_orders_customer_status_created;
