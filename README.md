# eco-cdp-be

Independent v0 CDP backend. The first increment serves Customer Intelligence Cards and Universe, token access checks, and optional non-production Self-Pilot token issuance. Segment APIs and the confirmation worker are reserved for a later increment.

## Run

Requires Go 1.26 and a dedicated PostgreSQL database. Apply `migrations/000001_customer_intelligence.up.sql` to that database with a migration runner. Populate the Customer Intelligence reference and reporting tables from the approved data pipeline before expecting chart results.

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

Then run `go run ./services/eco/cmd/server`. Protected requests validate RS256 JWT signature and claims and resolve effective permissions through Partner Portal introspection. The approved Customer Intelligence operation permission is `cdphub.cust_intel.view`; the literal `cdphub.*` grant also allows it. Cache lifetime bounds permission revocation delay. An expired JWT is revalidated and rejected before cache use.

All planned segment v0 routes use `cdphub.cust_intel.view` for now, including writes and export requests. Those routes are not yet registered; they belong to the later segment release.

For an isolated local/dev/UAT environment, set `APP_ENV=local`, `SELF_PILOT_ENABLED=true`, `SELF_PILOT_ISSUER=urn:eco-cdp-be:self-pilot:local`, and optionally `SELF_PILOT_TOKEN_TTL=1h`. Self-Pilot is disabled unless explicitly enabled and cannot run in production. Restrict its token route to an internal network in dev and UAT.

Health: `GET /healthz`; readiness: `GET /readyz`. The v0 API contract is in [.docs/openapi-v0.yaml](.docs/openapi-v0.yaml).

## Verify

`go test ./...` runs unit and contract tests. Set `ECO_TEST_DSN` to a disposable PostgreSQL database and run `go test ./internal/repository -run TestFreshCustomerIntelligenceMigration -count=1` to apply migration `000001` from scratch, load synthetic Universe data, and exercise Cards and Universe queries.

The route inventory, source provenance, remaining external decisions, and cutover gates are in [.docs/migration-status.md](.docs/migration-status.md).
