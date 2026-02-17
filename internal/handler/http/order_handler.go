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

package http //nolint:revive // intentional: matches handler layer convention

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/nsridhar76/go-ordersvc/internal/domain"
	"github.com/nsridhar76/go-ordersvc/internal/service"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// OrderHandler handles HTTP requests for order operations
type OrderHandler struct {
	service service.OrderService
}

// NewOrderHandler creates a new order handler
func NewOrderHandler(svc service.OrderService) *OrderHandler {
	return &OrderHandler{
		service: svc,
	}
}

// CreateOrder handles POST /api/v1/orders
// CONSTRAINT: Returns 201 + Location header (ADR-0002)
func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if req.CustomerID == "" {
		writeError(w, http.StatusBadRequest, "customer_id is required", "MISSING_CUSTOMER_ID")
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "items are required", "MISSING_ITEMS")
		return
	}

	dto := service.CreateOrderDTO{
		CustomerID: req.CustomerID,
		Items:      MapRequestToOrderItems(req.Items),
	}

	order, err := h.service.CreateOrder(r.Context(), dto)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", fmt.Sprintf("/api/v1/orders/%s", order.ID.String()))
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(MapOrderToResponse(order)); err != nil {
		// Log error but response headers already sent
		return
	}
}

// GetOrder handles GET /api/v1/orders/{id}
// CONSTRAINT: Returns 404 for missing orders (ADR-0002)
func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "order ID is required", "MISSING_ID")
		return
	}

	order, err := h.service.GetOrderByID(r.Context(), id)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(MapOrderToResponse(order)); err != nil {
		return
	}
}

// ListOrders handles GET /api/v1/orders
// Supports ?status=pending&limit=20&offset=0
func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limit := parseIntParam(r, "limit", defaultLimit)
	if limit > maxLimit {
		limit = maxLimit
	}
	if limit < 1 {
		limit = defaultLimit
	}

	offset := parseIntParam(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}

	// Convert limit/offset to page/pageSize for service layer
	page := (offset / limit) + 1
	pageSize := limit

	// Parse status filter
	var status *domain.OrderStatus
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		s := domain.OrderStatus(statusStr)
		status = &s
	}

	// Parse customer_id filter
	var customerID *string
	if cid := r.URL.Query().Get("customer_id"); cid != "" {
		customerID = &cid
	}

	req := service.ListOrdersRequest{
		Page:       page,
		PageSize:   pageSize,
		Status:     status,
		CustomerID: customerID,
	}

	result, err := h.service.ListOrders(r.Context(), req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response := ListOrdersResponse{
		Orders: MapOrdersToResponse(result.Data),
		Total:  result.TotalCount,
		Limit:  limit,
		Offset: offset,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}

// UpdateOrderStatus handles PATCH /api/v1/orders/{id}/status
// Returns 200 on success, 400 for invalid transitions, 404 for missing, 409 for conflicts
func (h *OrderHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "order ID is required", "MISSING_ID")
		return
	}

	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if req.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required", "MISSING_STATUS")
		return
	}

	newStatus := domain.OrderStatus(req.Status)

	order, err := h.service.UpdateOrderStatus(r.Context(), id, newStatus)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(MapOrderToResponse(order)); err != nil {
		return
	}
}

// UpdateOrder handles PUT /api/v1/orders/{id}
func (h *OrderHandler) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "order ID is required", "MISSING_ID")
		return
	}

	var req UpdateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "items are required", "MISSING_ITEMS")
		return
	}

	dto := service.UpdateOrderDTO{
		Items: MapRequestToOrderItems(req.Items),
	}

	order, err := h.service.UpdateOrder(r.Context(), id, dto)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(MapOrderToResponse(order)); err != nil {
		return
	}
}

// DeleteOrder handles DELETE /api/v1/orders/{id}
func (h *OrderHandler) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "order ID is required", "MISSING_ID")
		return
	}

	if err := h.service.DeleteOrder(r.Context(), id); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RegisterRoutes registers all order routes on the router
// CONSTRAINT: All endpoints must use /api/v1 prefix (ADR-0002)
func (h *OrderHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/orders", func(r chi.Router) {
		r.Post("/", h.CreateOrder)
		r.Get("/", h.ListOrders)
		r.Get("/{id}", h.GetOrder)
		r.Put("/{id}", h.UpdateOrder)
		r.Delete("/{id}", h.DeleteOrder)
		r.Patch("/{id}/status", h.UpdateOrderStatus)
	})
}

// Helper functions

func parseIntParam(r *http.Request, name string, defaultVal int) int {
	str := r.URL.Query().Get(name)
	if str == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(str)
	if err != nil {
		return defaultVal
	}
	return val
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := ErrorResponse{
		Error: message,
		Code:  code,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrOrderNotFound):
		writeError(w, http.StatusNotFound, "order not found", "ORDER_NOT_FOUND")
	case errors.Is(err, domain.ErrInvalidTransition):
		writeError(w, http.StatusBadRequest, "invalid status transition", "INVALID_TRANSITION")
	case errors.Is(err, domain.ErrConcurrentModification):
		writeError(w, http.StatusConflict, "order was modified by another process", "CONCURRENT_MODIFICATION")
	case errors.Is(err, domain.ErrInvalidCustomerID):
		writeError(w, http.StatusBadRequest, "invalid customer ID", "INVALID_CUSTOMER_ID")
	case errors.Is(err, domain.ErrNoItems):
		writeError(w, http.StatusBadRequest, "order must have at least one item", "NO_ITEMS")
	case errors.Is(err, domain.ErrOrderAlreadyDeleted):
		writeError(w, http.StatusNotFound, "order not found", "ORDER_NOT_FOUND")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error", "INTERNAL_ERROR")
	}
}
