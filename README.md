# eco-cdp-be

Go backend for CDP Hub Customer Intelligence Cards and Universe APIs. The service and its build files live in [`services/eco/`](services/eco/).

## Local setup

Requires Go 1.26, Docker, and [Air](https://github.com/air-verse/air). From the repository root:

```sh
make -C services/eco install-air # once, if Air is not installed
make run-local
```

`make run-local` starts PostgreSQL, applies migrations, creates `services/eco/.env` from `.env.example` if needed, and runs the API with Air auto reload at `http://localhost:8092`. Press `Ctrl+C` to stop Air; run `make -C services/eco local-db-down` to stop the database.

Check `curl http://localhost:8092/healthz` or request a local token with `curl -X POST http://localhost:8092/api/v0/internal/self-pilot/token`. The sample auth URLs in `.env.example` are placeholders for Self-Pilot testing; use real Partner Portal values for Partner Portal tokens.

OpenAPI is available at `http://localhost:8092/openapi.yaml`; Swagger UI is at `http://localhost:8092/docs/swagger-ui/`. The UI loads its JavaScript and CSS from a CDN. Deployed URLs require the gateway to forward those paths to this service.

If the ignored local seed file exists and `psql` is installed, run `make -C services/eco seed-local` to load synthetic Cards and Universe data into the local database. Run tests with `make -C services/eco test`.

See the [API contract](services/eco/docs/openapi-v0.yaml) and [migration status](.docs/migration-status.md) for further detail.
