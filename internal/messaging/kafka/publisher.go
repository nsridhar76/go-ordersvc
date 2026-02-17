package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nsridhar76/go-ordersvc/internal/domain"
	"github.com/nsridhar76/go-ordersvc/internal/messaging"
	"github.com/segmentio/kafka-go"
)

// Publisher implements service.EventPublisher using Kafka.
type Publisher struct {
	writer *kafka.Writer
	topic  string
}

// NewPublisher creates a Kafka event publisher.
func NewPublisher(brokers []string, topic string) *Publisher {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
	}
	return &Publisher{writer: w, topic: topic}
}

func (p *Publisher) PublishOrderCreated(ctx context.Context, order *domain.Order) error {
	evt := messaging.OrderEvent{
		EventType:  messaging.EventOrderCreated,
		OrderID:    order.ID.String(),
		CustomerID: order.CustomerID,
		Status:     string(order.Status),
		Total:      order.Total,
		Version:    order.Version,
		OccurredAt: time.Now(),
	}
	return p.publish(ctx, order.ID.String(), evt)
}

func (p *Publisher) PublishOrderUpdated(ctx context.Context, order *domain.Order) error {
	evt := messaging.OrderEvent{
		EventType:  messaging.EventOrderUpdated,
		OrderID:    order.ID.String(),
		CustomerID: order.CustomerID,
		Status:     string(order.Status),
		Total:      order.Total,
		Version:    order.Version,
		OccurredAt: time.Now(),
	}
	return p.publish(ctx, order.ID.String(), evt)
}

func (p *Publisher) PublishOrderStatusChanged(ctx context.Context, order *domain.Order, oldStatus, newStatus domain.OrderStatus) error {
	evt := messaging.OrderEvent{
		EventType:  messaging.EventOrderStatusChanged,
		OrderID:    order.ID.String(),
		CustomerID: order.CustomerID,
		Status:     string(order.Status),
		OldStatus:  string(oldStatus),
		NewStatus:  string(newStatus),
		Total:      order.Total,
		Version:    order.Version,
		OccurredAt: time.Now(),
	}
	return p.publish(ctx, order.ID.String(), evt)
}

// Close flushes and closes the underlying Kafka writer.
func (p *Publisher) Close() error {
	return p.writer.Close()
}

func (p *Publisher) publish(ctx context.Context, key string, evt messaging.OrderEvent) error {
	value, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: value,
	})
}
