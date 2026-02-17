# ADR-0006: Event-Driven Architecture via Kafka + gRPC Streaming

## Status

**Accepted**

**Date:** 2026-02-17

## Business Context

### Problem Statement
Warehouse and notification services poll the REST API every 5 seconds, causing 60% of API traffic and up to 5-second delays on order status changes. We need sub-1s event delivery without polling.

### Success Criteria
- Order status changes delivered to consumers within 1 second
- Polling-based API traffic eliminated for warehouse and notification services
- Service starts and operates normally when Kafka is unavailable

### Constraints
- Must use existing Kafka infrastructure (KRaft mode, single broker for dev)
- Must not break existing REST API behavior

## Options Considered

### Option 1: Kafka Events + gRPC Streaming
**Pros:**
- Sub-second delivery via Kafka consumer groups
- gRPC streaming provides real-time push to clients
- Decoupled publishers and consumers

**Cons:**
- Additional infrastructure dependency (Kafka)
- Increased operational complexity

**Estimated Effort:** M

### Option 2: WebSocket Push
**Pros:**
- Simpler infrastructure (no message broker)

**Cons:**
- No durability or replay capability
- Harder to scale horizontally
- No consumer group semantics

**Estimated Effort:** M

### Option 3: Server-Sent Events (SSE)
**Pros:**
- Simple HTTP-based protocol

**Cons:**
- Unidirectional only
- No built-in consumer groups or ordering
- Browser-centric

**Estimated Effort:** S

## Decision

**We will use Kafka Events + gRPC Streaming because it provides durable, ordered event delivery with sub-second latency and supports polyglot consumers.**

### Rationale
Kafka provides message durability, per-partition ordering (keyed by order ID), and consumer group semantics. gRPC server-streaming enables real-time push with status filtering. The combination eliminates polling while maintaining reliability.

## Constraints

> **CRITICAL: All CONSTRAINT blocks are enforceable rules. Violations will break the build.**

### CONSTRAINT: Publish After Successful DB Write
**BECAUSE:** Publishing before a DB write could emit events for operations that ultimately fail, leading to inconsistent state across consumers.

**CHECK:** `make drift-check` verifies publish calls appear after `repo.Create`/`repo.Update` in service methods.

**Example:**
```go
// Good:
if err := s.repo.Create(ctx, order); err != nil {
    return nil, err
}
s.publisher.PublishOrderCreated(ctx, order) // after DB write

// Bad:
s.publisher.PublishOrderCreated(ctx, order) // before DB write!
if err := s.repo.Create(ctx, order); err != nil {
    return nil, err
}
```

### CONSTRAINT: Publish Failures Never Returned to Caller
**BECAUSE:** Event publishing is asynchronous enrichment. A Kafka outage must not prevent order creation, updates, or status changes. Matches the cache failure pattern (ADR-0004).

**CHECK:** `make drift-check` verifies publish errors are logged as warnings, not returned.

**Example:**
```go
// Good:
if err := s.publisher.PublishOrderCreated(ctx, order); err != nil {
    slog.Warn("failed to publish", slog.String("error", err.Error()))
}

// Bad:
if err := s.publisher.PublishOrderCreated(ctx, order); err != nil {
    return nil, err // blocks caller on Kafka failure!
}
```

### CONSTRAINT: Messaging Package Imports Domain Only
**BECAUSE:** The messaging package is infrastructure. It must depend only on domain types to maintain clean architecture boundaries (ADR-0001).

**CHECK:** `make drift-check` scans `internal/messaging/` for forbidden imports (service, handler, repository, cache).

## Consequences

### Positive
- Sub-second event delivery eliminates 60% polling traffic
- Kafka ordering guarantees per-order event sequence
- NoopPublisher allows service to run without Kafka
- gRPC streaming enables real-time UI updates

### Negative
- Additional infrastructure dependency (Kafka broker)
- At-least-once delivery semantics require idempotent consumers

### Neutral
- JSON message format chosen for debuggability over Protobuf efficiency
- Single topic with event_type field vs. multiple topics

### Mitigations
- NoopPublisher ensures service starts without Kafka
- Consumer group per streaming client prevents message loss
- JSON format allows `kafka-console-consumer` debugging

## Traceability

### Related Work
- **Ticket:** ORD-201
- **Parent ADR:** ADR-0001 (clean architecture), ADR-0004 (cache failure pattern)

### Implementation Subtasks
- [x] **Task 1:** Kafka publisher implementing EventPublisher interface
  - **Acceptance:** Unit tests pass with mock publisher
  - **Status:** Done

- [x] **Task 2:** gRPC streaming service with WatchOrders RPC
  - **Acceptance:** grpcurl can stream events filtered by status
  - **Status:** Done

- [x] **Task 3:** Docker Compose with Kafka (KRaft mode)
  - **Acceptance:** `make compose-up` starts full stack including Kafka
  - **Status:** Done

### Updates
- **2026-02-17:** Initial creation
