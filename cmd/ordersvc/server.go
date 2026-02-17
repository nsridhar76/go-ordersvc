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
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nsridhar76/go-ordersvc/internal/cache/redis"
	"github.com/nsridhar76/go-ordersvc/internal/config"
	grpcHandler "github.com/nsridhar76/go-ordersvc/internal/handler/grpc"
	httpHandler "github.com/nsridhar76/go-ordersvc/internal/handler/http"
	kafkapub "github.com/nsridhar76/go-ordersvc/internal/messaging/kafka"
	"github.com/nsridhar76/go-ordersvc/internal/messaging/noop"
	"github.com/nsridhar76/go-ordersvc/internal/repository/postgres"
	"github.com/nsridhar76/go-ordersvc/internal/service"
	"google.golang.org/grpc"
)

// pgHealthChecker adapts pgxpool.Pool to the HealthChecker interface
type pgHealthChecker struct {
	pool *pgxpool.Pool
}

func (h *pgHealthChecker) Ping(ctx context.Context) error {
	return h.pool.Ping(ctx)
}

// Server holds the HTTP server and its dependencies
type Server struct {
	httpServer  *http.Server
	grpcServer  *grpc.Server
	cfg         *config.Config
	logger      *slog.Logger
	dbPool      *pgxpool.Pool
	redisCloser func() error
	kafkaCloser func() error
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config) *Server {
	// Setup structured JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Initialize PostgreSQL connection pool
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
		cfg.Database.SSLMode,
	)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		logger.Error("failed to parse database config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	poolCfg.MaxConns = safeInt32(cfg.Database.MaxOpenConns)
	poolCfg.MinConns = safeInt32(cfg.Database.MaxIdleConns)
	poolCfg.MaxConnLifetime = cfg.Database.ConnMaxLifetime
	poolCfg.MaxConnIdleTime = cfg.Database.ConnMaxIdleTime

	dbPool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		logger.Error("failed to create database pool", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := dbPool.Ping(context.Background()); err != nil {
		logger.Error("failed to ping database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("connected to PostgreSQL", slog.String("host", cfg.Database.Host), slog.Int("port", cfg.Database.Port))

	// Initialize Redis client
	redisClient, err := redis.NewClient(redis.Config{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		logger.Error("failed to connect to Redis", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("connected to Redis", slog.String("host", cfg.Redis.Host), slog.Int("port", cfg.Redis.Port))

	// Initialize event publisher
	var publisher service.EventPublisher
	var kafkaCloser func() error
	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		kp := kafkapub.NewPublisher(cfg.Kafka.Brokers, cfg.Kafka.Topic)
		publisher = kp
		kafkaCloser = kp.Close
		logger.Info("Kafka publisher initialized", slog.Any("brokers", cfg.Kafka.Brokers), slog.String("topic", cfg.Kafka.Topic))
	} else {
		publisher = noop.Publisher{}
		logger.Info("Kafka not configured, using no-op publisher")
	}

	// Create repository and cache
	repo := postgres.NewOrderRepository(dbPool)
	orderCache := redis.NewOrderCache(redisClient)

	// Create service
	orderService := service.NewOrderService(repo, orderCache, publisher)

	// Create HTTP handlers
	orderHandler := httpHandler.NewOrderHandler(orderService)
	healthHandler := httpHandler.NewHealthHandler(cfg.App.Version, &pgHealthChecker{pool: dbPool})

	// Create router with logger
	router := httpHandler.NewRouter(orderHandler, healthHandler, logger)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Create gRPC server
	grpcSrv := grpc.NewServer()
	grpcHandler.RegisterOrderServer(grpcSrv, orderService, cfg.Kafka)

	return &Server{
		httpServer:  httpServer,
		grpcServer:  grpcSrv,
		cfg:         cfg,
		logger:      logger,
		dbPool:      dbPool,
		redisCloser: redisClient.Close,
		kafkaCloser: kafkaCloser,
	}
}

// Start starts the HTTP and gRPC servers
func (s *Server) Start() error {
	// Start gRPC server in background
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Server.GRPCPort))
		if err != nil {
			s.logger.Error("failed to listen for gRPC", slog.String("error", err.Error()))
			return
		}
		s.logger.Info("starting gRPC server", slog.Int("port", s.cfg.Server.GRPCPort))
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error("gRPC server error", slog.String("error", err.Error()))
		}
	}()

	s.logger.Info("starting HTTP server", slog.Int("port", s.cfg.Server.HTTPPort))
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server and closes connections
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down server")

	if s.grpcServer != nil {
		s.logger.Info("stopping gRPC server")
		s.grpcServer.GracefulStop()
	}

	err := s.httpServer.Shutdown(ctx)

	if s.dbPool != nil {
		s.logger.Info("closing database connection pool")
		s.dbPool.Close()
	}

	if s.redisCloser != nil {
		s.logger.Info("closing Redis connection")
		if redisErr := s.redisCloser(); redisErr != nil {
			s.logger.Error("failed to close Redis", slog.String("error", redisErr.Error()))
		}
	}

	if s.kafkaCloser != nil {
		s.logger.Info("closing Kafka publisher")
		if kafkaErr := s.kafkaCloser(); kafkaErr != nil {
			s.logger.Error("failed to close Kafka publisher", slog.String("error", kafkaErr.Error()))
		}
	}

	return err
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

// safeInt32 converts int to int32 with clamping to prevent overflow.
func safeInt32(v int) int32 {
	const maxInt32 = 1<<31 - 1
	if v > maxInt32 {
		return maxInt32
	}
	if v < 0 {
		return 0
	}
	return int32(v) // #nosec G115 -- bounds checked above
}
