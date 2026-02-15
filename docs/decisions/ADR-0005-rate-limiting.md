# ADR-0005: API Rate Limiting

## Status

**Accepted**

**Date:** 2026-02-15

## Business Context

### Problem Statement
The public-facing order API (ADR-0002) has no protection against abuse. A single client can send unlimited requests, potentially degrading performance for all customers. Without rate limiting, a misbehaving script or malicious actor could exhaust database connections and Redis capacity, causing downtime.

### Success Criteria
- No single client can degrade service for others
- Rate limits are configurable without code changes
- Clients receive clear feedback when rate limited (status code + retry timing)
- Rate limiting is transparent to well-behaved clients

### Constraints
- Must work with the existing middleware stack (Chi router)
- Must not require per-handler configuration
- Must return standard HTTP 429 responses
- Must be configurable for different environments (stricter in prod, relaxed in dev)

## Options Considered

### Option 1: Per-Handler Rate Limiting
**Pros:**
- Fine-grained control per endpoint
- Different limits for read vs write

**Cons:**
- Duplicated logic in every handler
- Easy to forget on new endpoints
- Violates separation of concerns (handler knows about rate limits)

**Estimated Effort:** M

### Option 2: Middleware-Based Rate Limiting
**Pros:**
- Applied globally — all endpoints protected automatically
- New endpoints get rate limiting for free
- Single configuration point
- Follows Chi middleware pattern (compose, don't repeat)

**Cons:**
- Same limit for all endpoints (can be refined later with per-path middleware)
- Middleware layer has infrastructure dependency (Redis for distributed rate limiting)

**Estimated Effort:** S

### Option 3: API Gateway Rate Limiting
**Pros:**
- No application code changes
- Scales independently

**Cons:**
- Requires additional infrastructure (nginx, Kong, etc.)
- Not available on Docker Desktop dev setup
- Harder to test locally

**Estimated Effort:** L

## Decision

**We will use middleware-based rate limiting (Option 2) applied globally via Chi middleware.**

### Rationale
- Middleware is the standard place for cross-cutting concerns in Chi
- All endpoints protected automatically, including future additions
- Configuration via environment variables matches existing patterns
- Redis-backed for distributed rate limiting across replicas
- Stub already exists in `internal/middleware/rate_limit.go`

## Constraints

> **CRITICAL: All CONSTRAINT blocks are enforceable rules. Violations will break the build.**

### CONSTRAINT: Rate Limiting Must Be at Middleware Layer, Not Per-Handler
**BECAUSE:** Rate limiting is a cross-cutting concern that applies to all endpoints. Implementing it per-handler creates duplication, risks missing new endpoints, and pollutes handlers with infrastructure logic. Middleware ensures consistent protection.

**CHECK:** Verify no rate limiting imports or logic in handler layer:
`grep -rn "rate\|RateLimit\|limiter\|429\|TooManyRequests" internal/handler/ && echo "FAIL" || echo "PASS"`

**Example:**
```go
// Good: Middleware applied in router setup
r := chi.NewRouter()
r.Use(middleware.RateLimit(limiter, cfg))
r.Route("/api/v1/orders", func(r chi.Router) {
    r.Get("/", h.ListOrders)
})

// Bad: Rate limiting in handler
func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
    if !h.limiter.Allow(r) {  // Handler doing middleware's job!
        w.WriteHeader(http.StatusTooManyRequests)
        return
    }
    // ...
}
```

### CONSTRAINT: Rate Limits Must Be Configurable via Env Vars
**BECAUSE:** Different environments need different limits. Production needs strict limits (e.g., 100 req/min) while development and testing need relaxed limits (e.g., 1000 req/min). Hardcoded limits require code changes and redeployment.

**CHECK:** Verify rate limit configuration reads from environment:
`grep -rn "RATE_LIMIT" internal/config/config.go && echo "PASS" || echo "FAIL"`

**Example:**
```go
// Good: From environment
RateLimit: RateLimitConfig{
    RequestsPerMinute: getEnvAsInt("RATE_LIMIT_RPM", 100),
    BurstSize:         getEnvAsInt("RATE_LIMIT_BURST", 20),
},

// Bad: Hardcoded
const maxRequestsPerMinute = 100
```

### CONSTRAINT: 429 Response Must Include Retry-After Header
**BECAUSE:** RFC 6585 specifies that 429 responses SHOULD include a Retry-After header. Without it, clients don't know when to retry and may hammer the API in a tight loop, making the rate limiting less effective. Well-behaved clients use Retry-After to back off.

**CHECK:** Verify Retry-After header is set when returning 429:
`grep -A5 "StatusTooManyRequests\|429" internal/middleware/rate_limit.go | grep -q "Retry-After" && echo "PASS" || echo "FAIL"`

**Example:**
```go
// Good: Retry-After header included
w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
w.WriteHeader(http.StatusTooManyRequests)

// Bad: 429 without Retry-After
w.WriteHeader(http.StatusTooManyRequests) // Client doesn't know when to retry!
```

## Consequences

### Positive
- All endpoints protected from abuse automatically
- Configuration-driven limits adaptable per environment
- Clients receive actionable retry guidance
- New endpoints get rate limiting for free

### Negative
- Redis dependency for distributed rate limiting (shared with caching, ADR-0004)
- Adds latency for rate limit check on every request (~1ms with Redis)
- Global limit may be too coarse for some use cases

### Mitigations
- **Redis outage:** Rate limiter should fail-open (allow requests) to avoid blocking all traffic
- **Latency:** Redis rate limit check is <1ms, negligible
- **Granularity:** Can add per-path middleware later if needed (compose middleware per route group)

## Traceability

### Related Work
- **Jira Ticket:** N/A (security/reliability concern)
- **Pull Request:** TBD
- **Parent ADR:** ADR-0001 (Clean Architecture — middleware layer)
- **Related ADRs:** ADR-0002 (Order Details API — the API being protected), ADR-0004 (Redis Caching — shared Redis dependency)

### Implementation Subtasks

- [x] **Task 1:** Define RateLimiter interface
  - **Acceptance:** Interface in `internal/cache/order_cache.go`
  - **Status:** Done

- [ ] **Task 2:** Implement Redis rate limiter
  - **Acceptance:** Sliding window implementation in `internal/cache/redis/rate_limiter.go`
  - **Status:** Not Started (currently stub)

- [ ] **Task 3:** Implement middleware
  - **Acceptance:** `middleware.RateLimit()` checks limiter and returns 429 + Retry-After
  - **Status:** Not Started (currently passthrough)

- [ ] **Task 4:** Add config for rate limits
  - **Acceptance:** RATE_LIMIT_RPM and RATE_LIMIT_BURST in config
  - **Status:** Not Started

- [ ] **Task 5:** Wire middleware in router
  - **Acceptance:** `r.Use(middleware.RateLimit(...))` in router setup
  - **Status:** Not Started

- [ ] **Task 6:** Add drift-check rules
  - **Acceptance:** `make drift-check` verifies rate limiting constraints
  - **Status:** Not Started

### Updates
- **2026-02-15:** Initial acceptance
