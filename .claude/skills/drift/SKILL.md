# Drift: ADR Constraint Compliance Audit

## Purpose

Audit the current codebase against all CONSTRAINT/CHECK rules defined in accepted Architecture Decision Records. Reports each constraint as PASS, DRIFT, or UNCLEAR. This is a read-only skill — it never modifies any files.

## When to Use

Use this skill when:
- Before opening a PR to verify no ADR constraints are violated
- After implementing a feature that touches multiple layers
- As a periodic health check on architectural compliance
- When `make drift-check` passes but you want deeper verification (the Makefile target runs a subset of grep-based checks; this skill audits all constraints including those requiring semantic analysis)

## Read-Only Guarantee

This skill is strictly read-only. It:
- Reads ADR files in `docs/decisions/`
- Reads source code files to verify compliance
- Runs grep/glob searches against the codebase
- **Never** modifies, creates, or deletes any file
- **Never** runs `make`, `go build`, or any command that changes state

## Process

### Step 1: Discover All Accepted ADRs

1. Glob `docs/decisions/ADR-*.md` to find all ADR files
2. Read each file and check the `## Status` section
3. Only process ADRs with status **Accepted**
4. Skip ADRs with status Proposed, Deprecated, or Superseded

### Step 2: Extract CONSTRAINT Blocks

For each accepted ADR, extract every CONSTRAINT block by parsing:

- **CONSTRAINT name:** The `### CONSTRAINT:` heading text
- **BECAUSE:** The business/technical rationale — this becomes the "business impact" reported for any DRIFT finding
- **CHECK:** The verification method — this is the command or inspection to perform

Build a checklist of all constraints with their ADR source, e.g.:

```
ADR-0001 | Handlers Must Not Import Repository or Database Packages
ADR-0001 | Service Layer Must Depend on Repository Interfaces
ADR-0001 | Repository Layer Must Not Import Handler or Service Packages
ADR-0002 | All Endpoints Must Use /api/v1 Prefix
ADR-0002 | POST Must Return 201 with Location Header
ADR-0003 | All Update Operations Must Check Version
...
```

### Step 3: Execute Each CHECK

For each constraint, execute the CHECK instruction:

**If the CHECK is a grep/shell command:**
- Run the grep pattern against the specified paths using the Grep tool (not Bash)
- Interpret the result according to the CHECK's pass/fail logic

**If the CHECK requires code inspection:**
- Read the relevant source files
- Verify the code matches the constraint's Good example and does not match the Bad example
- Check for semantic compliance, not just pattern matching

**If the CHECK requires test verification:**
- Search for test files that cover the constraint scenario
- Verify test cases exist (do NOT run tests — this is read-only)
- If tests exist and cover the constraint, mark as PASS
- If tests are missing, mark as UNCLEAR with a note

### Step 4: Classify Each Result

Assign one of three verdicts:

**PASS** — The codebase complies with the constraint.
- The CHECK command/inspection confirms compliance
- No violations found in any relevant file

**DRIFT** — The codebase violates the constraint.
- The CHECK command/inspection found a violation
- Include the specific file(s) and line(s) where the violation occurs
- Include the BECAUSE text as the business impact

**UNCLEAR** — Compliance cannot be determined from static analysis alone.
- The CHECK requires runtime verification (e.g., "verify in handler tests" but tests don't exist)
- The constraint references code that doesn't exist yet
- The grep pattern is ambiguous or the CHECK instruction is incomplete

### Step 5: Generate Report

## Output Format

Present the full audit as a markdown report:

```markdown
## ADR Constraint Drift Report

**Date:** YYYY-MM-DD
**ADRs audited:** N accepted, N skipped (proposed/deprecated/superseded)
**Constraints checked:** N total

### Summary

| Verdict | Count |
|---------|-------|
| PASS    | N     |
| DRIFT   | N     |
| UNCLEAR | N     |

### Results

| ADR | Constraint | Verdict | Details |
|-----|-----------|---------|---------|
| ADR-0001 | Handlers Must Not Import Database Packages | PASS | No database imports found in `internal/handler/` |
| ADR-0001 | Service Must Depend on Repo Interfaces | PASS | Service imports `internal/repository` (interfaces only) |
| ADR-0003 | All Updates Must Check Version | DRIFT | `internal/repository/postgres/order_repo.go:45` — UPDATE missing `WHERE version = $N`. **Impact:** Skipping version checks defeats concurrency control and allows lost updates |
| ADR-0002 | POST Must Return 201 with Location | UNCLEAR | CHECK says "verify in handler tests" but `TestCreateOrder` does not assert Location header |

### DRIFT Details

For each DRIFT finding, provide an expanded section:

#### DRIFT: [Constraint Name] (ADR-NNNN)

**File:** `path/to/file.go:line`
**Violation:** [What was found]
**Expected:** [What the constraint requires]
**Business Impact:** [BECAUSE text from the ADR]
**Recommendation:** [How to fix]
```

## Example Walkthrough

Given ADR-0001 CONSTRAINT "Handlers Must Not Import Repository or Database Packages":

1. **Extract CHECK:** `grep -r "github.com/jackc/pgx\|database/sql" internal/handler/`
2. **Execute:** Use Grep tool to search for `github.com/jackc/pgx|database/sql` in `internal/handler/`
3. **Result:** No matches found
4. **Verdict:** PASS — No database imports found in handler layer

Given ADR-0003 CONSTRAINT "All Update Operations Must Check Version":

1. **Extract CHECK:** `grep -r "UPDATE orders" internal/repository/postgres/`
2. **Execute:** Use Grep tool to find all UPDATE statements
3. **Read** each matched file and verify every UPDATE includes `WHERE ... AND version = $N`
4. **Result:** Found UPDATE at line 45 without version check
5. **Verdict:** DRIFT
6. **Business Impact:** "Skipping version checks defeats concurrency control and allows lost updates"

## Notes

- This skill complements `make drift-check` — the Makefile runs a subset of automated grep checks, while this skill performs a comprehensive audit of every CONSTRAINT in every accepted ADR
- UNCLEAR is not a failure — it means the constraint needs a better CHECK definition or the verification requires runtime testing
- When reporting DRIFT, always quote the BECAUSE text so the reader understands the business impact without needing to read the ADR
- If a new ADR is added without CHECK instructions, report all its constraints as UNCLEAR with a recommendation to add CHECK definitions
- Never suggest fixing drift in this report — only report findings. The user decides what action to take
