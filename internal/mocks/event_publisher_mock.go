package mocks

import (
	"context"

	"github.com/nsridhar76/go-ordersvc/internal/domain"
)

// EventPublisherMock is a mock implementation of EventPublisher
type EventPublisherMock struct {
	PublishOrderCreatedFunc       func(ctx context.Context, order *domain.Order) error
	PublishOrderUpdatedFunc       func(ctx context.Context, order *domain.Order) error
	PublishOrderStatusChangedFunc func(ctx context.Context, order *domain.Order, oldStatus, newStatus domain.OrderStatus) error
}

// PublishOrderCreated delegates to PublishOrderCreatedFunc if set.
func (m *EventPublisherMock) PublishOrderCreated(ctx context.Context, order *domain.Order) error {
	if m.PublishOrderCreatedFunc != nil {
		return m.PublishOrderCreatedFunc(ctx, order)
	}
	return nil
}

// PublishOrderUpdated delegates to PublishOrderUpdatedFunc if set.
func (m *EventPublisherMock) PublishOrderUpdated(ctx context.Context, order *domain.Order) error {
	if m.PublishOrderUpdatedFunc != nil {
		return m.PublishOrderUpdatedFunc(ctx, order)
	}
	return nil
}

// PublishOrderStatusChanged delegates to PublishOrderStatusChangedFunc if set.
func (m *EventPublisherMock) PublishOrderStatusChanged(ctx context.Context, order *domain.Order, oldStatus, newStatus domain.OrderStatus) error {
	if m.PublishOrderStatusChangedFunc != nil {
		return m.PublishOrderStatusChangedFunc(ctx, order, oldStatus, newStatus)
	}
	return nil
}
