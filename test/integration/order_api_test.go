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

//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Request/Response types for integration tests

type CreateOrderRequest struct {
	CustomerID string      `json:"customer_id"`
	Items      []OrderItem `json:"items"`
}

type OrderItem struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type OrderResponse struct {
	ID         string  `json:"id"`
	CustomerID string  `json:"customer_id"`
	Items      []struct {
		ID        string  `json:"id"`
		ProductID string  `json:"product_id"`
		Name      string  `json:"name"`
		Quantity  int     `json:"quantity"`
		Price     float64 `json:"price"`
		Subtotal  float64 `json:"subtotal"`
	} `json:"items"`
	Status    string  `json:"status"`
	Total     float64 `json:"total"`
	Version   int     `json:"version"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type ListOrdersResponse struct {
	Orders []OrderResponse `json:"orders"`
	Total  int64           `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type UpdateStatusRequest struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Health check tests

func TestHealthz_ReturnsOK(t *testing.T) {
	resp, body := get(t, "/healthz")

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health map[string]interface{}
	err := json.Unmarshal(body, &health)
	require.NoError(t, err)
	assert.Equal(t, "ok", health["status"])
}

func TestReadyz_ReturnsOK(t *testing.T) {
	resp, body := get(t, "/readyz")

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health map[string]interface{}
	err := json.Unmarshal(body, &health)
	require.NoError(t, err)
	assert.Equal(t, "ok", health["status"])
}

// POST /api/v1/orders tests
// ADR-0002 CONSTRAINT: POST Must Return 201 with Location Header

func TestCreateOrder_ValidRequest_Returns201WithLocation(t *testing.T) {
	req := CreateOrderRequest{
		CustomerID: uuid.New().String(),
		Items: []OrderItem{
			{ProductID: "prod-1", Name: "Test Product", Quantity: 2, Price: 29.99},
		},
	}

	resp, body := post(t, "/api/v1/orders", req)

	// ADR-0002 CONSTRAINT: Returns 201
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Expected 201 Created")

	// ADR-0002 CONSTRAINT: Location header present
	location := resp.Header.Get("Location")
	assert.NotEmpty(t, location, "Location header should be set")
	assert.True(t, strings.HasPrefix(location, "/api/v1/orders/"), "Location should start with /api/v1/orders/")

	var order OrderResponse
	err := json.Unmarshal(body, &order)
	require.NoError(t, err)

	assert.NotEmpty(t, order.ID)
	assert.Equal(t, req.CustomerID, order.CustomerID)
	assert.Equal(t, "pending", order.Status)
	assert.Equal(t, 1, order.Version, "Initial version should be 1 (ADR-0003)")
	assert.Equal(t, 59.98, order.Total)
}

func TestCreateOrder_MissingCustomerID_Returns400(t *testing.T) {
	req := CreateOrderRequest{
		CustomerID: "",
		Items: []OrderItem{
			{ProductID: "prod-1", Name: "Test", Quantity: 1, Price: 10.00},
		},
	}

	resp, body := post(t, "/api/v1/orders", req)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp ErrorResponse
	err := json.Unmarshal(body, &errResp)
	require.NoError(t, err)
	assert.Equal(t, "MISSING_CUSTOMER_ID", errResp.Code)
}

func TestCreateOrder_EmptyItems_Returns400(t *testing.T) {
	req := CreateOrderRequest{
		CustomerID: uuid.New().String(),
		Items:      []OrderItem{},
	}

	resp, body := post(t, "/api/v1/orders", req)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp ErrorResponse
	err := json.Unmarshal(body, &errResp)
	require.NoError(t, err)
	assert.Equal(t, "MISSING_ITEMS", errResp.Code)
}

// GET /api/v1/orders/:id tests
// ADR-0002 CONSTRAINT: GET by ID Must Return 404 for Missing Orders

func TestGetOrder_ExistingOrder_Returns200(t *testing.T) {
	// First create an order
	createReq := CreateOrderRequest{
		CustomerID: uuid.New().String(),
		Items: []OrderItem{
			{ProductID: "prod-1", Name: "Test", Quantity: 1, Price: 19.99},
		},
	}
	createResp, createBody := post(t, "/api/v1/orders", createReq)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createdOrder OrderResponse
	err := json.Unmarshal(createBody, &createdOrder)
	require.NoError(t, err)

	// Now get the order
	resp, body := get(t, "/api/v1/orders/"+createdOrder.ID)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var order OrderResponse
	err = json.Unmarshal(body, &order)
	require.NoError(t, err)
	assert.Equal(t, createdOrder.ID, order.ID)
	assert.Equal(t, createdOrder.CustomerID, order.CustomerID)
}

func TestGetOrder_NonExistent_Returns404(t *testing.T) {
	nonExistentID := uuid.New().String()

	resp, body := get(t, "/api/v1/orders/"+nonExistentID)

	// ADR-0002 CONSTRAINT: Returns 404 for missing orders
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected 404 for non-existent order")

	var errResp ErrorResponse
	err := json.Unmarshal(body, &errResp)
	require.NoError(t, err)
	assert.Equal(t, "ORDER_NOT_FOUND", errResp.Code)
}

// GET /api/v1/orders tests (pagination)

func TestListOrders_Pagination_ReturnsCorrectFormat(t *testing.T) {
	// Create a few orders first
	for i := 0; i < 3; i++ {
		req := CreateOrderRequest{
			CustomerID: uuid.New().String(),
			Items: []OrderItem{
				{ProductID: "prod-" + string(rune('a'+i)), Name: "Test", Quantity: 1, Price: 10.00},
			},
		}
		resp, _ := post(t, "/api/v1/orders", req)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	// List orders with pagination
	resp, body := get(t, "/api/v1/orders?limit=2&offset=0")

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResp ListOrdersResponse
	err := json.Unmarshal(body, &listResp)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(listResp.Orders), 2, "Should respect limit")
	assert.Equal(t, 2, listResp.Limit)
	assert.Equal(t, 0, listResp.Offset)
	assert.GreaterOrEqual(t, listResp.Total, int64(3), "Total should be at least 3")
}

func TestListOrders_StatusFilter_FiltersCorrectly(t *testing.T) {
	// Create an order
	req := CreateOrderRequest{
		CustomerID: uuid.New().String(),
		Items: []OrderItem{
			{ProductID: "prod-filter", Name: "Filter Test", Quantity: 1, Price: 15.00},
		},
	}
	resp, _ := post(t, "/api/v1/orders", req)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// List orders filtered by pending status
	resp, body := get(t, "/api/v1/orders?status=pending&limit=100")

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResp ListOrdersResponse
	err := json.Unmarshal(body, &listResp)
	require.NoError(t, err)

	// All returned orders should have pending status
	for _, order := range listResp.Orders {
		assert.Equal(t, "pending", order.Status, "All orders should be pending")
	}
}

// PATCH /api/v1/orders/:id/status tests

func TestUpdateOrderStatus_ValidTransition_Returns200(t *testing.T) {
	// Create an order
	createReq := CreateOrderRequest{
		CustomerID: uuid.New().String(),
		Items: []OrderItem{
			{ProductID: "prod-1", Name: "Test", Quantity: 1, Price: 25.00},
		},
	}
	createResp, createBody := post(t, "/api/v1/orders", createReq)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createdOrder OrderResponse
	err := json.Unmarshal(createBody, &createdOrder)
	require.NoError(t, err)

	// Update status from pending to confirmed (valid transition)
	updateReq := UpdateStatusRequest{Status: "confirmed"}
	resp, body := patch(t, "/api/v1/orders/"+createdOrder.ID+"/status", updateReq)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var updatedOrder OrderResponse
	err = json.Unmarshal(body, &updatedOrder)
	require.NoError(t, err)
	assert.Equal(t, "confirmed", updatedOrder.Status)
	assert.Equal(t, 2, updatedOrder.Version, "Version should increment on update (ADR-0003)")
}

func TestUpdateOrderStatus_InvalidTransition_Returns400(t *testing.T) {
	// Create an order
	createReq := CreateOrderRequest{
		CustomerID: uuid.New().String(),
		Items: []OrderItem{
			{ProductID: "prod-1", Name: "Test", Quantity: 1, Price: 25.00},
		},
	}
	createResp, createBody := post(t, "/api/v1/orders", createReq)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createdOrder OrderResponse
	err := json.Unmarshal(createBody, &createdOrder)
	require.NoError(t, err)

	// Try invalid transition: pending -> delivered (should fail)
	updateReq := UpdateStatusRequest{Status: "delivered"}
	resp, body := patch(t, "/api/v1/orders/"+createdOrder.ID+"/status", updateReq)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp ErrorResponse
	err = json.Unmarshal(body, &errResp)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_TRANSITION", errResp.Code)
}

func TestUpdateOrderStatus_NonExistentOrder_Returns404(t *testing.T) {
	nonExistentID := uuid.New().String()
	updateReq := UpdateStatusRequest{Status: "confirmed"}

	resp, body := patch(t, "/api/v1/orders/"+nonExistentID+"/status", updateReq)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var errResp ErrorResponse
	err := json.Unmarshal(body, &errResp)
	require.NoError(t, err)
	assert.Equal(t, "ORDER_NOT_FOUND", errResp.Code)
}

// DELETE /api/v1/orders/:id tests

func TestDeleteOrder_ExistingOrder_Returns204(t *testing.T) {
	// Create an order
	createReq := CreateOrderRequest{
		CustomerID: uuid.New().String(),
		Items: []OrderItem{
			{ProductID: "prod-delete", Name: "To Delete", Quantity: 1, Price: 5.00},
		},
	}
	createResp, createBody := post(t, "/api/v1/orders", createReq)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createdOrder OrderResponse
	err := json.Unmarshal(createBody, &createdOrder)
	require.NoError(t, err)

	// Delete the order
	resp, _ := delete(t, "/api/v1/orders/"+createdOrder.ID)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Verify it's gone (should return 404)
	getResp, _ := get(t, "/api/v1/orders/"+createdOrder.ID)
	assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
}

func TestDeleteOrder_NonExistent_Returns404(t *testing.T) {
	nonExistentID := uuid.New().String()

	resp, body := delete(t, "/api/v1/orders/"+nonExistentID)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var errResp ErrorResponse
	err := json.Unmarshal(body, &errResp)
	require.NoError(t, err)
	assert.Equal(t, "ORDER_NOT_FOUND", errResp.Code)
}

// GET /api/v1/orders?customer_id= tests (ORD-100)

func TestListOrders_CustomerIDFilter_ReturnsOnlyCustomerOrders(t *testing.T) {
	customerA := uuid.New().String()
	customerB := uuid.New().String()

	// Create 2 orders for customer A
	for i := 0; i < 2; i++ {
		req := CreateOrderRequest{
			CustomerID: customerA,
			Items:      []OrderItem{{ProductID: "prod-a", Name: "Product A", Quantity: 1, Price: 10.00}},
		}
		resp, _ := post(t, "/api/v1/orders", req)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	// Create 1 order for customer B
	reqB := CreateOrderRequest{
		CustomerID: customerB,
		Items:      []OrderItem{{ProductID: "prod-b", Name: "Product B", Quantity: 1, Price: 20.00}},
	}
	resp, _ := post(t, "/api/v1/orders", reqB)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// List orders for customer A only
	resp, body := get(t, "/api/v1/orders?customer_id="+customerA)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResp ListOrdersResponse
	err := json.Unmarshal(body, &listResp)
	require.NoError(t, err)

	assert.Equal(t, int64(2), listResp.Total, "Customer A should have exactly 2 orders")
	for _, order := range listResp.Orders {
		assert.Equal(t, customerA, order.CustomerID, "All returned orders should belong to customer A")
	}
}

func TestListOrders_CustomerIDWithStatusFilter_ReturnsCombinedFilter(t *testing.T) {
	customerID := uuid.New().String()

	// Create an order for this customer
	createReq := CreateOrderRequest{
		CustomerID: customerID,
		Items:      []OrderItem{{ProductID: "prod-combo", Name: "Combo Test", Quantity: 1, Price: 30.00}},
	}
	createResp, createBody := post(t, "/api/v1/orders", createReq)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createdOrder OrderResponse
	err := json.Unmarshal(createBody, &createdOrder)
	require.NoError(t, err)

	// Transition to confirmed
	statusResp, _ := patch(t, "/api/v1/orders/"+createdOrder.ID+"/status", UpdateStatusRequest{Status: "confirmed"})
	require.Equal(t, http.StatusOK, statusResp.StatusCode)

	// Filter by customer_id + status=pending (should NOT include the confirmed order)
	resp, body := get(t, "/api/v1/orders?customer_id="+customerID+"&status=pending")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var pendingResp ListOrdersResponse
	err = json.Unmarshal(body, &pendingResp)
	require.NoError(t, err)

	for _, order := range pendingResp.Orders {
		assert.Equal(t, "pending", order.Status, "Should only return pending orders")
	}

	// Filter by customer_id + status=confirmed (should include the confirmed order)
	resp, body = get(t, "/api/v1/orders?customer_id="+customerID+"&status=confirmed")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var confirmedResp ListOrdersResponse
	err = json.Unmarshal(body, &confirmedResp)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(confirmedResp.Orders), 1, "Should find the confirmed order")
	for _, order := range confirmedResp.Orders {
		assert.Equal(t, customerID, order.CustomerID)
		assert.Equal(t, "confirmed", order.Status)
	}
}

func TestListOrders_CustomerIDWithNoOrders_ReturnsEmptyList(t *testing.T) {
	nonExistentCustomer := uuid.New().String()

	resp, body := get(t, "/api/v1/orders?customer_id="+nonExistentCustomer)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 with empty list, not 404")

	var listResp ListOrdersResponse
	err := json.Unmarshal(body, &listResp)
	require.NoError(t, err)

	assert.Equal(t, int64(0), listResp.Total)
	assert.Empty(t, listResp.Orders)
}

// Full lifecycle test

func TestOrderLifecycle_FullFlow(t *testing.T) {
	customerID := uuid.New().String()

	// 1. Create order
	createReq := CreateOrderRequest{
		CustomerID: customerID,
		Items: []OrderItem{
			{ProductID: "prod-lifecycle", Name: "Lifecycle Test", Quantity: 2, Price: 49.99},
		},
	}
	createResp, createBody := post(t, "/api/v1/orders", createReq)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var order OrderResponse
	err := json.Unmarshal(createBody, &order)
	require.NoError(t, err)
	assert.Equal(t, "pending", order.Status)
	assert.Equal(t, 1, order.Version)

	// 2. Get order
	getResp, getBody := get(t, "/api/v1/orders/"+order.ID)
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	var fetchedOrder OrderResponse
	err = json.Unmarshal(getBody, &fetchedOrder)
	require.NoError(t, err)
	assert.Equal(t, order.ID, fetchedOrder.ID)

	// 3. Update status: pending -> confirmed
	statusResp, statusBody := patch(t, "/api/v1/orders/"+order.ID+"/status", UpdateStatusRequest{Status: "confirmed"})
	require.Equal(t, http.StatusOK, statusResp.StatusCode)

	var confirmedOrder OrderResponse
	err = json.Unmarshal(statusBody, &confirmedOrder)
	require.NoError(t, err)
	assert.Equal(t, "confirmed", confirmedOrder.Status)
	assert.Equal(t, 2, confirmedOrder.Version)

	// 4. Update status: confirmed -> processing
	statusResp, statusBody = patch(t, "/api/v1/orders/"+order.ID+"/status", UpdateStatusRequest{Status: "processing"})
	require.Equal(t, http.StatusOK, statusResp.StatusCode)

	var processingOrder OrderResponse
	err = json.Unmarshal(statusBody, &processingOrder)
	require.NoError(t, err)
	assert.Equal(t, "processing", processingOrder.Status)
	assert.Equal(t, 3, processingOrder.Version)

	// 5. List orders and verify it's included
	listResp, listBody := get(t, "/api/v1/orders?status=processing&limit=100")
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listResult ListOrdersResponse
	err = json.Unmarshal(listBody, &listResult)
	require.NoError(t, err)

	found := false
	for _, o := range listResult.Orders {
		if o.ID == order.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "Order should appear in filtered list")

	// 6. Delete order
	deleteResp, _ := delete(t, "/api/v1/orders/"+order.ID)
	assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode)

	// 7. Verify deleted
	verifyResp, _ := get(t, "/api/v1/orders/"+order.ID)
	assert.Equal(t, http.StatusNotFound, verifyResp.StatusCode)
}
