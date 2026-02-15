# Refine: Jira Ticket → Implementation Plan + ADR

## Purpose

Translate business requirements from Jira tickets into technical implementation plans with Architecture Decision Records (ADRs).

## When to Use

Use this skill when:
- Starting work on a new Jira ticket
- User provides a business requirement without technical details
- Planning a feature that involves architectural decisions
- Breaking down a large story into implementable tasks

## Process

### Phase 1: Understand Business Outcome

1. **Extract business context** from the Jira ticket
   - What problem does this solve for users/customers?
   - What is the success criteria?
   - What are the business constraints (timeline, budget, compliance)?

2. **Clarify ambiguities** using AskUserQuestion
   - Missing acceptance criteria
   - Unclear business rules
   - Edge cases not covered in the ticket

3. **Identify affected domains**
   - Which parts of the system need to change?
   - What are the integration points?

### Phase 2: Analyze Codebase

1. **Search for existing patterns**
   - Use Grep/Glob to find similar implementations
   - Check how related features are structured
   - Identify reusable components

2. **Check existing ADRs**
   - Read `docs/decisions/*.md` for related decisions
   - Extract all CONSTRAINT rules that apply
   - Flag any constraints that might conflict with requirements

3. **Review architecture boundaries**
   - Verify which layers (handler/service/repo) need changes
   - Check if changes stay within Clean Architecture rules
   - Identify if new interfaces are needed

### Phase 3: Propose Options

**Generate 2-3 technical approaches:**

For each option, document:
- **Description:** High-level approach
- **Pros:** Benefits (performance, simplicity, maintainability)
- **Cons:** Drawbacks (complexity, risk, technical debt)
- **Effort:** Estimate as S/M/L/XL
- **Constraints:** List CONSTRAINT rules from existing ADRs that apply
- **Risks:** What could go wrong?

**Example:**

```markdown
### Option 1: Add Status Filter to Existing ListOrders Endpoint
**Pros:**
- Reuses existing pagination logic
- No new endpoint needed
- Simple query parameter

**Cons:**
- Query complexity increases
- Index strategy needs review

**Effort:** S (4-6 hours)

**Constraints:**
- CONSTRAINT from ADR-0001: Must use optimistic locking for updates
- CONSTRAINT from ADR-0005: Max 100 items per page

**Risks:**
- Performance degradation if index missing
```

### Phase 4: Recommend One Option

**Provide a clear recommendation:**

```markdown
## Recommendation: Option 2

**Rationale:**
- Best aligns with existing architecture patterns
- Lowest risk given current constraints
- Meets performance requirements (<100ms)
- Team has experience with this approach

**Trade-offs Accepted:**
- Slightly higher initial effort vs Option 1
- But better long-term maintainability
```

### Phase 5: Break Into Subtasks

**Create actionable subtasks with acceptance criteria:**

```markdown
### Implementation Subtasks

1. **Update Domain Layer**
   - [ ] Add new entity fields
   - [ ] Update validation logic
   - [ ] Add unit tests
   - **Acceptance:** `make test` passes, coverage >80%

2. **Implement Repository**
   - [ ] Add new queries with optimistic locking
   - [ ] Update migrations
   - [ ] Test version conflict handling
   - **Acceptance:** Integration tests pass

3. **Update Service Layer**
   - [ ] Add business logic
   - [ ] Handle concurrent modification errors
   - [ ] Add service tests
   - **Acceptance:** All CONSTRAINT rules verified

4. **Add HTTP Handlers**
   - [ ] Implement endpoints
   - [ ] Add request/response validation
   - [ ] Update OpenAPI spec
   - **Acceptance:** API tests pass, 409 handling works

5. **Documentation**
   - [ ] Update CLAUDE.md if needed
   - [ ] Add API examples
   - [ ] Update Traceability in ADR
   - **Acceptance:** Docs reviewed

6. **Verification**
   - [ ] Run `make ci`
   - [ ] Manual testing against acceptance criteria
   - [ ] Update Jira ticket
   - **Acceptance:** All checks pass
```

