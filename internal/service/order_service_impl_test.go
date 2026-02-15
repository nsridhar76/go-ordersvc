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
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nsridhar76/go-ordersvc/internal/domain"
	"github.com/nsridhar76/go-ordersvc/internal/mocks"
	"github.com/nsridhar76/go-ordersvc/internal/repository"
	"github.com/stretchr/testify/assert"
)

func TestOrderService_CreateOrder_ValidInput_ReturnsOrder(t *testing.T) {
	tests := []struct {
		name    string
		dto     CreateOrderDTO
		wantErr error
	}{
		{
			name: "valid order with single item",
			dto: CreateOrderDTO{
				CustomerID: uuid.New().String(),
				Items: []domain.OrderItem{
					{
						ProductID: "product-1",
						Name:      "Test Product",
						Quantity:  2,
						Price:     10.50,
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "valid order with multiple items",
			dto: CreateOrderDTO{
				CustomerID: uuid.New().String(),
				Items: []domain.OrderItem{
					{
						ProductID: "product-1",
						Name:      "Product A",
						Quantity:  1,
						Price:     5.00,
					},
					{
						ProductID: "product-2",
						Name:      "Product B",
						Quantity:  3,
						Price:     15.00,
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.OrderRepositoryMock{
				CreateFunc: func(ctx context.Context, order *domain.Order) error {
					return nil
				},
			}

			service := NewOrderService(mockRepo)
			order, err := service.CreateOrder(context.Background(), tt.dto)

			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.wantErr, err)
				assert.Nil(t, order)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, order)
				assert.Equal(t, tt.dto.CustomerID, order.CustomerID)
				assert.Equal(t, len(tt.dto.Items), len(order.Items))
				assert.Equal(t, domain.OrderStatusPending, order.Status)
				assert.NotEqual(t, uuid.Nil, order.ID)
				assert.Greater(t, order.Total, 0.0)
			}
		})
	}
}

func TestOrderService_CreateOrder_InvalidInput_ReturnsError(t *testing.T) {
	tests := []struct {
		name    string
		dto     CreateOrderDTO
		wantErr error
	}{
		{
			name: "missing customer ID",
			dto: CreateOrderDTO{
				CustomerID: "",
				Items: []domain.OrderItem{
					{
						ProductID: "product-1",
						Name:      "Test Product",
						Quantity:  1,
						Price:     10.00,
					},
				},
			},
			wantErr: domain.ErrInvalidCustomerID,
		},
		{
			name: "empty items",
			dto: CreateOrderDTO{
				CustomerID: uuid.New().String(),
				Items:      []domain.OrderItem{},
			},
			wantErr: domain.ErrNoItems,
		},
		{
			name: "item with invalid quantity",
			dto: CreateOrderDTO{
				CustomerID: uuid.New().String(),
				Items: []domain.OrderItem{
					{
						ProductID: "product-1",
						Name:      "Test Product",
						Quantity:  0,
						Price:     10.00,
					},
				},
			},
			wantErr: domain.ErrInvalidQuantity,
		},
		{
			name: "item with invalid price",
			dto: CreateOrderDTO{
				CustomerID: uuid.New().String(),
				Items: []domain.OrderItem{
					{
						ProductID: "product-1",
						Name:      "Test Product",
						Quantity:  1,
						Price:     -5.00,
					},
				},
			},
			wantErr: domain.ErrInvalidPrice,
		},
		{
			name: "item with missing product ID",
			dto: CreateOrderDTO{
				CustomerID: uuid.New().String(),
				Items: []domain.OrderItem{
					{
						ProductID: "",
						Name:      "Test Product",
						Quantity:  1,
						Price:     10.00,
					},
				},
			},
			wantErr: domain.ErrInvalidProductID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.OrderRepositoryMock{}
			service := NewOrderService(mockRepo)

			order, err := service.CreateOrder(context.Background(), tt.dto)

			assert.Error(t, err)
			assert.Equal(t, tt.wantErr, err)
			assert.Nil(t, order)
		})
	}
}

func TestOrderService_GetOrderByID_Found_ReturnsOrder(t *testing.T) {
	orderID := uuid.New()
	expectedOrder := &domain.Order{
		ID:         orderID,
		CustomerID: uuid.New().String(),
		Items: []domain.OrderItem{
			{
				ID:        uuid.New(),
				ProductID: "product-1",
				Name:      "Test Product",
				Quantity:  2,
				Price:     10.00,
				Subtotal:  20.00,
			},
		},
		Status:    domain.OrderStatusPending,
		Total:     20.00,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo := &mocks.OrderRepositoryMock{
		FindByIDFunc: func(ctx context.Context, id string) (*domain.Order, error) {
			assert.Equal(t, orderID.String(), id)
			return expectedOrder, nil
		},
	}

	service := NewOrderService(mockRepo)
	order, err := service.GetOrderByID(context.Background(), orderID.String())

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, expectedOrder.ID, order.ID)
	assert.Equal(t, expectedOrder.CustomerID, order.CustomerID)
}

func TestOrderService_GetOrderByID_NotFound_ReturnsError(t *testing.T) {
	orderID := uuid.New()

	mockRepo := &mocks.OrderRepositoryMock{
		FindByIDFunc: func(ctx context.Context, id string) (*domain.Order, error) {
			return nil, domain.ErrOrderNotFound
		},
	}

	service := NewOrderService(mockRepo)
	order, err := service.GetOrderByID(context.Background(), orderID.String())

	assert.Error(t, err)
	assert.Equal(t, domain.ErrOrderNotFound, err)
	assert.Nil(t, order)
}

func TestOrderService_ListOrders_WithPagination_ReturnsOrders(t *testing.T) {
	tests := []struct {
		name           string
		request        ListOrdersRequest
		mockOrders     []*domain.Order
		mockTotalCount int64
		expectedCount  int
		expectedPages  int
	}{
		{
			name: "first page with 10 items",
			request: ListOrdersRequest{
				Page:     1,
				PageSize: 10,
			},
			mockOrders:     createMockOrders(10),
			mockTotalCount: 25,
			expectedCount:  10,
			expectedPages:  3,
		},
		{
			name: "second page with 10 items",
			request: ListOrdersRequest{
				Page:     2,
				PageSize: 10,
			},
			mockOrders:     createMockOrders(10),
			mockTotalCount: 25,
			expectedCount:  10,
			expectedPages:  3,
		},
		{
			name: "last page with partial items",
			request: ListOrdersRequest{
				Page:     3,
				PageSize: 10,
			},
			mockOrders:     createMockOrders(5),
			mockTotalCount: 25,
			expectedCount:  5,
			expectedPages:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.OrderRepositoryMock{
				ListFunc: func(ctx context.Context, opts repository.ListOptions) ([]*domain.Order, int64, error) {
					expectedOffset := (tt.request.Page - 1) * tt.request.PageSize
					assert.Equal(t, tt.request.PageSize, opts.Limit)
					assert.Equal(t, expectedOffset, opts.Offset)
					return tt.mockOrders, tt.mockTotalCount, nil
				},
			}

			service := NewOrderService(mockRepo)
			result, err := service.ListOrders(context.Background(), tt.request)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedCount, len(result.Data))
			assert.Equal(t, tt.mockTotalCount, result.TotalCount)
			assert.Equal(t, tt.expectedPages, result.TotalPages)
			assert.Equal(t, tt.request.Page, result.Page)
		})
	}
}

