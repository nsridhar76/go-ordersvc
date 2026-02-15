# ADR-0004: Redis Order Caching

## Status

**Accepted**

**Date:** 2026-02-15

## Business Context

### Problem Statement
The order details feature (ADR-0002) queries PostgreSQL on every `GET /api/v1/orders/{id}` request. As customer self-service adoption grows (ORD-100), repeat lookups of the same order (status checks, tracking page refreshes) create unnecessary database load. Customers checking "where is my order" will poll the same order ID multiple times.

### Success Criteria
- Reduce database queries for order lookups by 60%+ through cache hits
- p95 response time for GET order by ID under 10ms (vs ~50ms with DB)
- Zero customer-visible errors when Redis is unavailable
- Cache stays consistent with database after status updates

### Constraints
- Must work with existing Redis 7 infrastructure (already deployed)
- Must not change API behavior or response format
- Must follow Clean Architecture (ADR-0001) layer boundaries
- Must not block requests when Redis is down

## Options Considered

### Option 1: Cache at Handler Layer
**Pros:**
- Simple HTTP-level caching
- Can use HTTP cache headers

**Cons:**
- Violates Clean Architecture (handler has infrastructure dependency)
- Cannot share cache across HTTP and gRPC handlers
- Handler knows about caching strategy (wrong layer)

**Estimated Effort:** S

### Option 2: Cache at Service Layer (Read-Through with Invalidation)
**Pros:**
- Follows Clean Architecture — service depends on cache interface
- Shared across HTTP and gRPC handlers
- Cache logic encapsulated with business logic
- Can selectively cache (skip 404s, invalidate on mutations)

**Cons:**
- Service has additional dependency (cache interface)
- Cache invalidation logic lives in service methods

**Estimated Effort:** S

### Option 3: Cache at Repository Layer (Decorator Pattern)
**Pros:**
- Transparent to service layer
- Repository interface unchanged

**Cons:**
- Repository layer shouldn't make caching decisions (business concern)
- Hard to skip caching for specific operations
- Couples data access with caching policy

**Estimated Effort:** M

## Decision

**We will use service-layer caching (Option 2) with read-through pattern and explicit invalidation.**

### Rationale
- Service layer is the right place for caching decisions because cache TTL and invalidation are business logic concerns (e.g., "invalidate when status changes")
- Depends on `cache.OrderCache` interface, not Redis implementation directly
- Already partially implemented: `GetOrderByID` reads cache first, `UpdateOrderStatus` invalidates cache
- Follows ADR-0001 dependency flow: handler -> service -> cache interface -> redis implementation

## Constraints

> **CRITICAL: All CONSTRAINT blocks are enforceable rules. Violations will break the build.**

### CONSTRAINT: Caching Must Be at Service Layer, Not Handler
**BECAUSE:** Handlers handle HTTP/gRPC concerns only (ADR-0001). Caching is a business optimization that must be shared across transport layers. Importing cache or Redis packages in the handler layer violates Clean Architecture and makes handlers impossible to test without cache infrastructure.

**CHECK:** Verify no cache or Redis imports in handler layer:
`grep -rn "internal/cache\|go-redis\|redis" internal/handler/ && echo "FAIL" || echo "PASS"`

**Example:**
```go
// Good: Service layer owns caching
package service

import "github.com/nsridhar76/go-ordersvc/internal/cache"

type orderServiceImpl struct {
    repo  repository.OrderRepository
    cache cache.OrderCache
}

// Bad: Handler importing cache directly
package http

import "github.com/nsridhar76/go-ordersvc/internal/cache/redis"

type OrderHandler struct {
    cache *redis.OrderCache // Handler should not know about caching!
}
```

### CONSTRAINT: TTL Must Be Configurable via CACHE_TTL_SECONDS Env Var
**BECAUSE:** Different environments need different TTLs. Production may need 5 minutes while dev needs 30 seconds. Hardcoded TTLs require code changes and redeployment to tune cache behavior.

**CHECK:** Verify CACHE_TTL_SECONDS is read from environment in config:
`grep -rn "CACHE_TTL_SECONDS" internal/config/config.go && echo "PASS" || echo "FAIL"`

**Example:**
```go
// Good: TTL from environment
Cache: CacheConfig{
    DefaultTTL: getEnvAsDuration("CACHE_TTL_SECONDS", 300),
},

// Bad: Hardcoded TTL
const orderCacheTTL = 5 * time.Minute
```

### CONSTRAINT: Redis Failure Must Fall Back to DB Silently
**BECAUSE:** Redis is a performance optimization, not a correctness requirement. If Redis is down, the system must continue serving requests from the database. Cache errors must be logged as warnings but never returned to the caller.

