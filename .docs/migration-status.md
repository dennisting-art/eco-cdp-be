# v0 migration status

This repository has fresh Git history. The legacy `cdp-be` and `acs-cdp-be` checkouts were read only. No legacy service is a build or runtime dependency.

## Approved contract and release order

The user confirmed `cdphub.cust_intel.view` for the new Customer Intelligence routes. The legacy OpenAPI and implementation use that tag; the plan's `acs_cdp.cust_intel.view` entry was only an example. The first release implements the first four rows below. Segment operations remain in the later release.

| Method and path | Body or parameters | Response data | Permission | Current consumer and public URL |
| --- | --- | --- | --- | --- |
| `POST /api/v0/customer-intelligence/cards` | JSON `scope_code`; omitted, null, or empty selects AEON | status, warnings, scope_code, configured features | `cdphub.cust_intel.view` | CDP Hub; legacy dev/UAT gateway uses `/contextiq` prefix; eco gateway path pending owner confirmation |
| `POST /api/v0/customer-intelligence/universe` | JSON `start_date`, `end_date`; nonblank dates rejected | latest chart dates, scopes, group, opcos, warnings | `cdphub.cust_intel.view` | CDP Hub; eco gateway path pending owner confirmation |
| `GET /api/v0/internal/access` | Bearer token | `allowed: true` for accepted token | valid token; no operation grant | Internal access client; legacy gateway uses `/users` prefix; eco gateway path pending owner confirmation |
| `POST /api/v0/internal/self-pilot/token` | Empty body or `{}` | token, type, expiry, profile | no caller grant; non-production and internal network only | Internal test clients; eco gateway path pending owner confirmation |
| `GET /api/v0/segments/list` | `page`, `pageSize`, `search`, `tags` | segments, page, pageSize, total | `cdphub.aud_build.view` proposed, registration pending | CDP Hub; later release |
| `GET /api/v0/segments/{id}` | segment ID | segment definition | `cdphub.aud_build.view` proposed, registration pending | CDP Hub; later release |
| `POST /api/v0/segments` | name, tags, description, operation, filters | created segment | `cdphub.aud_build.create` proposed, registration pending | CDP Hub; later release |
| `POST /api/v0/segments/requests` | mode and segment selections; `Idempotency-Key` for confirmation | estimate or request ID/status | `cdphub.aud_build.create` proposed, registration pending | CDP Hub; later release |
| `GET /api/v0/segments/requests/{request_id}` | request ID | status and, when ready, counts and bucket | `cdphub.aud_build.view` proposed, registration pending | CDP Hub; later release |

The route catalogs match these nine methods and paths. The legacy ContextIQ v0 OpenAPI omits `/api/v0/internal/access`; UserMS OpenAPI defines it. Segment OpenAPI still says only a valid bearer token is required and underspecifies body and response shapes. Resolve the segment permission and contract gaps before registering those routes. The first-release OpenAPI in this repository contains only registered v0 routes. Organization-level authorization is deferred. Segment writes require a stable UUID mapping from Partner Portal `sub` and object checks before that release; the current Partner Portal `sub` has not been verified as the required UUID.

## Provenance and disposition

User approval in this session covers adapting Customer Intelligence source and SQL from `cdp-be`, and separately adapting the JWKS validator from `acs-cdp-be`. Record these origins in future changes. Baseline source revisions inspected: `cdp-be` `0798d33dfa57786d278a3bb7bfc612d6d02fd4cc`; `acs-cdp-be` `59a5351f0499c9017365737df14913c3f70d625d`.

| Material | Origin | Decision |
| --- | --- | --- |
| Cards and Universe domain, DTO, service, repository, SQL, HTTP handlers and mapper; service tests | `cdp-be/services/contextiq/internal/` | Adapted into `internal/`; imports, logging and module boundary changed. No other ContextIQ domains included. |
| Customer Intelligence schema, view, functions, trigger and matrix feature seed | `cdp-be/migrations/000057_contextiq_customer_intelligence_universe.{up,down}.sql` | Adapted as fresh `000001_customer_intelligence`. Includes only reporting schema. |
| Synthetic Universe fixture | `cdp-be/scripts/seed_customer_intelligence_universe_local.sql` | Retained only under repository testdata; never production seed. |
| First-release OpenAPI | `cdp-be/services/contextiq/docs/openapi-v0.yaml`, UserMS access spec | Adapted; removed segment routes and legacy server URLs, added access route. |
| RS256/JWKS validator and its tests | `acs-cdp-be/services/custms/internal/auth/jwks_validator{,_test}.go` | Adapted; bearer token no longer sent to public JWKS; future `iat` rejected. |
| Self-Pilot issuer and verifier | `cdp-be/services/userms/internal/selfpilot/selfpilot.go` | Adapted to eco audience, scope and independent configuration. |
| Permission lookup and cache | Partner Portal interface described by ACS auth integration reference | Rewritten; caches the full effective permission set by token hash, revalidates JWT on every request. |
| `gin`, `pgx`, `uuid` | Existing dependencies in the reference repositories | Retained as direct Go module dependencies; module download uses public upstream modules, not legacy artifacts. |
| `userms`, `taskms`, `eventms`, legacy `/api/v1`, segment schema and worker, customer-specific ACS schema/API | Legacy repositories | Excluded from first release. |
| Production datasets, reference values, member cookbook, segment definitions, in-flight requests | Legacy database or upstream pipelines; ownership still unconfirmed | Excluded from repository. Migration and reconciliation are pending approved data owners. |

## Data and deployment gates

`000001` creates these Customer Intelligence objects from an empty database: `ecosystem_scope`, `feature_summary`, `feature_bucket_dim`, `ecosystem_scope_feature`, `customer_daily_summary`, `member_daily_distribution`, `annual_member_acquisition`, `member_entity_daily`, `member_daily_summary`, `member_monthly_summary`, `transaction_daily_summary`, `vw_monthly_member_penetration`, and monthly summary ingestion functions and triggers. It creates no segment table. A disposable PostgreSQL 16 run applied it successfully; the integration test then loaded synthetic data and queried both domains.

The service and migration Docker images built locally from this repository. The service image started against the disposable database; readiness returned 200 and a Self-Pilot authenticated Universe request returned `status=complete`, four OpCo cards, and a matrix. This confirms a local smoke path only, not Partner Portal or public gateway behavior.

The repository only contains local synthetic scripts for Customer Intelligence; no production loader or owning data pipeline was found in `cdp-be`. The team must identify who populates every reporting table, provision initial data into the dedicated eco database, and keep it fresh after cutover. For the later release, the member cookbook's local import script is only a test helper; the production feed owner remains unknown. Segment definitions, member attributes, requests, identifiers and sequence values require a separate migration and reconciliation plan.

The external gateway configuration is in `cdp-k8s-manifests/overlays/{dev,uat,prep,prod}/kong-ingress/cdphub-service.yml`. The ingress currently routes `/contextiq` and `/users` by broad prefixes to legacy services and strips the matched path. Its `CODEOWNERS` assigns ingress changes to `@akmal.harith`. A new rule must preserve the public v0 path and the service's `/api/v0/...` upstream path without stealing legacy `/api/v1` traffic. No gateway file was changed.

The CI files follow the ACS pattern but the shared `dso-cicd` template, GitLab project settings, runner permissions, registry name, and new deployment manifests remain unverified. The new GitLab project, separate test environment, pipeline image publication, data sync, response parity, live Partner Portal auth latency, backup/restore, write recovery, and routing cutover have not been executed. Confirm the intended public URL and gateway rewrite with the ingress owner before adding external servers to OpenAPI or moving traffic.
