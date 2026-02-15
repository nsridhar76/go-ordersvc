# Review: Architecture Review Beyond Linting

## Purpose

Perform deep architecture review catching issues that linters miss: wrong abstraction boundaries, business logic errors, context.Context misuse, and Clean Architecture violations.

## When to Use

Use this skill when:
- Before opening a PR for review
- After implementing a feature across multiple layers
- When refactoring service or repository boundaries
- When reviewing code that handles context propagation, concurrency, or transactions

## Prerequisite

**`make lint` must pass before this skill runs.**

Run `make lint` first. If it fails, fix all lint errors before proceeding. This skill focuses on concerns that static analysis cannot catch.

## Process

### Step 1: Verify Prerequisite

Run `make lint` and confirm it passes with zero errors. If it fails, stop and report the lint failures to the user. Do not proceed with architecture review until linting is clean.

### Step 2: Identify Changed Files

Determine the scope of review:
- Use `git diff --name-only` (against main or the relevant base branch) to find changed files
- If no diff context, ask the user which files or packages to review

### Step 3: Check ADR Constraints

1. Read all ADRs in `docs/decisions/*.md`
2. Extract every CONSTRAINT block
3. Verify each changed file complies with applicable constraints
4. Flag any violations immediately

### Step 4: Review Abstraction Boundaries

Check for Clean Architecture violations:

- **handler importing service internals:** Handlers should only use service interfaces and DTOs, never repo types or domain internals directly
- **service importing repo implementations:** Services must depend on repository interfaces, never concrete implementations
- **repo containing business logic:** Repository layer should only do data access — conditionals, calculations, and rules belong in the service layer
- **domain depending on infrastructure:** Domain entities must have zero imports from handler, service, repo, or external libraries
- **Cross-layer type leakage:** Database models (sqlx/pgx tags) appearing in handler responses, or HTTP request types reaching the repo layer

### Step 5: Review Business Logic

Look for logic errors in the service layer:

- **Missing validation:** Business rules not enforced before state mutations
- **Incorrect state transitions:** Order status changes that skip required intermediate states
- **Silent data loss:** Updates that overwrite fields without checking existing values
- **Race conditions:** Shared state accessed without synchronization or optimistic locking
- **Broken invariants:** Operations that leave domain objects in an inconsistent state

### Step 6: Review context.Context Usage

Check for common context.Context misuse:

- **Storing business data in context:** Context should carry deadlines, cancellation signals, and request-scoped values (trace IDs, auth tokens) — not business entities or DTOs
- **Ignoring context cancellation:** Long-running operations (DB queries, HTTP calls, Kafka publishes) that don't pass or check context
- **Background context in request paths:** Using `context.Background()` or `context.TODO()` inside HTTP/gRPC handlers instead of the request context
- **Context value type keys:** Using string keys for context values instead of unexported typed keys (risks collision)
- **Missing context propagation:** Functions that accept context but don't forward it to downstream calls

### Step 7: Review Error Handling

Check for error handling patterns beyond what linters catch:

- **Swallowed errors:** Errors caught but not logged, returned, or acted upon
- **Wrong error wrapping:** Using `fmt.Errorf` without `%w` verb, breaking `errors.Is`/`errors.As` chains
- **Sentinel error misuse:** Comparing errors with `==` instead of `errors.Is`
- **Error type confusion:** Returning infrastructure errors (pgx, redis) directly to handlers instead of translating to domain errors
- **Missing error context:** Errors returned without enough information to diagnose the failure location

### Step 8: Review Concurrency Patterns

If the code uses goroutines, channels, or shared state:

- **Goroutine leaks:** Goroutines started without cancellation or shutdown mechanism
- **Unbounded goroutines:** Missing semaphores or worker pools for fan-out operations
- **Channel misuse:** Unbuffered channels causing unexpected blocking, or channels never closed
- **sync.Mutex scope:** Locks held across I/O operations or external calls

## Output Format

Present findings as a markdown table:

```markdown
## Architecture Review Results

**Prerequisite:** `make lint` passed

| File | Concern | Severity | Recommendation |
|------|---------|----------|----------------|
| `internal/order/service.go:45` | Repo implementation imported directly instead of interface | HIGH | Depend on `OrderRepository` interface defined in service package |
| `internal/order/handler.go:92` | `context.Background()` used in request handler | HIGH | Use `r.Context()` from the HTTP request |
| `internal/order/service.go:78` | Order status changes from "pending" to "shipped" skipping "confirmed" | HIGH | Enforce state machine: pending -> confirmed -> shipped |
| `internal/order/repo.go:31` | Business rule (discount calculation) in repository layer | MEDIUM | Move discount logic to service layer |
| `internal/order/handler.go:55` | pgx error type leaked to HTTP response | MEDIUM | Map to domain error, return appropriate HTTP status |
| `internal/order/service.go:120` | Error wrapped without `%w`, breaks `errors.Is` chain | LOW | Use `fmt.Errorf("...: %w", err)` |
```

### Severity Levels

- **HIGH:** Violates Clean Architecture boundaries, breaks correctness, or causes data loss/corruption
- **MEDIUM:** Wrong layer responsibility, poor error handling, or context misuse that could cause issues under load
- **LOW:** Style/convention issues beyond linter scope, minor improvements to error messages or naming

## Notes

- This review complements `make lint` — never duplicate what golangci-lint already catches
- Always cross-reference findings against ADR constraints
- If a HIGH severity finding involves a CONSTRAINT violation, reference the specific ADR
- Focus review effort on the service layer where business logic lives
- When in doubt about severity, err on the side of higher severity