func TestOrderService_ListOrders_WithStatusFilter_ReturnsFilteredOrders(t *testing.T) {
	pendingStatus := domain.OrderStatusPending
	confirmedStatus := domain.OrderStatusConfirmed

	tests := []struct {
		name       string
		request    ListOrdersRequest
		mockOrders []*domain.Order
	}{
		{
			name: "filter by pending status",
			request: ListOrdersRequest{
				Page:     1,
				PageSize: 10,
				Status:   &pendingStatus,
			},
			mockOrders: []*domain.Order{
				createMockOrder(domain.OrderStatusPending),
				createMockOrder(domain.OrderStatusPending),
			},
		},
		{
			name: "filter by confirmed status",
			request: ListOrdersRequest{
				Page:     1,
				PageSize: 10,
				Status:   &confirmedStatus,
			},
			mockOrders: []*domain.Order{
				createMockOrder(domain.OrderStatusConfirmed),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.OrderRepositoryMock{
				ListFunc: func(ctx context.Context, opts repository.ListOptions) ([]*domain.Order, int64, error) {
					assert.Equal(t, tt.request.Status, opts.Status)
					return tt.mockOrders, int64(len(tt.mockOrders)), nil
				},
			}

			service := NewOrderService(mockRepo)
			result, err := service.ListOrders(context.Background(), tt.request)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, len(tt.mockOrders), len(result.Data))

			// Verify all returned orders have the requested status
			if tt.request.Status != nil {
				for _, order := range result.Data {
					assert.Equal(t, *tt.request.Status, order.Status)
				}
			}
		})
	}
}

