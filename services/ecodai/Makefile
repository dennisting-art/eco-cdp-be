.PHONY: run-local check-air install-air local-db-up local-db-down migrate-up seed-local seed-segments-local seed-segments-scale-local seed-segments-attributes-local seed-segments-card-activity-local test-segments-scale-local test

run-local: check-air local-db-up migrate-up
	@if [ ! -f .env ]; then cp .env.example .env; echo 'Created services/ecodai/.env from .env.example'; fi
	air -c .air.toml

check-air:
	@if ! command -v air >/dev/null 2>&1; then echo 'Air is required: make -C services/ecodai install-air' >&2; exit 1; fi

install-air:
	go install github.com/air-verse/air@latest

local-db-up:
	docker compose up -d --wait postgres

local-db-down:
	docker compose down

migrate-up:
	docker compose run --rm migrations

seed-local:
	@if [ ! -f seed/local_customer_intelligence.sql ]; then echo 'The ignored local seed is missing: services/ecodai/seed/local_customer_intelligence.sql' >&2; exit 1; fi
	@psql -v ON_ERROR_STOP=1 -d 'postgres://eco:eco@localhost:55434/eco?sslmode=disable' -f seed/local_customer_intelligence.sql

seed-segments-local:
	@psql -v ON_ERROR_STOP=1 -d 'postgres://eco:eco@localhost:55434/eco?sslmode=disable' -f internal/features/segments/testdata/local.sql

seed-segments-scale-local:
	@psql -v ON_ERROR_STOP=1 -d 'postgres://eco:eco@localhost:55434/eco?sslmode=disable' -f internal/features/segments/testdata/scale_1m.sql

seed-segments-attributes-local:
	@psql -v ON_ERROR_STOP=1 -d 'postgres://eco:eco@localhost:55434/eco?sslmode=disable' -f internal/features/segments/testdata/attribute_local.sql

# One-shot local playground on a freshly migrated eco database.
seed-segments-card-activity-local:
	@psql -v ON_ERROR_STOP=1 -d 'postgres://eco:eco@localhost:55434/eco?sslmode=disable' -f internal/features/segments/testdata/card_activity_local.sql
	@APP_ENV=local DATABASE_URL='postgres://eco:eco@localhost:55434/eco?sslmode=disable' go run ./cmd/segment-build CARD_PLAY_01

test-segments-scale-local:
	@SEGMENT_SCALE_DSN='postgres://eco:eco@localhost:55434/eco?sslmode=disable' go test ./internal/features/segments -run TestMillionMemberLocalExport -count=1 -v -timeout 12m

test:
	go test ./...
