# v0 migration status

This repository has fresh Git history. The legacy `cdp-be` and `acs-cdp-be` checkouts were read only. No legacy service is a build or runtime dependency.

The ecodai service lives under `services/ecodai/` and depends on the shared root `pkg/` Go module for HTTP server setup, responses, common errors, and logging. Its service-specific code, SQL, migrations, API contract, Dockerfiles, CI includes, and local scripts remain under `services/ecodai/`. The service image builds from the repository root so both modules are available. Root `.gitlab-ci.yml` triggers its child pipelines. The local synthetic seed at `services/ecodai/seed/` is ignored by Git.

## Approved contract and release order

The user confirmed `cdphub.cust_intel.view` for Customer Intelligence and the segment list, get, create, and estimate/confirm routes, including segment writes and exports. A later instruction makes segment request-status polling accessible to anyone holding the request ID, without a token, operation permission, requester check, or route-specific rate limit. The plan's `acs_cdp.*` and `aud_build` entries were examples, not approved permission tags. The segment schema and feature runtime have been implemented for a later release behind default-off API and worker switches; no production stage has been deployed.

| Method and path | Body or parameters | Response data | Permission | Current consumer and public URL |
| --- | --- | --- | --- | --- |
| `POST /api/v0/customer-intelligence/cards` | JSON `scope_code`, optional `start_date` and `end_date`; omitted, null, or empty scope selects AEON | resolved dates, status, warnings, scope code, configured features with chart-level data status when needed | `cdphub.cust_intel.view` | CDP Hub; legacy dev/UAT gateway uses `/contextiq` prefix; ecodai gateway path pending owner confirmation |
| `POST /api/v0/customer-intelligence/universe` | Optional paired `start_date` and `end_date` | resolved dates, chart dates, scopes, group, opcos, chart-level data status when needed | `cdphub.cust_intel.view` | CDP Hub; ecodai gateway path pending owner confirmation |
| `GET /api/v0/internal/access` | Bearer token | `allowed: true` for accepted token | valid token; no operation grant | Internal access client; legacy gateway uses `/users` prefix; ecodai gateway path pending owner confirmation |
| `POST /api/v0/internal/self-pilot/token` | Empty body or `{}` | token, type, expiry, profile | no caller grant; non-production and internal network only | Internal test clients; ecodai gateway path pending owner confirmation |
| `GET /api/v0/segments/list` | `page`, `pageSize`, `search`, `tags` | segments, page, pageSize, total | `cdphub.cust_intel.view` | CDP Hub; later release |
| `GET /api/v0/segments/{id}` | segment ID | segment definition | `cdphub.cust_intel.view` | CDP Hub; later release |
| `POST /api/v0/segments` | name, tags, description, operation, filters | created segment | `cdphub.cust_intel.view` | CDP Hub; later release |
| `POST /api/v0/segments/requests` | mode and segment selections; `Idempotency-Key` for confirmation | estimate or request ID/status | `cdphub.cust_intel.view` | CDP Hub; later release |
| `GET /api/v0/segments/requests/{request_id}` | request ID | status and, when ready, counts and bucket | request ID only; no token or permission | CDP Hub; later release |

The route catalogs match these nine methods and paths. The legacy ContextIQ v0 OpenAPI omits `/api/v0/internal/access`; UserMS OpenAPI defines it. The eco OpenAPI now documents the five segment routes, including unauthenticated request-status polling. The API stores the caller's subject as text; production stability and ownership of Partner Portal subjects still need verification. Organization-level authorization is deferred.

The later approved local segment catalogue uses one reserved category tag per visible row: `predefine`, `custom`, or `attribute`. Migration `000004_attribute_segments` removes the old `type` response field, adds the attribute rule fields, and is applied only to the local test database so far. The 27 attribute rows and their shared all-members snapshot are synthetic local fixtures, not a production feed.

The v0 error envelope uses `errors.code` as a stable string. Missing or invalid bearer tokens return `401` with message `unauthorized`; an authentication provider failure returns `503` with message `service unavailable`. These numeric-string codes standardize the earlier invalid-token `Unauthorized` and provider-failure `Service Unavailable` values. Clients that branch on those earlier values must switch to `401` and `503` before cutover. HTTP statuses remain 401 and 503 respectively. Customer Intelligence data failures can also return HTTP 503 with feature-specific `930xx` codes.

## Provenance and disposition

User approval in this session covers adapting Customer Intelligence source and SQL from `cdp-be`, and separately adapting the JWKS validator from `acs-cdp-be`. Record these origins in future changes. Baseline source revisions inspected: `cdp-be` `0798d33dfa57786d278a3bb7bfc612d6d02fd4cc`; `acs-cdp-be` `59a5351f0499c9017365737df14913c3f70d625d`.