**CHECK:** Verify cache errors are logged but not returned in GetOrderByID:
`grep -A2 "cache.*Get\|cache.*Set\|cache.*Delete" internal/service/order_service_impl.go | grep -q "slog.Warn" && echo "PASS" || echo "FAIL"`

**Example:**
```go
// Good: Log and continue
cached, err := s.cache.Get(ctx, id)
if err != nil {
    slog.Warn("cache get failed", slog.String("order_id", id), slog.String("error", err.Error()))
}

// Bad: Return cache error to caller
cached, err := s.cache.Get(ctx, id)
if err != nil {
    return nil, fmt.Errorf("cache error: %w", err) // Breaks on Redis outage!
}
```

### CONSTRAINT: 404s Must Not Be Cached
**BECAUSE:** Caching "not found" results would cause newly created orders to appear missing until the negative cache entry expires. This creates a confusing user experience where an order is created successfully but cannot be retrieved.

**CHECK:** Verify cache is only populated after successful DB lookup (order != nil):
`grep -B5 "cache.Set" internal/service/order_service_impl.go | grep -q "order == nil" || grep -B10 "cache.Set" internal/service/order_service_impl.go | grep -q "ErrOrderNotFound" && echo "PASS" || echo "FAIL"`

**Example:**
```go
// Good: Only cache found orders
order, err := s.repo.FindByID(ctx, id)
if order == nil {
    return nil, domain.ErrOrderNotFound  // Return 404, don't cache
}
// Only cache after confirmed existence
s.cache.Set(ctx, order, ttl)

// Bad: Caching before nil check
result, _ := s.repo.FindByID(ctx, id)
s.cache.Set(ctx, result, ttl) // Caches nil!
```

### CONSTRAINT: Cache Must Invalidate on UpdateOrderStatus
**BECAUSE:** Order status is the most frequently viewed field (customers checking "where is my order"). Stale status in cache means a customer sees "pending" after their order shipped. Cache invalidation on status change ensures customers always see the latest status.

**CHECK:** Verify cache.Delete is called in UpdateOrderStatus after successful repo update:
`grep -A20 "func.*UpdateOrderStatus" internal/service/order_service_impl.go | grep -q "cache.*Delete" && echo "PASS" || echo "FAIL"`

**Example:**
```go
// Good: Invalidate after successful update
if err := s.repo.Update(ctx, order); err != nil {
    return nil, err
}
if s.cache != nil {
    if err := s.cache.Delete(ctx, id); err != nil {
        slog.Warn("cache delete failed", ...)
    }
}

// Bad: No invalidation
if err := s.repo.Update(ctx, order); err != nil {
    return nil, err
}
return order, nil // Stale cache!
```

## Consequences

### Positive
- Database load reduced for repeat order lookups
- Sub-10ms response time for cache hits
- Zero downtime when Redis is unavailable (graceful degradation)
- Cache consistency maintained through explicit invalidation

### Negative
- Additional infrastructure dependency (Redis)
- Cache invalidation adds complexity to mutation methods
- Brief staleness window between DB write and cache delete (acceptable for read-your-writes within same request)

### Mitigations
- **Redis outage:** Silent fallback to database with warning logs
- **Staleness:** Short TTL (configurable) + explicit invalidation on mutations
- **Complexity:** Cache logic contained entirely in service layer, testable with mock

## Traceability

### Related Work
- **Jira Ticket:** ORD-100 (Customer order details)
- **Pull Request:** TBD
- **Parent ADR:** ADR-0001 (Clean Architecture — layer boundaries)
- **Related ADRs:** ADR-0002 (Order Details API — the feature being optimized)

### Implementation Subtasks

- [x] **Task 1:** Define OrderCache interface
  - **Acceptance:** Interface in `internal/cache/order_cache.go`
  - **Status:** Done

- [x] **Task 2:** Implement Redis cache
  - **Acceptance:** Implementation in `internal/cache/redis/`
  - **Status:** Done

- [x] **Task 3:** Integrate cache in service layer
  - **Acceptance:** `GetOrderByID` reads cache first, `UpdateOrderStatus` invalidates
  - **Status:** Done

- [ ] **Task 4:** Make TTL configurable via CACHE_TTL_SECONDS
  - **Acceptance:** `config.LoadFromEnv()` reads CACHE_TTL_SECONDS
  - **Status:** Not Started

- [ ] **Task 5:** Wire Redis client in server.go
  - **Acceptance:** Cache passed to `NewOrderService(repo, cache)`
  - **Status:** Not Started (currently nil)

- [ ] **Task 6:** Add drift-check rules
  - **Acceptance:** `make drift-check` verifies caching constraints
  - **Status:** Not Started

### Updates
- **2026-02-15:** Initial acceptance
