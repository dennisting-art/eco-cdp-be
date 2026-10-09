# eco-cdp-be

Go backend for CDP Hub Customer Intelligence Cards and Universe APIs. The service and its build files live in [`services/eco/`](services/eco/).

## Local setup

Requires Go 1.26, Docker, and `psql`. From the repository root:

```sh
docker run --name eco-cdp-postgres -e POSTGRES_USER=eco -e POSTGRES_PASSWORD=eco -e POSTGRES_DB=eco -p 5433:5432 -d postgres:16
until docker exec eco-cdp-postgres pg_isready -U eco -d eco; do sleep 1; done
export DATABASE_URL='postgres://eco:eco@localhost:5433/eco?sslmode=disable'
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f services/eco/migrations/000001_customer_intelligence.up.sql
```

For local Self-Pilot testing, set the required auth config and start the service:

```sh
export AUTH_ISSUER='http://localhost:9999'
export AUTH_JWKS_URL='http://localhost:9999/jwks'
export AUTH_AUDIENCE='eco-cdp-be'
export AUTH_INTROSPECTION_URL='http://localhost:9999/introspect'
export APP_ENV=local SELF_PILOT_ENABLED=true SELF_PILOT_ISSUER='urn:eco-cdp-be:self-pilot:local'
go -C services/eco run ./cmd/server
```

The `localhost:9999` auth URLs are placeholders for Self-Pilot use only. Use real Partner Portal values to test Partner Portal tokens. In another terminal, check `curl http://localhost:8080/healthz` and request a local token with `curl -X POST http://localhost:8080/api/v0/internal/self-pilot/token`.

If you have the ignored local seed file, load synthetic Cards and Universe data with `psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f services/eco/seed/local_customer_intelligence.sql`. The seed is for a disposable database only.

Run tests with `go -C services/eco test ./...`. The [API contract](services/eco/docs/openapi-v0.yaml) and [migration status](.docs/migration-status.md) have further detail.