| Material | Origin | Decision |
| --- | --- | --- |
| Cards and Universe domain, DTO, service, repository, SQL, HTTP handlers and mapper; service tests | `cdp-be/services/contextiq/internal/` | Adapted into `services/ecodai/internal/features/customerintelligence/`; imports, logging and module boundary changed. No other ContextIQ domains included. |
| Customer Intelligence schema, view, functions, trigger and matrix feature seed | `cdp-be/migrations/000057_contextiq_customer_intelligence_universe.{up,down}.sql` | Adapted as fresh `000001_customer_intelligence`. Includes only reporting schema. |
| Customer Intelligence scopes, feature catalog, buckets and enablement | `cdp-be/migrations/000058_seed_ecosystem_scope_features.{up,down}.sql` | Adapted as `services/ecodai/migrations/000002_seed_ecosystem_scope_features`. The rollback retains `MEMBER_MATRIX_ACTIVITY` from `000001`. |
| Synthetic Universe fixture | `cdp-be/scripts/seed_customer_intelligence_universe_local.sql` | Retained only under `internal/features/customerintelligence/postgres/testdata/`; never production seed. |
| First-release OpenAPI | `cdp-be/services/contextiq/docs/openapi-v0.yaml`, UserMS access spec | Adapted; removed segment routes and legacy server URLs, added access route. |
| RS256/JWKS validator and its tests | `acs-cdp-be/services/custms/internal/auth/jwks_validator{,_test}.go` | Adapted; bearer token no longer sent to public JWKS; future `iat` rejected. |
| Self-Pilot issuer and verifier | `cdp-be/services/userms/internal/selfpilot/selfpilot.go` | Adapted to eco audience, scope and independent configuration. |
| Permission lookup and cache | Partner Portal interface described by ACS auth integration reference | Rewritten; caches the full effective permission set by token hash, revalidates JWT on every request. |
| `echo`, `pgx`, `uuid` | Existing dependencies in the reference repositories | Echo replaced Gin for the HTTP runtime; pgx and uuid remain direct Go dependencies. Module download uses public upstream modules, not legacy artifacts. |
| `userms`, `taskms`, `eventms`, legacy `/api/v1`, segment schema and worker, customer-specific ACS schema/API | Legacy repositories | Excluded from first release. |
| Production reporting facts, member cookbook, segment definitions, in-flight requests | Legacy database or upstream pipelines; ownership still unconfirmed | Excluded from repository. Migration and reconciliation are pending approved data owners. |

## Data and deployment gates

`000001` creates these Customer Intelligence objects from an empty database: `ecosystem_scope`, `feature_summary`, `feature_bucket_dim`, `ecosystem_scope_feature`, `customer_daily_summary`, `member_daily_distribution`, `annual_member_acquisition`, `member_entity_daily`, `member_daily_summary`, `member_monthly_summary`, `transaction_daily_summary`, `vw_monthly_member_penetration`, and monthly summary ingestion functions and triggers. It creates no segment table. A disposable PostgreSQL 16 run applied it successfully; the integration test then loaded synthetic data and queried both domains.

`000002` seeds five scopes, nine features, 28 buckets, and seven scope-feature enablements from the legacy reference migration. It contains no production reporting facts. A read-only query of the current database confirmed its `AEON`, `ABG`, `AMY`, `ACSM`, and `ADB` scopes, `TIER` and MVC feature codes, and MVC bucket definitions. The Cards service and synthetic fixtures now use those catalog terms; the local fixture preserves availability flags already seeded by `000002`. `TIER` has no scope assignment and MVC is disabled in the source seed, so the local test seed enables them only in its disposable database. The down migration is intended for a fresh catalog before reporting facts reference these rows.

The service and migration Docker images built locally from this repository. The service image started against the disposable database; readiness returned 200 and a Self-Pilot authenticated Universe request returned `status=complete`, four OpCo cards, and a matrix. This confirms a local smoke path only, not Partner Portal or public gateway behavior.

The repository contains local synthetic fixtures for Customer Intelligence and segments, including a repeatable million-member segment scale fixture. No production loader or owning data pipeline was found in `cdp-be`. The segment migration and API are implemented and locally tested; production data loading is a later data-team handoff, not a blocker to the current implementation or deployment of the schema with segment switches off. Before enabling traffic on real data, confirm the feed's member IDs and consent semantics and ensure the latest `computed_date` is a complete membership snapshot. Ongoing ownership, refresh monitoring, and reconciliation belong in that production handoff and release runbook. Historical ContextIQ data and active requests need migration only if they must remain available after cutover.

The external gateway configuration is in `cdp-k8s-manifests/overlays/{dev,uat,prep,prod}/kong-ingress/cdphub-service.yml`. The ingress currently routes `/contextiq` and `/users` by broad prefixes to legacy services and strips the matched path. Its `CODEOWNERS` assigns ingress changes to `@akmal.harith`. A new rule must preserve the public v0 path and the service's `/api/v0/...` upstream path without stealing legacy `/api/v1` traffic. No gateway file was changed.

The ecodai app serves `GET /openapi.yaml`, `GET /docs/openapi.yaml`, and `GET /docs/swagger-ui/` without a token. The OpenAPI server URL is relative to the spec, so Swagger UI uses the current host and gateway prefix. The external ecodai gateway prefix is still unconfirmed; these documentation paths need forwarding when it is configured.

The service and migration child pipelines both include `dso-cicd/templates/build/docker.yml`, inspected at revision `b57cf491a54e4fd4c70a8a5fa3d3010ac40fd3ee`. The template publishes to GCP Artifact Registry and expects `repo-ecocdphub-svc` to exist in `prj-cmn-svc-3c`; that repository and runner access are not provisioned yet. Its default-branch job promotes a staging image rather than building one, so bootstrap the new GitLab project by building `develop` and `staging` before running the first `main` release pipeline. Do the same for both child pipelines.

The GitLab project and dedicated database/test environment are not yet set up, as confirmed by the user. Production reporting data feed ownership is also not yet set up. Separate deployment manifests, pipeline image publication, data sync, response parity, live Partner Portal auth latency, backup/restore, write recovery, and routing cutover have not been executed. Confirm the intended public URL and gateway rewrite with the ingress owner before adding external servers to OpenAPI or moving traffic.
