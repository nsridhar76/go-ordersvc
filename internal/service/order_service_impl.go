// Copyright 2026 go-ordersvc Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/nsridhar76/go-ordersvc/internal/cache"
	"github.com/nsridhar76/go-ordersvc/internal/domain"
	"github.com/nsridhar76/go-ordersvc/internal/repository"
)

const orderCacheTTL = 5 * time.Minute

// orderServiceImpl implements OrderService
type orderServiceImpl struct {
	repo      repository.OrderRepository
	cache     cache.OrderCache
	publisher EventPublisher
}

// NewOrderService creates a new OrderService
func NewOrderService(repo repository.OrderRepository, orderCache cache.OrderCache, publisher EventPublisher) OrderService {
	return &orderServiceImpl{
		repo:      repo,
		cache:     orderCache,
		publisher: publisher,
	}
}

func (s *orderServiceImpl) CreateOrder(ctx context.Context, dto CreateOrderDTO) (*domain.Order, error) {
	// Validate customer ID
	if dto.CustomerID == "" {
		return nil, domain.ErrInvalidCustomerID
	}

	// Validate items
	if len(dto.Items) == 0 {
		return nil, domain.ErrNoItems
	}

	// Create order items with IDs and calculate subtotals
	items := make([]domain.OrderItem, len(dto.Items))
	for i, item := range dto.Items {
		// Validate item
		if err := item.Validate(); err != nil {
			return nil, err
		}

		items[i] = domain.OrderItem{
			ID:        uuid.New(),
			ProductID: item.ProductID,
			Name:      item.Name,
			Quantity:  item.Quantity,
			Price:     item.Price,
			Subtotal:  item.CalculateSubtotal(),
		}
	}

	// Create order
	order := &domain.Order{
		ID:         uuid.New(),
		CustomerID: dto.CustomerID,
		Items:      items,
		Status:     domain.OrderStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Calculate total
	order.Total = order.CalculateTotal()

	// Validate order
	if err := order.Validate(); err != nil {
		return nil, err
	}

	// Save to repository
	if err := s.repo.Create(ctx, order); err != nil {
		return nil, err
	}

	// Publish event (warn + continue on failure)
	if s.publisher != nil {
		if err := s.publisher.PublishOrderCreated(ctx, order); err != nil {
			slog.Warn("failed to publish order.created event", slog.String("order_id", order.ID.String()), slog.String("error", err.Error()))
		}
	}

	return order, nil
}

func (s *orderServiceImpl) GetOrderByID(ctx context.Context, id string) (*domain.Order, error) {
	// Check cache first
	if s.cache != nil {
		cached, err := s.cache.Get(ctx, id)
		if err != nil {
			slog.Warn("cache get failed", slog.String("order_id", id), slog.String("error", err.Error()))
		} else if cached != nil {
			return cached, nil
		}
	}

	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if order == nil {
		return nil, domain.ErrOrderNotFound
	}

	// Populate cache
	if s.cache != nil {
		if err := s.cache.Set(ctx, order, orderCacheTTL); err != nil {
			slog.Warn("cache set failed", slog.String("order_id", id), slog.String("error", err.Error()))
		}
	}

	return order, nil
}

// UpdateOrder updates an existing order.
// Uses optimistic locking - returns ErrConcurrentModification if the order
// was modified by another process between read and write.
func (s *orderServiceImpl) UpdateOrder(ctx context.Context, id string, dto UpdateOrderDTO) (*domain.Order, error) {
	// Get existing order (includes current version for optimistic locking)
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if order == nil {
		return nil, domain.ErrOrderNotFound
	}

	// Update items if provided
	if len(dto.Items) > 0 {
		items := make([]domain.OrderItem, len(dto.Items))
		for i, item := range dto.Items {
			if err := item.Validate(); err != nil {
				return nil, err
			}

			items[i] = domain.OrderItem{
				ID:        uuid.New(),
				ProductID: item.ProductID,
				Name:      item.Name,
				Quantity:  item.Quantity,
				Price:     item.Price,
				Subtotal:  item.CalculateSubtotal(),
			}
		}
		order.Items = items
		order.Total = order.CalculateTotal()
	}

	// Update status if provided
	if dto.Status != nil {
		if !order.Status.CanTransitionTo(*dto.Status) {
			return nil, domain.ErrInvalidTransition
		}
		order.Status = *dto.Status
	}

	order.UpdatedAt = time.Now()

	// Save to repository
	if err := s.repo.Update(ctx, order); err != nil {
		return nil, err
	}

	// Publish event (warn + continue on failure)
	if s.publisher != nil {
		if err := s.publisher.PublishOrderUpdated(ctx, order); err != nil {
			slog.Warn("failed to publish order.updated event", slog.String("order_id", order.ID.String()), slog.String("error", err.Error()))
		}
	}

	return order, nil
}

func (s *orderServiceImpl) DeleteOrder(ctx context.Context, id string) error {
	// Check if order exists
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if order == nil {
		return domain.ErrOrderNotFound
	}

	// Soft delete
	return s.repo.Delete(ctx, id)
}

func (s *orderServiceImpl) ListOrders(ctx context.Context, req ListOrdersRequest) (*domain.PaginatedOrders, error) {
	// Set defaults
	page := req.Page
	if page < 1 {
		page = 1
	}

	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Build list options
	opts := repository.ListOptions{
		Limit:  pageSize,
		Offset: offset,
		Status: req.Status,
	}

	// Get orders from repository
	var orders []*domain.Order
	var totalCount int64
	var err error

	if req.CustomerID != nil && *req.CustomerID != "" {
		orders, totalCount, err = s.repo.FindByCustomerID(ctx, *req.CustomerID, opts)
	} else {
		orders, totalCount, err = s.repo.List(ctx, opts)
	}
	if err != nil {
		return nil, err
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(totalCount) / float64(pageSize)))

	return &domain.PaginatedOrders{
		Data:       orders,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: totalPages,
	}, nil
}

// UpdateOrderStatus transitions an order to a new status.
// Uses optimistic locking - returns ErrConcurrentModification if the order
// was modified by another process between read and write.
// This prevents race conditions like two concurrent status changes.
func (s *orderServiceImpl) UpdateOrderStatus(ctx context.Context, id string, newStatus domain.OrderStatus) (*domain.Order, error) {
	// Get existing order (includes current version for optimistic locking)
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if order == nil {
		return nil, domain.ErrOrderNotFound
	}

	// Validate status transition
	if !order.Status.CanTransitionTo(newStatus) {
		return nil, domain.ErrInvalidTransition
	}

	// Capture old status before mutation
	oldStatus := order.Status

	// Update status
	order.Status = newStatus
	order.UpdatedAt = time.Now()

	// Save to repository
	if err := s.repo.Update(ctx, order); err != nil {
		return nil, err
	}

	// Publish event (warn + continue on failure)
	if s.publisher != nil {
		if err := s.publisher.PublishOrderStatusChanged(ctx, order, oldStatus, newStatus); err != nil {
			slog.Warn("failed to publish order.status_changed event", slog.String("order_id", order.ID.String()), slog.String("error", err.Error()))
		}
	}

	// Invalidate cache
	if s.cache != nil {
		if err := s.cache.Delete(ctx, id); err != nil {
			slog.Warn("cache delete failed", slog.String("order_id", id), slog.String("error", err.Error()))
		}
	}

	return order, nil
}
