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

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nsridhar76/go-ordersvc/internal/config"
	"github.com/nsridhar76/go-ordersvc/internal/domain"
	httpHandler "github.com/nsridhar76/go-ordersvc/internal/handler/http"
	"github.com/nsridhar76/go-ordersvc/internal/repository"
	"github.com/nsridhar76/go-ordersvc/internal/service"
)

// inMemoryRepository is a temporary in-memory implementation for testing
type inMemoryRepository struct {
	mu     sync.RWMutex
	orders map[string]*domain.Order
}

func newInMemoryRepository() *inMemoryRepository {
	return &inMemoryRepository{
		orders: make(map[string]*domain.Order),
	}
}

func (r *inMemoryRepository) Create(_ context.Context, order *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	order.Version = 1
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()
	r.orders[order.ID.String()] = order
	return nil
}

func (r *inMemoryRepository) FindByID(_ context.Context, id string) (*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, ok := r.orders[id]
	if !ok || order.DeletedAt != nil {
		return nil, domain.ErrOrderNotFound
	}
	// Return a copy to prevent mutation
	orderCopy := *order
	return &orderCopy, nil
}

func (r *inMemoryRepository) Update(_ context.Context, order *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.orders[order.ID.String()]
	if !ok || existing.DeletedAt != nil {
		return domain.ErrOrderNotFound
	}

	// Check version for optimistic locking (ADR-0003)
	if existing.Version != order.Version {
		return domain.ErrConcurrentModification
	}

	order.Version++
	order.UpdatedAt = time.Now()
	r.orders[order.ID.String()] = order
	return nil
}

func (r *inMemoryRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[id]
	if !ok || order.DeletedAt != nil {
		return domain.ErrOrderNotFound
	}

	now := time.Now()
	order.DeletedAt = &now
	return nil
}

func (r *inMemoryRepository) List(_ context.Context, opts repository.ListOptions) ([]*domain.Order, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*domain.Order
	for _, order := range r.orders {
		if order.DeletedAt != nil {
			continue
		}
		if opts.Status != nil && order.Status != *opts.Status {
			continue
		}
		result = append(result, order)
	}

	total := int64(len(result))

	// Apply pagination
	start := opts.Offset
	if start > len(result) {
		start = len(result)
	}
	end := start + opts.Limit
	if end > len(result) {
		end = len(result)
	}

	return result[start:end], total, nil
}

func (r *inMemoryRepository) FindByCustomerID(_ context.Context, customerID string, opts repository.ListOptions) ([]*domain.Order, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*domain.Order
	for _, order := range r.orders {
		if order.DeletedAt != nil {
			continue
		}
		if order.CustomerID != customerID {
			continue
		}
		if opts.Status != nil && order.Status != *opts.Status {
			continue
		}
		result = append(result, order)
	}

	total := int64(len(result))
	return result, total, nil
}

// stubHealthChecker is a temporary stub for health checks
type stubHealthChecker struct{}

func (s *stubHealthChecker) Ping(_ context.Context) error {
	return nil
}

// Server holds the HTTP server and its dependencies
type Server struct {
	httpServer *http.Server
	cfg        *config.Config
	logger     *slog.Logger
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config) *Server {
	// Setup structured JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// TODO: Initialize database connection and repository
	// For now, using in-memory repository for testing
	repo := newInMemoryRepository()
	healthChecker := &stubHealthChecker{}

	// Create service
	orderService := service.NewOrderService(repo, nil)

	// Create handlers
	orderHandler := httpHandler.NewOrderHandler(orderService)
	healthHandler := httpHandler.NewHealthHandler(cfg.App.Version, healthChecker)

	// Create router with logger
	router := httpHandler.NewRouter(orderHandler, healthHandler, logger)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	return &Server{
		httpServer: httpServer,
		cfg:        cfg,
		logger:     logger,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", slog.Int("port", s.cfg.Server.HTTPPort))
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down server")
	return s.httpServer.Shutdown(ctx)
}

// Run starts the server and handles graceful shutdown
func Run(cfg *config.Config) error {
	server := NewServer(cfg)

	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			server.logger.Error("server error", slog.String("error", err.Error()))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	return server.Shutdown(ctx)
}
