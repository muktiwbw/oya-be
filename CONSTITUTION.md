# Project Constitution & Engineering Principles

This document defines the foundational engineering principles, development workflow, and coding standards for the OYA project. All contributors and coding agents must strictly adhere to these rules.

---

## Core Tenets

### 1. Test-Driven Development (TDD) Workflow: Red-Green-Refactor
All feature work, bug fixes, and behavioral changes must follow a disciplined TDD cycle:
1. **Red**: Write a focused, failing unit or integration test that precisely defines the expected behavior, input validation, or contract before writing production code. Run the test to confirm it fails for the expected reason.
2. **Green**: Write the simplest, most direct code necessary to make the test pass. Avoid premature generalization during this step.
3. **Refactor**: Clean up the implementation—improve naming, eliminate duplication, and streamline structure—while ensuring all tests remain green.

> Never write speculative production code without a corresponding test that justifies its existence.

---

### 2. Do Not Over-Complicate Code (KISS & YAGNI)
- Solve the immediate problem clearly and completely. Do not build frameworks or speculative abstractions for hypothetical future features.
- Prefer straightforward, boring, and predictable solutions over clever or esoteric ones.
- Avoid unnecessary design patterns, excessive interfaces, or convoluted indirection layers. An interface should only exist when there are multiple concrete implementations or for test decoupling at boundaries.

---

### 3. Prioritize Readability Above All Else
- Code is read significantly more often than it is written. Optimize for ease of understanding by the next engineer.
- Use explicit, descriptive variable and symbol names. Avoid vague acronyms or cryptic abbreviations.
- Keep control flow linear and obvious. Emphasize early returns ("guard clauses") to handle errors or preconditions early, keeping the happy path at minimal indentation levels.
- Avoid deeply nested conditionals or dense one-liners that combine multiple side effects.

---

### 4. Do Not Create Separate Functions Unless Truly Necessary
- Keep logic inline within its caller when it is straightforward and executed in a single place.
- Do not fragment linear readability by breaking simple code into trivial, single-use helper functions. Jumping between tiny functions degrades comprehension.
- **Only extract a separate function when:**
  1. The logic is genuinely reused in multiple places.
  2. The extracted logic implements a distinct, self-contained algorithm or domain rule that benefits from isolated unit testing.
  3. The caller's length or complexity would otherwise obscure high-level control flow.

---

### 5. Document Complex Logic and Specific Algorithms
- Routine, idiomatic code should be self-explanatory through clean naming and clear structure.
- **Mandatory Comments**: Any function, method, or code block that implements:
  - Non-obvious business logic or domain rules,
  - Specific algorithms (e.g., cursor encoding/decoding, pagination math, hashing schemes),
  - Concurrency guarantees, locks, or atomic operations,
  - Workarounds for third-party quirks or edge cases,
  **must** include clear comments explaining the **why** (the intent/rationale) and the non-obvious **how**.

---

## Development & Architecture Best Practices

### 6. Idiomatic Go Error Handling
- Never ignore errors. Check errors explicitly at the call site.
- Wrap errors with descriptive operational context using `fmt.Errorf("action failed: %w", err)` to preserve the causal error chain.
- Use sentinel errors (`var ErrNotFound = errors.New(...)`) or typed validation errors to represent expected domain failures, and check them using `errors.Is()` or `errors.As()`.

### 7. Clean Architectural Layering
Maintain strict boundaries between layers:
- **Transport (`internal/transport/http`)**: Responsible only for HTTP serialization, status codes, route dispatch, authentication extraction, and request body decoding. Contains no domain or business rules.
- **Service & Domain (`internal/<domain>`)**: Contains core application use cases, business validation, and entity behavior. Completely decoupled from transport protocols and raw database drivers.
- **Persistence (`internal/persistence`)**: Implements domain repository interfaces using PostgreSQL and `sqlc`-generated queries. Transport entities must never leak into persistence queries, and database models must not leak into HTTP responses.

### 8. Database Integrity & Type Safety
- **Migrations First**: All schema updates must be written as Goose-compatible versioned migrations under `migrations/` (`<sequence>_<description>.sql`) and include a `-- +goose Up` directive. Never modify existing migrations after they are shared.
- **Type-Safe Queries with `sqlc`**: Use `sqlc` with `pgx/v5` for all database interactions. Raw ad-hoc queries outside `queries/` are disallowed unless dynamic SQL generation (such as optional multi-field cursor filtering) strictly requires it.
- **Database-Level Invariants**: Rely on database constraints (foreign keys, check constraints, unique constraints, and partial indexes for soft-deleted entities) to guarantee data integrity.

### 9. Verification & Quality Gates
Before any work is considered complete:
1. `make fmt` & `make vet`: Format code and ensure static analysis passes.
2. `make test`: Run all unit tests.
3. `make test-race`: Run unit tests with the Go race detector enabled.
4. `make check`: Run the full verification suite (formatting verification, vet, unit tests).
5. Always update `resources/api/spec.yaml` whenever there are updates to available endpoints (schemas, parameters, authentication, or responses).
6. Always update `README.md` whenever there are architectural, functional, configuration, or workflow updates.

---

### 10. Documentation & API Contract Integrity
- **Always update `README.md` whenever there is an update**: Never leave documentation outdated. Any change affecting architecture, dependencies, database configuration, environment variables, Make targets, or local development workflow must be documented in `README.md`.
- **Always update `resources/api/spec.yaml` whenever there is an update on available endpoints**: The OpenAPI specification is the source of truth for the API contract. Whenever endpoints are added, altered, or deprecated, the spec must be kept in strict lockstep, and validated against transport layer test suites.