func TestOrderService_ListOrders_WithCustomerID_ReturnsFilteredOrders(t *testing.T) {
	customerA := "customer-a"
	customerB := "customer-b"
	pendingStatus := domain.OrderStatusPending

	tests := []struct {
		name             string
		request          ListOrdersRequest
		mockOrders       []*domain.Order
		mockTotalCount   int64
		expectCustomerID string
	}{
		{
			name: "filter by customer_id only",
			request: ListOrdersRequest{
				Page:       1,
				PageSize:   10,
				CustomerID: &customerA,
			},
			mockOrders:       createMockOrdersForCustomer(customerA, 3),
			mockTotalCount:   3,
			expectCustomerID: customerA,
		},
		{
			name: "filter by customer_id and status",
			request: ListOrdersRequest{
				Page:       1,
				PageSize:   10,
				CustomerID: &customerB,
				Status:     &pendingStatus,
			},
			mockOrders:       createMockOrdersForCustomer(customerB, 2),
			mockTotalCount:   2,
			expectCustomerID: customerB,
		},
		{
			name: "customer with no orders returns empty list",
			request: ListOrdersRequest{
				Page:       1,
				PageSize:   10,
				CustomerID: &customerA,
			},
			mockOrders:       []*domain.Order{},
			mockTotalCount:   0,
			expectCustomerID: customerA,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mocks.OrderRepositoryMock{
				FindByCustomerIDFunc: func(ctx context.Context, customerID string, opts repository.ListOptions) ([]*domain.Order, int64, error) {
					assert.Equal(t, tt.expectCustomerID, customerID)
					assert.Equal(t, tt.request.Status, opts.Status)
					return tt.mockOrders, tt.mockTotalCount, nil
				},
				ListFunc: func(ctx context.Context, opts repository.ListOptions) ([]*domain.Order, int64, error) {
					t.Fatal("List should not be called when CustomerID is set")
					return nil, 0, nil
				},
			}

			svc := NewOrderService(mockRepo)
			result, err := svc.ListOrders(context.Background(), tt.request)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, len(tt.mockOrders), len(result.Data))
			assert.Equal(t, tt.mockTotalCount, result.TotalCount)
		})
	}
}

func TestOrderService_ListOrders_WithoutCustomerID_CallsList(t *testing.T) {
	mockRepo := &mocks.OrderRepositoryMock{
		ListFunc: func(ctx context.Context, opts repository.ListOptions) ([]*domain.Order, int64, error) {
			return createMockOrders(5), 5, nil
		},
		FindByCustomerIDFunc: func(ctx context.Context, customerID string, opts repository.ListOptions) ([]*domain.Order, int64, error) {
			t.Fatal("FindByCustomerID should not be called when CustomerID is nil")
			return nil, 0, nil
		},
	}

	svc := NewOrderService(mockRepo)
	result, err := svc.ListOrders(context.Background(), ListOrdersRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 5, len(result.Data))
}

