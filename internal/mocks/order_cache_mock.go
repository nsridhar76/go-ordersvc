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

package mocks

import (
	"context"
	"time"

	"github.com/nsridhar76/go-ordersvc/internal/domain"
)

// OrderCacheMock is a mock implementation of OrderCache
type OrderCacheMock struct {
	GetFunc           func(ctx context.Context, id string) (*domain.Order, error)
	SetFunc           func(ctx context.Context, order *domain.Order, ttl time.Duration) error
	DeleteFunc        func(ctx context.Context, id string) error
	DeletePatternFunc func(ctx context.Context, pattern string) error
}

// Get retrieves an order from cache.
func (m *OrderCacheMock) Get(ctx context.Context, id string) (*domain.Order, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	return nil, nil
}

// Set stores an order in cache with TTL.
func (m *OrderCacheMock) Set(ctx context.Context, order *domain.Order, ttl time.Duration) error {
	if m.SetFunc != nil {
		return m.SetFunc(ctx, order, ttl)
	}
	return nil
}

// Delete removes an order from cache.
func (m *OrderCacheMock) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

// DeletePattern removes all keys matching pattern.
func (m *OrderCacheMock) DeletePattern(ctx context.Context, pattern string) error {
	if m.DeletePatternFunc != nil {
		return m.DeletePatternFunc(ctx, pattern)
	}
	return nil
}
