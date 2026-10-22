# Engineering and code review standard

This standard applies to `eco-cdp-be`. The first release serves Customer Intelligence Cards and Universe, internal access, and optional non-production Self-Pilot token issuance. Segment v0 is implemented locally for a later release behind default-off API and worker switches; its current contract is in the segment migration plan and OpenAPI.

## Code structure

Feature layout within the existing `ecodai` service:

```text
eco-cdp-be/
├── go.work
├── pkg/                                Shared Go module
│   ├── errors/                         Safe API error type and common client catalogue
│   ├── logger/                         Trace context and structured slog
│   ├── response/                       Echo success/error envelope
│   └── server/                         Echo setup and request middleware
└── services/ecodai/                   One service and deployment for product features
    ├── cmd/server/                     Configuration, wiring, lifecycle
    ├── internal/
    │   ├── auth/                       Bearer validation, grants, middleware
    │   ├── config/                     Service configuration
    │   ├── database/                   PostgreSQL pool holder
    │   ├── features/                   Product capabilities served by ecodai
    │   │   ├── customerintelligence/   Cards and Universe
    │   │   │   ├── api/                 Echo routes, handlers, DTOs, response mapping
    │   │   │   ├── service/             Use cases, reporting rules, consumed ports
    │   │   │   ├── model/               Persistence-neutral queries, snapshots, views
    │   │   │   └── postgres/            Adapters, handwritten SQL, generated sqlc
    │   │   ├── segments/               Segment discovery, rules, requests, exports
    │   │   │   ├── api/                 HTTP contract and handlers
    │   │   │   ├── service/             Use cases and business rules
    │   │   │   ├── model/               Business types when a separate package helps
    │   │   │   ├── postgres/            Database adapters and queries
    │   │   │   └── worker/              Background build/export orchestration
    │   │   └── newfeature/             Example: start small; add role packages as needed
    │   ├── selfpilot/                  Optional token issuer and route
    │   └── system/                     Health, docs, access, composition
    ├── migrations/                     Paired schema migrations
    ├── docs/                           Served OpenAPI and Swagger assets
    ├── sqlc.yaml
    └── Dockerfile
```

Customer Intelligence and Segments now live under `internal/features/`, with Segments split by role. The `newfeature/` entry illustrates where a future capability belongs; it is not a required empty folder. Segment integration tests and their fixtures live at the Segments feature root so they can exercise the API, service, PostgreSQL adapter, and worker together.

The root `pkg/` contains only behavior reusable by another service without importing `services/ecodai/internal/`. A second service may depend on its module with a pinned version. In this repository, `go.work` is for local development and the service module's relative `replace` makes its Docker build use the same shared source. Review and version the shared module before moving it to a separate repository.

New product features stay in `services/ecodai/internal/features/` and share the existing process, Echo server, authentication, configuration, and database pool. A menu tab is a clue to ownership, not an automatic package boundary: a new view of existing Customer Intelligence rules belongs with Customer Intelligence; a capability with its own rules and data lifecycle gets its own feature folder. Do not create a new microservice or Go module merely because a new tab is added. Keep service-wide `auth`, `config`, `database`, `selfpilot`, and `system` outside `features/`. Do not move feature-specific rules into those service-wide packages.

Customer Intelligence is one feature area with role packages because its HTTP, reporting, and persistence code each have substantial responsibilities. The dependency direction is `api -> service -> model` and `postgres -> model`; `cmd/server` constructs the PostgreSQL adapters and passes them to services and handlers. `service/ports.go` declares the repository interfaces consumed by the use cases. PostgreSQL adapters satisfy them implicitly and do not import `service`. Keep service entry points short enough to show the request flow; put feature calculations, status decisions, and response mapping in their named files. Same-package helpers stay unexported. HTTP DTOs must not contain `pgtype` or sqlc types. The PostgreSQL adapter alone imports generated SQL code. `cmd/server` owns wiring and process lifecycle, not reporting rules.

Feature ownership is a directory boundary, not permission to combine unrelated responsibilities in one file. Start a small feature with purposeful files in one package; add role packages when HTTP, business rules, persistence, or background work have independently changing responsibilities and tests. A large file or package prompts that review, but line count alone is not the rule. Split a file when it starts owning two independently changing rules. Segments has separate API, service, model, PostgreSQL, and worker packages because these responsibilities are substantial. Do not create empty `dto`, `ports`, `domain`, `postgres`, or `worker` folders, or separate handler and controller packages for the same HTTP job. HTTP DTOs belong with the feature API; repository interfaces belong near their consuming service; business types belong in a feature `model` package when that boundary helps. For example, a Cards freshness change belongs in `service/cards_features.go`, a penetration calculation in `service/cards_trends.go`, a Universe Op-Co rule in `service/universe_opcos.go`, and a JSON shape change in an `api` mapper or DTO file.