func TestOrderService_ListOrders_EmptyResults_ReturnsEmptyList(t *testing.T) {
	mockRepo := &mocks.OrderRepositoryMock{
		ListFunc: func(ctx context.Context, opts repository.ListOptions) ([]*domain.Order, int64, error) {
			return []*domain.Order{}, 0, nil
		},
	}

	service := NewOrderService(mockRepo)
	result, err := service.ListOrders(context.Background(), ListOrdersRequest{
		Page:     1,
		PageSize: 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, len(result.Data))
	assert.Equal(t, int64(0), result.TotalCount)
	assert.Equal(t, 0, result.TotalPages)
}

func TestOrderService_UpdateOrderStatus_ValidTransitions_Success(t *testing.T) {
	tests := []struct {
		name          string
		currentStatus domain.OrderStatus
		newStatus     domain.OrderStatus
	}{
		{
			name:          "pending to confirmed",
			currentStatus: domain.OrderStatusPending,
			newStatus:     domain.OrderStatusConfirmed,
		},
		{
			name:          "pending to cancelled",
			currentStatus: domain.OrderStatusPending,
			newStatus:     domain.OrderStatusCancelled,
		},
		{
			name:          "confirmed to processing",
			currentStatus: domain.OrderStatusConfirmed,
			newStatus:     domain.OrderStatusProcessing,
		},
		{
			name:          "confirmed to cancelled",
			currentStatus: domain.OrderStatusConfirmed,
			newStatus:     domain.OrderStatusCancelled,
		},
		{
			name:          "processing to shipped",
			currentStatus: domain.OrderStatusProcessing,
			newStatus:     domain.OrderStatusShipped,
		},
		{
			name:          "processing to cancelled",
			currentStatus: domain.OrderStatusProcessing,
			newStatus:     domain.OrderStatusCancelled,
		},
		{
			name:          "shipped to delivered",
			currentStatus: domain.OrderStatusShipped,
			newStatus:     domain.OrderStatusDelivered,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orderID := uuid.New()
			currentOrder := &domain.Order{
				ID:         orderID,
				CustomerID: uuid.New().String(),
				Items: []domain.OrderItem{
					{
						ID:        uuid.New(),
						ProductID: "product-1",
						Name:      "Test Product",
						Quantity:  1,
						Price:     10.00,
						Subtotal:  10.00,
					},
				},
				Status:    tt.currentStatus,
				Total:     10.00,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			mockRepo := &mocks.OrderRepositoryMock{
				FindByIDFunc: func(ctx context.Context, id string) (*domain.Order, error) {
					return currentOrder, nil
				},
				UpdateFunc: func(ctx context.Context, order *domain.Order) error {
					assert.Equal(t, tt.newStatus, order.Status)
					return nil
				},
			}

			service := NewOrderService(mockRepo)
			updatedOrder, err := service.UpdateOrderStatus(context.Background(), orderID.String(), tt.newStatus)

			assert.NoError(t, err)
			assert.NotNil(t, updatedOrder)
			assert.Equal(t, tt.newStatus, updatedOrder.Status)
		})
	}
}

func TestOrderService_UpdateOrderStatus_InvalidTransitions_ReturnsError(t *testing.T) {
	tests := []struct {
		name          string
		currentStatus domain.OrderStatus
		newStatus     domain.OrderStatus
	}{
		{
			name:          "pending to shipped (skip confirmed)",
			currentStatus: domain.OrderStatusPending,
			newStatus:     domain.OrderStatusShipped,
		},
		{
			name:          "pending to delivered (skip steps)",
			currentStatus: domain.OrderStatusPending,
			newStatus:     domain.OrderStatusDelivered,
		},
		{
			name:          "confirmed to shipped (skip processing)",
			currentStatus: domain.OrderStatusConfirmed,
			newStatus:     domain.OrderStatusShipped,
		},
		{
			name:          "shipped to confirmed (backwards)",
			currentStatus: domain.OrderStatusShipped,
			newStatus:     domain.OrderStatusConfirmed,
		},
		{
			name:          "delivered to any state (terminal)",
			currentStatus: domain.OrderStatusDelivered,
			newStatus:     domain.OrderStatusPending,
		},
		{
			name:          "cancelled to any state (terminal)",
			currentStatus: domain.OrderStatusCancelled,
			newStatus:     domain.OrderStatusConfirmed,
		},
		{
			name:          "shipped to cancelled (can't cancel after shipped)",
			currentStatus: domain.OrderStatusShipped,
			newStatus:     domain.OrderStatusCancelled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orderID := uuid.New()
			currentOrder := &domain.Order{
				ID:         orderID,
				CustomerID: uuid.New().String(),
				Items: []domain.OrderItem{
					{
						ID:        uuid.New(),
						ProductID: "product-1",
						Name:      "Test Product",
						Quantity:  1,
						Price:     10.00,
						Subtotal:  10.00,
					},
				},
				Status:    tt.currentStatus,
				Total:     10.00,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			mockRepo := &mocks.OrderRepositoryMock{
				FindByIDFunc: func(ctx context.Context, id string) (*domain.Order, error) {
					return currentOrder, nil
				},
			}

			service := NewOrderService(mockRepo)
			updatedOrder, err := service.UpdateOrderStatus(context.Background(), orderID.String(), tt.newStatus)

			assert.Error(t, err)
			assert.Equal(t, domain.ErrInvalidTransition, err)
			assert.Nil(t, updatedOrder)
		})
	}
}

func TestOrderService_UpdateOrderStatus_OrderNotFound_ReturnsError(t *testing.T) {
	orderID := uuid.New()

	mockRepo := &mocks.OrderRepositoryMock{
		FindByIDFunc: func(ctx context.Context, id string) (*domain.Order, error) {
			return nil, domain.ErrOrderNotFound
		},
	}

	service := NewOrderService(mockRepo)
	updatedOrder, err := service.UpdateOrderStatus(context.Background(), orderID.String(), domain.OrderStatusConfirmed)

	assert.Error(t, err)
	assert.Equal(t, domain.ErrOrderNotFound, err)
	assert.Nil(t, updatedOrder)
}

// Helper functions

func createMockOrders(count int) []*domain.Order {
	orders := make([]*domain.Order, count)
	for i := 0; i < count; i++ {
		orders[i] = createMockOrder(domain.OrderStatusPending)
	}
	return orders
}

func createMockOrder(status domain.OrderStatus) *domain.Order {
	return createMockOrderWithVersion(status, 1)
}

func createMockOrdersForCustomer(customerID string, count int) []*domain.Order {
	orders := make([]*domain.Order, count)
	for i := 0; i < count; i++ {
		o := createMockOrder(domain.OrderStatusPending)
		o.CustomerID = customerID
		orders[i] = o
	}
	return orders
}

func createMockOrderWithVersion(status domain.OrderStatus, version int) *domain.Order {
	return &domain.Order{
		ID:         uuid.New(),
		CustomerID: uuid.New().String(),
		Items: []domain.OrderItem{
			{
				ID:        uuid.New(),
				ProductID: "product-1",
				Name:      "Test Product",
				Quantity:  1,
				Price:     10.00,
				Subtotal:  10.00,
			},
		},
		Status:    status,
		Total:     10.00,
		Version:   version,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// =============================================================================
// Optimistic Locking Tests
// =============================================================================

func TestOrderService_UpdateOrderStatus_ConcurrentModification_ReturnsError(t *testing.T) {
	orderID := uuid.New()

	// Order with version 1
	currentOrder := &domain.Order{
		ID:         orderID,
		CustomerID: uuid.New().String(),
		Items: []domain.OrderItem{
			{
				ID:        uuid.New(),
				ProductID: "product-1",
				Name:      "Test Product",
				Quantity:  1,
				Price:     10.00,
				Subtotal:  10.00,
			},
		},
		Status:    domain.OrderStatusPending,
		Total:     10.00,
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo := &mocks.OrderRepositoryMock{
		FindByIDFunc: func(ctx context.Context, id string) (*domain.Order, error) {
			return currentOrder, nil
		},
		UpdateFunc: func(ctx context.Context, order *domain.Order) error {
			// Simulate another process having modified the order
			// (version in DB is now 2, but we're trying to update with version 1)
			return domain.ErrConcurrentModification
		},
	}

	service := NewOrderService(mockRepo)
	updatedOrder, err := service.UpdateOrderStatus(context.Background(), orderID.String(), domain.OrderStatusConfirmed)

	assert.Error(t, err)
	assert.Equal(t, domain.ErrConcurrentModification, err)
	assert.Nil(t, updatedOrder)
}

func TestOrderService_UpdateOrder_ConcurrentModification_ReturnsError(t *testing.T) {
	orderID := uuid.New()

	currentOrder := &domain.Order{
		ID:         orderID,
		CustomerID: uuid.New().String(),
		Items: []domain.OrderItem{
			{
				ID:        uuid.New(),
				ProductID: "product-1",
				Name:      "Test Product",
				Quantity:  1,
				Price:     10.00,
				Subtotal:  10.00,
			},
		},
		Status:    domain.OrderStatusPending,
		Total:     10.00,
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo := &mocks.OrderRepositoryMock{
		FindByIDFunc: func(ctx context.Context, id string) (*domain.Order, error) {
			return currentOrder, nil
		},
		UpdateFunc: func(ctx context.Context, order *domain.Order) error {
			// Simulate concurrent modification
			return domain.ErrConcurrentModification
		},
	}

	service := NewOrderService(mockRepo)

	dto := UpdateOrderDTO{
		Items: []domain.OrderItem{
			{
				ProductID: "product-2",
				Name:      "New Product",
				Quantity:  2,
				Price:     20.00,
			},
		},
	}

	updatedOrder, err := service.UpdateOrder(context.Background(), orderID.String(), dto)

	assert.Error(t, err)
	assert.Equal(t, domain.ErrConcurrentModification, err)
	assert.Nil(t, updatedOrder)
}

func TestOrderService_UpdateOrderStatus_VersionIncrementsOnSuccess(t *testing.T) {
	orderID := uuid.New()

	currentOrder := &domain.Order{
		ID:         orderID,
		CustomerID: uuid.New().String(),
		Items: []domain.OrderItem{
			{
				ID:        uuid.New(),
				ProductID: "product-1",
				Name:      "Test Product",
				Quantity:  1,
				Price:     10.00,
				Subtotal:  10.00,
			},
		},
		Status:    domain.OrderStatusPending,
		Total:     10.00,
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo := &mocks.OrderRepositoryMock{
		FindByIDFunc: func(ctx context.Context, id string) (*domain.Order, error) {
			return currentOrder, nil
		},
		UpdateFunc: func(ctx context.Context, order *domain.Order) error {
			// Simulate successful update - version gets incremented
			order.Version++
			return nil
		},
	}

	service := NewOrderService(mockRepo)
	updatedOrder, err := service.UpdateOrderStatus(context.Background(), orderID.String(), domain.OrderStatusConfirmed)

	assert.NoError(t, err)
	assert.NotNil(t, updatedOrder)
	assert.Equal(t, 2, updatedOrder.Version) // Version should be incremented
	assert.Equal(t, domain.OrderStatusConfirmed, updatedOrder.Status)
}

func TestOrderService_CreateOrder_SetsInitialVersion(t *testing.T) {
	mockRepo := &mocks.OrderRepositoryMock{
		CreateFunc: func(ctx context.Context, order *domain.Order) error {
			// Repository sets version to 1 on create
			order.Version = 1
			return nil
		},
	}

	service := NewOrderService(mockRepo)

	dto := CreateOrderDTO{
		CustomerID: uuid.New().String(),
		Items: []domain.OrderItem{
			{
				ProductID: "product-1",
				Name:      "Test Product",
				Quantity:  1,
				Price:     10.00,
			},
		},
	}

	order, err := service.CreateOrder(context.Background(), dto)

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, 1, order.Version)
}

func TestOrderService_UpdateOrderStatus_PreservesVersionFromRead(t *testing.T) {
	orderID := uuid.New()

	// Order at version 5 (has been updated several times)
	currentOrder := &domain.Order{
		ID:         orderID,
		CustomerID: uuid.New().String(),
		Items: []domain.OrderItem{
			{
				ID:        uuid.New(),
				ProductID: "product-1",
				Name:      "Test Product",
				Quantity:  1,
				Price:     10.00,
				Subtotal:  10.00,
			},
		},
		Status:    domain.OrderStatusProcessing,
		Total:     10.00,
		Version:   5,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	var capturedVersion int
	mockRepo := &mocks.OrderRepositoryMock{
		FindByIDFunc: func(ctx context.Context, id string) (*domain.Order, error) {
			return currentOrder, nil
		},
		UpdateFunc: func(ctx context.Context, order *domain.Order) error {
			// Capture the version being sent to update
			capturedVersion = order.Version
			order.Version++
			return nil
		},
	}

	service := NewOrderService(mockRepo)
	updatedOrder, err := service.UpdateOrderStatus(context.Background(), orderID.String(), domain.OrderStatusShipped)

	assert.NoError(t, err)
	assert.NotNil(t, updatedOrder)
	assert.Equal(t, 5, capturedVersion)      // Should use version from FindByID
	assert.Equal(t, 6, updatedOrder.Version) // Should be incremented after update
}
