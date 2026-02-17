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
	"log/slog"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/nsridhar76/go-ordersvc/internal/middleware"
)

// NewRouter creates a new Chi router with all routes configured
// CONSTRAINT: Health endpoints must not require authentication (ADR-0002)
func NewRouter(orderHandler *OrderHandler, healthHandler *HealthHandler, logger *slog.Logger) *chi.Mux {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logging(logger))
	r.Use(chimiddleware.Recoverer)

	// Health checks (outside any auth middleware)
	r.Get("/healthz", healthHandler.Healthz)
	r.Get("/readyz", healthHandler.Readyz)

	// Order routes with /api/v1 prefix
	orderHandler.RegisterRoutes(r)

	return r
}
