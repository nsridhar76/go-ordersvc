package noop

import (
	"context"

	"github.com/nsridhar76/go-ordersvc/internal/domain"
)

// Publisher is a no-op EventPublisher used when Kafka is not configured.
type Publisher struct{}

func (Publisher) PublishOrderCreated(_ context.Context, _ *domain.Order) error { return nil }

func (Publisher) PublishOrderUpdated(_ context.Context, _ *domain.Order) error { return nil }

func (Publisher) PublishOrderStatusChanged(_ context.Context, _ *domain.Order, _, _ domain.OrderStatus) error {
	return nil
}
