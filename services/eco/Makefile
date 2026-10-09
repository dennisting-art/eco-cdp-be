.PHONY: run-local check-air install-air local-db-up local-db-down migrate-up seed-local test

run-local: check-air local-db-up migrate-up
	@if [ ! -f .env ]; then cp .env.example .env; echo 'Created services/eco/.env from .env.example'; fi
	air -c .air.toml

check-air:
	@if ! command -v air >/dev/null 2>&1; then echo 'Air is required: make -C services/eco install-air' >&2; exit 1; fi

install-air:
	go install github.com/air-verse/air@latest

local-db-up:
	docker compose up -d --wait postgres

local-db-down:
	docker compose down

migrate-up:
	docker compose run --rm migrations

seed-local:
	@if [ ! -f seed/local_customer_intelligence.sql ]; then echo 'The ignored local seed is missing: services/eco/seed/local_customer_intelligence.sql' >&2; exit 1; fi
	@psql -v ON_ERROR_STOP=1 -d 'postgres://eco:eco@localhost:55434/eco?sslmode=disable' -f seed/local_customer_intelligence.sql

test:
	go test ./...
