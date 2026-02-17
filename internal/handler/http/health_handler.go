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

// Package http provides HTTP handlers for the order service API.
package http //nolint:revive // intentional: matches handler layer convention

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HealthChecker defines the interface for checking service health
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// HealthHandler handles health check endpoints
// CONSTRAINT: Health endpoints must not require authentication (ADR-0002)
type HealthHandler struct {
	version   string
	dbChecker HealthChecker
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(version string, dbChecker HealthChecker) *HealthHandler {
	return &HealthHandler{
		version:   version,
		dbChecker: dbChecker,
	}
}

// Healthz handles liveness probe GET /healthz
// Returns 200 if server is alive
func (h *HealthHandler) Healthz(w http.ResponseWriter, _ *http.Request) {
	response := HealthResponse{
		Status:  "ok",
		Version: h.version,
		Checks: map[string]string{
			"server": "ok",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}

// Readyz handles readiness probe GET /readyz
// Returns 200 if all dependencies are healthy, 503 otherwise
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string)
	allHealthy := true

	// Check database connection
	if h.dbChecker != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := h.dbChecker.Ping(ctx); err != nil {
			checks["database"] = "unhealthy: " + err.Error()
			allHealthy = false
		} else {
			checks["database"] = "ok"
		}
	} else {
		checks["database"] = "not configured"
	}

	status := "ok"
	httpStatus := http.StatusOK
	if !allHealthy {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	response := HealthResponse{
		Status:  status,
		Version: h.version,
		Checks:  checks,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}
