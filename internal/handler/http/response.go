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

import "time"

// OrderResponse represents an order in HTTP responses
type OrderResponse struct {
	ID         string              `json:"id"`
	CustomerID string              `json:"customer_id"`
	Items      []OrderItemResponse `json:"items"`
	Status     string              `json:"status"`
	Total      float64             `json:"total"`
	Version    int                 `json:"version"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// OrderItemResponse represents an item in an order response
type OrderItemResponse struct {
	ID        string  `json:"id"`
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
	Subtotal  float64 `json:"subtotal"`
}

// ListOrdersResponse represents a paginated list of orders (ADR-0002 format)
type ListOrdersResponse struct {
	Orders []OrderResponse `json:"orders"`
	Total  int64           `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status  string            `json:"status"`
	Checks  map[string]string `json:"checks"`
	Version string            `json:"version"`
}
