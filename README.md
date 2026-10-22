# eco-cdp-be

Go backend for CDP Hub Customer Intelligence Cards and Universe APIs. The service lives in [`services/ecodai/`](services/ecodai/); reusable HTTP, error, response, and logging code lives in [`pkg/`](pkg/).

## Local setup

Requires Go 1.26, Docker, and [Air](https://github.com/air-verse/air). From the repository root:

```sh
make -C services/ecodai install-air # once, if Air is not installed
make run-local
```

`make run-local` starts PostgreSQL, applies migrations, creates `services/ecodai/.env` from `.env.example` if needed, and runs the API with Air auto reload at `http://localhost:8092`. Press `Ctrl+C` to stop Air; run `make -C services/ecodai local-db-down` to stop the database.

The renamed service uses a new Docker Compose project and database volume. Existing data in the old `eco` project remains on disk. Stop its PostgreSQL container if it still uses port 55434, then run `seed-local` again if you need sample data in the new local database.

Check `curl http://localhost:8092/healthz` or request a local token with `curl -X POST http://localhost:8092/api/v0/internal/self-pilot/token`. The sample auth URLs in `.env.example` are placeholders for Self-Pilot testing; use real Partner Portal values for Partner Portal tokens.

OpenAPI is available at `http://localhost:8092/openapi.yaml`; Swagger UI is at `http://localhost:8092/docs/swagger-ui/`. The UI loads its JavaScript and CSS from a CDN. Deployed URLs require the gateway to forward those paths to this service.

If the ignored local seed file exists and `psql` is installed, run `make -C services/ecodai seed-local` to load synthetic Cards and Universe data into the local database. Run tests with `make -C services/ecodai test`.

The local segment prototype uses migrations `000003` and `000004`. Migration `000004` seeds 27 approved attribute options and their hidden shared base with `seg_######` IDs. Set `SEGMENT_V0_API_ENABLED=true` for listing and draft custom definitions. For synthetic build evaluation and exports, set `APP_ENV=local`, `SEGMENT_V0_BUILD_ENABLED=true`, and `SEGMENT_V0_WORKER_ENABLED=true`; the build and worker flags are rejected in other environments until the organisation guard is implemented. Insert a LOADING build with its frozen required outputs, build-scoped source rows, and completed source-load records, then run `go run ./cmd/segment-build BUILD_ID` from `services/ecodai`. Counts prefer the current READY build and otherwise show a labelled patch or no count. Exports finish under `SEGMENT_V0_LOCAL_EXPORT_DIR`. Request status requires a bearer token with `cdphub.cust_intel.view`. The older `make -C services/ecodai seed-segments-local` and `seed-segments-scale-local` commands exercise the legacy local fixture only. See the [scale plan](.docs/segment-bitmap-builder-plan.md) for the remaining release gates.

For a card activity playground on a **freshly migrated** local `eco` database, run `make -C services/ecodai seed-segments-card-activity-local`. The one-shot [seed file](services/ecodai/internal/features/segments/testdata/card_activity_local.sql) creates `CARD_PLAY_01`, 120 synthetic members, their retail rows, and local predefined segment `seg_000001`; the target also publishes SQL counts, bitmaps, and dashboard results. The predefined rule is active card plus a retail transaction in the 30 days ending on the build source date. It has 90 raw members and 80 build-consented members. Optional `annual_income >= 100000` and `race IN ('malay')` filters return 40 each, or 10 when combined. A numeric filter selection uses one scalar value, while an `in` selection uses an array. This fixture ID is for local exploration, not an approved production predefined ID. Do not run the seed target twice on the same READY build; reset the disposable database first.

For a larger local test, run `make -C services/ecodai seed-segments-scale-local` after the migrations. It adds exactly 1,000,000 synthetic members to `member_cookbook_v0` and three dedicated overlapping snapshots (`seg_900001` through `seg_900003`); it leaves the small fixture intact. Run `make -C services/ecodai test-segments-scale-local` to exercise estimate, confirm, status, and a full local CSV export. The scale test removes its request and CSV afterward and keeps the synthetic member data for manual API testing. Use this fixture only in the local development database.

The repository uses `go.work` for local development. To test both modules and build the service image from the root build context:

```sh
go test ./pkg/... ./services/ecodai/...
docker build -f services/ecodai/Dockerfile -t ecodai-local .
```

Product capabilities live under `services/ecodai/internal/features/`: Customer Intelligence and Segments have separate API, service, model, and PostgreSQL packages; Segments also has a worker package. Authentication, configuration, database setup, and system routes remain service-wide packages under `internal/`. New menu features stay in this service and get role packages only as their responsibilities grow. Run `sqlc generate` from `services/ecodai/` after editing Customer Intelligence queries. The [engineering standard](.docs/code-review-standard.md) records package ownership and the Echo HTTP contract.

See the [API contract](services/ecodai/docs/openapi.yaml) and [migration status](.docs/migration-status.md) for further detail.
