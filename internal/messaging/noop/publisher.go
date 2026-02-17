// Package noop provides a no-op EventPublisher for use when Kafka is not configured.
package noop

import (
	"context"

	"github.com/nsridhar76/go-ordersvc/internal/domain"
)

// Publisher is a no-op EventPublisher used when Kafka is not configured.
type Publisher struct{}

// PublishOrderCreated is a no-op.
func (Publisher) PublishOrderCreated(_ context.Context, _ *domain.Order) error { return nil }

// PublishOrderUpdated is a no-op.
func (Publisher) PublishOrderUpdated(_ context.Context, _ *domain.Order) error { return nil }

// PublishOrderStatusChanged is a no-op.
func (Publisher) PublishOrderStatusChanged(_ context.Context, _ *domain.Order, _, _ domain.OrderStatus) error {
	return nil
}