## Routing and HTTP contract

- Use Echo. `pkg/server` owns global tracing, recovery, the 10-second request deadline, and the 4096-byte request body bound. Handlers map oversized or malformed JSON to the existing 400 status.
- Feature packages declare their routes. `internal/system` composes the server, public endpoints, and feature registration. Every protected operation declares a nonempty permission; Cards and Universe require `cdphub.cust_intel.view`. Internal access requires a valid token without that operation grant.
- Keep the upstream v0 paths and methods in the served `docs/openapi.yaml`. Review gateway rewrites separately from service routing. Update OpenAPI alongside changes to payloads, errors, permissions, or status codes. Document that Segment routes are registered only when their API switch is enabled.
- Handlers validate transport input, call a use case, and map its result. Use cases return business errors and must not use Echo or SQL types. Preserve valid partial responses, freshness states, date rules, and explicit permission checks.

## Errors, responses, and logs

- Use `pkg/errors.APIError` for safe code, message, and HTTP status. `pkg/errors` owns named constructors for common client contracts; each call returns a fresh value. Keep feature-specific code definitions beside the feature. Map domain causes with `errors.Is` or `errors.As` at the HTTP boundary. Keep wrapped internal causes for diagnosis, but never expose raw causes to clients.
- Return failures through `pkg/response.ErrorHandler`. The error envelope contains `success: false`, `errors.code`, `errors.message`, and `trace_id` when available. Success envelopes retain `success`, `message`, `data`, and trace ID. Return the same ID in `X-Trace-ID`.
- `pkg/errors` owns `10001` invalid request, `40007` access denied, `401` unauthorized, `503` service unavailable, and `99999` unknown internal failure. The `10009` database-operation code remains service-local; Customer Intelligence currently owns its constructor. Customer Intelligence owns `93001` through `93005`; Self-Pilot owns `429` for its throttle. Codes and safe messages are client contract: update OpenAPI and tests when changing them. Do not copy a common code/message/status triple into a feature.
- `pkg/response.ResolveError` defines the client-visible error status for both response writing and request logging. Keep logs aligned with the status actually returned to clients. Preserve the safe generic 500 response for unexpected Echo 5xx errors unless a separate contract decision changes it.
- Use `pkg/logger` for request-scoped logs. Include trace IDs automatically, stable keys, and sanitized failure categories. Do not log tokens, JWT contents, raw auth responses, customer identifiers, SQL arguments, or raw database errors. Avoid logging the same failure at every layer.

## Data and dependency rules

- Flow request context through handler, use case, repository, and outbound auth calls. Child database deadlines are 3 seconds; external auth HTTP calls are bounded separately. Never detach request-path work from cancellation.
- Share identical request framing within the owning HTTP package. Cards and Universe use `api/request.go` for strict JSON object framing; their allowed fields, null handling, and date trimming remain endpoint-specific. Self-Pilot's optional empty body is a separate contract.
- Keep shared PostgreSQL query-error classification and nullable pgx conversion in feature-owned `postgres/query_error.go` and `postgres/values.go`; keep repository names, query names, and safe log metadata explicit at call sites.
- Keep read-only, repeatable-read transactions for reporting snapshots. Do not replace sqlc with handwritten SQL as part of this standard. Author static SQL in the owning feature's `postgres/sql/` (for Customer Intelligence, `internal/features/customerintelligence/postgres/sql/`), regenerate `sqlc/` with `sqlc generate`, and never edit generated scanners directly. Use pgx generic collectors only for handwritten queries when column names and nullable values map clearly; verify converted queries with a database test.
- Put schema changes in paired `migrations/*.up.sql` and `*.down.sql`. Review released migration history, rollback order, and existing data. Keep synthetic fixtures out of production feeds.
- Authentication uses local RS256/JWKS validation and Partner Portal permission lookup. Self-Pilot remains disabled in production and its route is registered only when configured. Do not move CDP Hub grants or the Partner Portal adapter into `pkg/`.

## Review and verification

Check route and permission parity, error status and envelope, trace header/body, malformed and oversized JSON, freshness and valid-zero cases, and snapshot consistency. Run both module test suites and regenerate sqlc. Build the service image from the repository root context; check embedded docs and migration paths. Use a disposable PostgreSQL database for repository and migration verification when available. A local pass does not prove production reporting feeds, gateway routing, Partner Portal integration, or deployment readiness.

Report confirmed findings first, highest severity first, with file and line, triggering scenario, and smallest reasonable fix. State which checks were run and which external behavior remains unverified. Do not require unrelated refactors for a focused change.
