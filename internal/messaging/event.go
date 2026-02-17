// Package messaging defines event types for order domain events.
package messaging

import "time"

// Event type constants for order domain events.
const (
	EventOrderCreated       = "order.created"
	EventOrderUpdated       = "order.updated"
	EventOrderStatusChanged = "order.status_changed"
)

// OrderEvent is the Kafka message envelope for order domain events.
type OrderEvent struct {
	EventType  string    `json:"event_type"`
	OrderID    string    `json:"order_id"`
	CustomerID string    `json:"customer_id"`
	Status     string    `json:"status"`
	OldStatus  string    `json:"old_status,omitempty"`
	NewStatus  string    `json:"new_status,omitempty"`
	Total      float64   `json:"total"`
	Version    int       `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
}