### Phase 6: Generate ADR (After Engineer Approval)

**Wait for explicit engineer approval before generating the ADR.**

Once the engineer approves the recommended option:

1. **Determine ADR number**
   - Check existing ADRs in `docs/decisions/`
   - Use next sequential number (e.g., ADR-0004)

2. **Generate ADR file** at `docs/decisions/ADR-NNNN-<slug>.md`:

```markdown
# ADR-NNNN: <Title>

## Status
Accepted

**Date:** <today>

## Business Context

### Problem Statement
<extracted from Jira ticket - what user/business problem are we solving?>

### Success Criteria
<measurable outcomes from the ticket>

### Constraints
<business constraints: timeline, compliance, compatibility>

## Options Considered

### Option 1: <name>
**Pros:**
- ...

**Cons:**
- ...

**Estimated Effort:** S/M/L

### Option 2: <name>
...

## Decision

**We will use <chosen option>.**

### Rationale
<why this option was selected>

## Constraints

> **CRITICAL: All CONSTRAINT blocks are enforceable rules. Violations will break the build.**

### CONSTRAINT: <Rule Name>
**BECAUSE:** <why this rule exists>

**CHECK:** <how to verify compliance - grep command, test, or manual check>

**Example:**
\`\`\`go
// Good:
<compliant code>

// Bad:
<non-compliant code>
\`\`\`

## Consequences

### Positive
- ...

### Negative
- ...

### Mitigations
- ...

## Traceability

### Related Work
- **Jira Ticket:** <link>
- **Pull Request:** TBD
- **Parent ADR:** <if applicable>
- **Related ADRs:** <if applicable>

### Implementation Subtasks
<copy from Phase 5>

### Updates
- **<today>:** Initial acceptance
```

3. **Add CONSTRAINT checks to drift-check** (if new rules added)
   - Update `Makefile` drift-check target with grep patterns
   - Ensure CI catches violations

4. **Link back to Jira**
   - Add ADR link to the Jira ticket
   - Create Jira subtasks from implementation tasks if needed

## Output Format

Provide the analysis in this structure:

```markdown
# Implementation Plan: [Jira Ticket ID] - [Title]

## Business Outcome
[What user value does this create?]

## Codebase Analysis
[Relevant patterns, existing ADRs, constraints]

## Options Considered

### Option 1: [Name]
[Details]

### Option 2: [Name]
[Details]

### Option 3: [Name] (if needed)
[Details]

## Recommendation
[Which option and why]

## ADR Required?
[Yes/No - if yes, which decisions need documentation?]

## Implementation Subtasks
[Numbered checklist with acceptance criteria]

## Constraints to Verify
[List of CONSTRAINT rules from ADRs that apply]

## Questions for Team
[Anything unclear or needing decision?]
```

## Example Usage

**User:** "Implement PROJ-123: Add ability to filter orders by date range"

**Skill Output:**

1. Asks clarifying questions about date range format, timezone handling
2. Searches codebase for existing date filtering patterns
3. Checks ADR-0008 (pagination constraints), ADR-0012 (query performance)
4. Proposes 3 options:
   - Query parameter approach
   - GraphQL-style filter syntax
   - Separate filtered endpoint
5. Recommends query parameter (simplest, matches existing pattern)
6. Breaks into 7 subtasks with acceptance criteria
7. Lists constraints: max 100 items/page, must use indexes
8. Flags question: "Should we support timezone conversion or UTC only?"

## Integration with Workflow

1. User provides Jira ticket
2. Run `/refine` to generate implementation plan
3. Review options with team
4. Create ADR if architectural decision needed
5. Use subtasks as PR checklist
6. Verify CONSTRAINT compliance before merge

## Notes

- Always check `docs/decisions/` before proposing options
- Flag constraint conflicts immediately - don't implement
- Prefer simple options unless complexity is justified
- Keep subtasks small enough for same-day completion
- Link ADR back to Jira ticket in Traceability section
