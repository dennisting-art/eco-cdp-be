# eco-cdp-be

Independent v0 CDP backend. The `services/eco/` directory owns the Go module, implementation, migrations, Dockerfiles, CI includes, API contract, and service tools. The repository root contains only project-level orchestration and documentation. The first increment serves Customer Intelligence Cards and Universe, token access checks, and optional non-production Self-Pilot token issuance. Segment APIs and the confirmation worker are reserved for a later increment.

## Run

Requires Go 1.26 and a dedicated PostgreSQL database. Apply `services/eco/migrations/000001_customer_intelligence.up.sql` to that database with a migration runner. For an isolated local database, `services/eco/seed/local_customer_intelligence.sql` provides synthetic data; it is ignored by Git. Populate the Customer Intelligence reference and reporting tables from the approved data pipeline before expecting production chart results.

Set:

| Variable | Purpose |
| --- | --- |
| `DATABASE_URL` | Dedicated PostgreSQL connection string |
| `AUTH_ISSUER` | Exact Partner Portal JWT issuer |
| `AUTH_JWKS_URL` | Partner Portal public JWKS URL |
| `AUTH_AUDIENCE` | Required JWT audience |
| `AUTH_INTROSPECTION_URL` | Token introspection URL with effective permissions |
| `AUTH_CACHE_TTL` | Permission-set cache lifetime, default 30 seconds, maximum 1 minute |
| `HTTP_ADDR` | HTTP listener, default `:8080` |

Then run `go -C services/eco run ./cmd/server`. Protected requests validate RS256 JWT signature and claims and resolve effective permissions through Partner Portal introspection. The approved Customer Intelligence operation permission is `cdphub.cust_intel.view`; the literal `cdphub.*` grant also allows it. Cache lifetime bounds permission revocation delay. An expired JWT is revalidated and rejected before cache use.

All planned segment v0 routes use `cdphub.cust_intel.view` for now, including writes and export requests. Those routes are not yet registered; they belong to the later segment release.

For an isolated local/dev/UAT environment, set `APP_ENV=local`, `SELF_PILOT_ENABLED=true`, `SELF_PILOT_ISSUER=urn:eco-cdp-be:self-pilot:local`, and optionally `SELF_PILOT_TOKEN_TTL=1h`. Self-Pilot is disabled unless explicitly enabled and cannot run in production. Restrict its token route to an internal network in dev and UAT.

Health: `GET /healthz`; readiness: `GET /readyz`. The v0 API contract is in [services/eco/docs/openapi-v0.yaml](services/eco/docs/openapi-v0.yaml).

## Verify

`go -C services/eco test ./...` runs unit and contract tests. Set `ECO_TEST_DSN` to a disposable PostgreSQL database and run `go -C services/eco test ./internal/repository -run TestFreshCustomerIntelligenceMigration -count=1` to apply migration `000001` from scratch, load synthetic Universe data, and exercise Cards and Universe queries. Build either image with `docker build -f services/eco/Dockerfile services/eco` or `docker build -f services/eco/Dockerfile.migrations services/eco`.

The route inventory, source provenance, remaining external decisions, and cutover gates are in [.docs/migration-status.md](.docs/migration-status.md).
