# Segment and dashboard scale plan

## Local implementation status (2026-10-21)

The prototype now has the Slice 1 category/patch read model and draft custom creation; Slice 2 versioned source tables, publication record, SQL set evaluator, pinned confirm/export jobs, and current-consent check; and a Slice 3 Roaring builder/reader plus stored dashboard scopes and monthly revisions. The local B21 fixture checks computed count precedence, exact SQL/bitmap IDs, consent, dashboard scopes, export, failed-build rollback, and READY-row immutability. Both build-backed API and worker require `APP_ENV=local`; request status now requires the segment permission.

**This is not a Slice 3 release.** The organisation guard, dashboard serving API and live scope computation, erasure/withdrawal across retained builds and finalized exports, retention cleanup, custom bitmap policy, per-instance memory budget, small-group rules, and 5M/10M/20M benchmarks remain open. Build-scoped rows use ordinary tables and foreign keys; partition interval and index strategy still need measured workload plans. The build fixture is synthetic and has no approved production predefined ID. Keep real member IDs and non-local build flags disabled until these gates pass.

## Decision and current state

The phase 1 programme has three delivery slices: catalogue listing; versioned feeds and SQL evaluation; then backend-built Roaring bitmaps, dashboards, and the organisation guard. Use SQL on the same immutable build as the correctness oracle and for predicates the bitmap catalogue cannot represent. Benchmark both paths at 5M, 10M, and 20M members. Each build has one READY publication after **its recorded required outputs** reconcile; the required set expands as later slices ship. The previous SQL-only benchmark gate is superseded by this decision.

**Scope decisions:** (1) Slice 1 tests segment listing using patched catalogue data, with no manual or automated member build. (2) Organisation scoping is not enforced in Slices 1–2; those slices use internal access and non-production member data only. The org guard is required before real member-ID access or exports. Schema and evaluator scope hooks are present from the start.

The legacy v0 path still reads `segment_members_v0` and `member_cookbook_v0`. The local build-backed path reads versioned `member_facts_v0` and related tables, evaluates SQL sets in PostgreSQL, and uses stored Roaring bitmaps when the build manifest includes them. `000003_segment_v0` now contains the ordinal map and bitmap table. PostgreSQL's transient bitmap index scan remains different from this stored Roaring path. The local million-member fixture is one functional legacy scale point, not evidence for 5M–20M latency or concurrency. There is no segment builder UI in this repository.

The supplied three-member, B21 table sample is a reference, not production seed data or executable schema. Its ordinal, bitmap, `customer_daily`, `agg_*`, and widget tables are proposed, not existing schema. Gender and Age Band already have approved option rows in `segments_v0` via `000004_attribute_segments`; Income Band does not. `seg_000001` is a local synthetic fixture, not an approved production credit-card segment ID. Each category groups options, while the stable opaque segment ID identifies an option. `segment_field_catalog_v0` holds fields/operators for filters, and `feature_summary` holds Customer Intelligence widget metadata. Neither is the segment option catalogue.

This document owns all unfinished work carried forward from [the implemented v0 record](segment-v0-migration-plan.md). The v0 migrations and routes are implemented locally, but the production feed, target environments, and this scale design are not deployed. Because the segment migrations have only run locally, the catalogue seed and Slice 1 schema foundation are being consolidated into `000003`–`000004`; later deployed changes will need forward migrations. The local database that recorded the earlier version 5 will not replay the edited files automatically.

## Delivery slices

| Slice | Scope | Not included yet |
| --- | --- | --- |
| 1. Segment listing with patched data | Local `000003`–`000004` schema foundation for stable category codes, canonical `kind`, definition version/owners, and patch provenance. The 27 approved options remain seeded in `000004`. Complete the list/get and draft-only custom API transition, and load any sourced patch counts with a reviewed script. | Build job, member facts, ordinals, bitmaps, dashboards, estimate and confirm remain disabled. Draft customs have no count. |
| 2. Data feed, builds, SQL counts | Feed contract, `segment_build_v0`, versioned member facts and relations, SQL counts, estimate, confirm, export jobs, current-consent check. Computed counts replace patched ones. Internal testing uses non-production member data. | Bitmaps, dashboards, org guard, and production access to real member IDs. |
| 3. Bitmaps, dashboards, org guard | Ordinals and bitmaps with SQL parity, benchmark at 5M/10M/20M, dashboards joining the required-outputs list, and the organisation guard. Real member-ID access and export activate only after the guard and its tests pass. | |

Rules for slice 1:

- **Patched counts must be labelled.** Put approved patch counts in `segment_count_patch_v0` with their population/consent basis, as-of date, and source. The read model returns `count_source=patched` until a computed READY build count is available, then returns `count_source=computed` from `segment_build_count_v0`. An absent count has no source and must be returned as unavailable, not zero; a draft custom segment therefore has no count.
- **Use final IDs now.** Later builds, recipes and saved custom segments attach to these IDs. Replace the local `seg_000001` fixture with an approved production ID before patching.
- **Patch through a reviewed forward migration or seed script** with its source recorded. Do not hand-edit shared environments, and do not load synthetic members into dev or UAT.
- **Create drafts only.** A Slice 1 custom definition has an ID and rules but no evaluated count; estimate, confirm and export remain disabled. Make `estimated_count` nullable or omit it for drafts in the API contract. The legacy `NOT NULL DEFAULT 0` column and non-null response remain temporarily for API compatibility; change the read model before claiming this rule is implemented.

Rules while organisation scoping is open:

- **Keep the hooks.** Put `org_id` (default meaning "all") on requests, recipes, `segment_build_count_v0`, `dashboard_result_v0` and any per-build visibility table from the start, because adding it later to large partitioned tables is expensive. The evaluator and cache keys already take a scope `{org_id, retail_ids}`, with the open default. One setting in one place decides whether the org filter is applied.
- **Consent still applies** to counts and exports.
- **Keep access internal.** Slices 1–2 use non-production member data. Do not route any segment endpoint, including the currently unauthenticated request-status endpoint and its export reference, to a broader audience. Real member-ID pages and exports stay disabled until the org guard is active; the caller's org comes from trusted identity, and all-scope access later requires an explicit privileged grant.
- **Keep an audit trail** (`segment_usage_v0` plus request and export records) so the later usage cross-check has data.

## Ownership and data publication

| Boundary | Data team | Backend team | Shared contract |
| --- | --- | --- | --- |
| Source facts and classifications | Load versioned member facts, consent, retail, and organisation visibility; keep its `load_log` for source ingestion. Compute classifications assigned to it. | Define ingestion schema, validate source loads, and publish only complete readable builds. | Member IDs, source date/version, freshness, null/unknown values, duplicates, org scope, correction and replay rules. |
| Attribute definitions | Propose categories/options and compute classifications assigned to it. Preserve backend custom rows during loads. | Govern stable category codes, option IDs, API visibility, custom definitions, and backend-computed rules. | Record the definition owner and computation owner separately for each option; version approved definitions. |
| SQL membership | Deliver relationally queryable facts or approved option memberships. | Own migrations, indexes, SQL evaluation, jobs, and retention. | Reconcile row and distinct-member counts plus sampled IDs before publication. A bitmap-only feed cannot serve as a SQL baseline. |
| Phase 1 bitmaps | No bitmap production in the selected backend-built path. If an external producer is later chosen, build artifacts from a published, versioned ordinal map. | Assign ordinals; build bitmaps from validated immutable PostgreSQL rows before publication; own reader, authorization, and ID resolution. | The producer owns the artifact lifecycle regardless of whether bytes live in PostgreSQL or object storage. Freeze encoding, mapping version, build ID, checksums, and publication. |
| Dashboards | Supply source facts and agreed upstream metric definitions. | Own published aggregate results and dashboard serving. | Reconcile against the same source snapshot and population/scope rules. |

Before adding schema, agree on the relational shape of upstream attributes. Prefer one member-facts row per build and member with raw columns plus approved IDs for single-valued categories, and a relation for genuinely multi-valued retail/product/organisation data. Keep raw income, spend, dates, and other facts required for free ranges or sums. A saved custom segment has its own ID and references selected option IDs or field/operator/raw-threshold predicates. The backend derives backend-owned age bands from birth date at `source_date`; an upstream-owned classification is accepted or rejected against its own agreed contract, not silently recomputed by a second owner.

## Phase 1 table contract to draft

| Table | Identity and purpose | Ownership/transition |
| --- | --- | --- |
| Data-team `load_log` | Source load completion, source version, and build ID. | Data team writes; backend waits for every required source table. This remains separate from backend publication. |
| `segment_build_v0` | `build_id` PK, `source_date`, status, source counts, `published_at`, optional `supersedes`, duration/errors, and an immutable required-output manifest/version. | Backend writes LOADING/FAILED/READY and atomically advances the current READY pointer. A corrected same-day load has a new ID. Slice 2 requires source validation and SQL segment counts; Slice 3 adds bitmaps and required dashboards. Freeze the list when build work starts and publish only after every listed output reconciles. |
| `segment_attribute_category_v0` and `segments_v0` | Stable category codes and segment IDs; canonical `kind` for option/predefined/custom, category reference for options, definition version, definition owner, computation owner, rule/source. | Backend governs definitions; data team may supply approved predefined/classified data. Migrate current reserved tags and `fixed_rule` check compatibly; do not leave two conflicting classifiers. Deprecate the legacy `estimated_member_count` as an authoritative value; use patch/computed count relations and return no count for drafts. Migrate its current `NOT NULL DEFAULT 0` behavior and the API compatibly. |
| `segment_count_patch_v0` | `patch_id` PK and at most one active patch per segment/org; count, population basis, as-of date, patch source/reviewer, and replacement status. | Backend applies reviewed seed/migration. Patched counts never enter `segment_build_count_v0`; the read model prefers a computed READY-build count when one exists. |
| `segment_build_count_v0` | `(build_id, segment_id, definition_version, org_id)`, unconsented and build-consented distinct-member counts. | Backend derives from the same build and records `count_source=computed`. Use an explicit ALL scope key instead of nullable `org_id` in the composite identity. Patched counts never enter this table. |
| `member_facts_v0` | `(build_id, member_id)`; raw columns, approved single-valued option IDs, build consent. | Immutable source/derived rows for SQL, bitmap construction, free ranges, and audit. Exact columns follow the feed contract. |
| `member_org_v0`, `member_retail_v0`, `member_product_v0` | Build-scoped member-to-org, member-to-retail/activity, and member-to-product relations. | Required where the agreed org/retail/product predicates and dashboard widgets need them. Define distinct-person counting and referential rules. These are proposed, not current segment migrations. |
| `member_ordinal_v0` and `member_bitmap_v0` | Permanent never-reused `member_id ↔ ordinal`; immutable `(build_id, predicate_kind, predicate_id)` bitmap, cardinality, encoding and definition versions. | Backend builds from validated unpublished rows. Category names are obtained through catalogue joins, not copied into bitmap keys. |
| `dashboard_result_v0`, `customer_daily`, `agg_*` | Results and sparse cells keyed by build, org/scope, and metric/dimension IDs. | Built only once dashboards are included in that build's required-output manifest. Exact shapes follow metric definitions. |
| `customer_monthly_summary_v0` | Month/org/retail metric with source build and revision provenance. | Keep beyond raw-build retention; a correction replaces the month's current revision and retains an audit trail according to policy. An unpublished revision is invisible until its build becomes READY. |
| Requests, recipes, exports | Existing IDs plus pinned `build_id`, definition versions, build count, current-consent count, and `consent_checked_at` where applicable. | Confirm creates durable rows; estimate remains a synchronous response without a request row unless product explicitly approves history. Migrate historical `snapshot_date` fields compatibly. |

Use `(build_id, predicate_kind, predicate_id)` for bitmap lookup. For Female, `predicate_id=seg_100016`; `GENDER` is catalogue metadata. A B-tree locates bitmap rows and resolves ordinals, while a Go Roaring library performs set operations. The sample's `kind/key/value` triple is not the target schema.

### Proposed table structure by slice

This table remains the **logical target**. The local migrations now contain the category, patch, build, fact, ordinal, bitmap, and dashboard tables, and list/get prefers computed build counts over labelled patches. The concrete local shapes are a prototype; types, indexes, partition keys, exact feed columns, and dashboard metrics still need the production contract and benchmark. Keep the data team's actual `load_log` under its own contract; `segment_source_load_v0` is the local prototype equivalent.

| Slice | Table | Key and essential columns | Example row / purpose |
| --- | --- | --- | --- |
| 1 | `segment_attribute_category_v0` | `category_code` PK; `display_name`, `sort_order`, `status` | `GENDER`, `Gender`: one category for its option rows. |
| 1 | `segments_v0` | `id` PK; `kind`, `category_code` nullable FK, `name`, `definition_version`, `definition_owner`, `computation_owner`, `rule_json`, `status`, visibility/tags | `seg_100016`, `option`, `GENDER`, `Female`, version 1. A custom has its own ID and rule references; no category is required. |
| 1 | `segment_count_patch_v0` | `patch_id` PK; `segment_id`, `org_id`, `member_count`, `population_basis`, `as_of_date`, `source_ref`, `reviewed_by`, `status`; at most one active patch per segment/org | `seg_100016`, `ALL`, 1234, `build_consented`, 2026-10-20, `approved-report-42`: **illustrative patch only**, never a B21 result. |
| Existing | `segment_field_catalog_v0` | `field_key` PK; `data_type`, `allowed_operators`, value bounds/source mapping | `annual_income`, `numeric`, `[gte,lte]`: permits a free-range rule; it is not the Gender/Age option list. |
| 2 | `segment_build_v0` | `build_id` PK; `source_date`, `status`, `required_outputs`, manifest version, source counts, `published_at`, `supersedes` | B21, 2026-10-21, READY, `[source,sql_counts]` in Slice 2. A separate current-build pointer changes atomically. |
| 2 | `member_facts_v0` | `(build_id,member_id)` PK; raw gender/birth date/income/spend, approved single-valued option IDs, `build_consent` | B21/M001, Female `seg_100016`, age `seg_100004`, consent true. Preserve raw values for free ranges and sums. |
| 2 | `member_org_v0`, `member_retail_v0`, `member_product_v0` | `(build_id,member_id,org/retail/product_id)` keys; relation-specific facts such as `last_txn_date` | B21/M001/big and B21/M001/co: two retail rows, one person. Org rows become the visibility input when the guard is on. |
| 2 | `segment_build_count_v0` | `(build_id,segment_id,definition_version,org_id)` key; `raw_count`, `build_consented_count`, computed timestamp | B21/`seg_100016`/v1/ALL → 2 raw, 1 consented. Only computed counts belong here. |
| 2 | `segment_requests_v0`, `recipes_v0` | Existing IDs plus `build_id`, `org_id`, frozen definition versions/rule payload; requests carry status and counts | Confirm request `req_B21_01` pins B21. Estimate returns a response and creates no request row. |
| 3 | `member_ordinal_v0` | `member_id` unique, `ordinal` unique, retirement/tombstone state | M001 → 1; assignment is permanent and ordinal 1 is never reused. |
| 3 | `member_bitmap_v0` | `(build_id,predicate_kind,predicate_id)` key; `definition_version`, `encoding_version`, `cardinality`, checksum, payload/location | B21/option/`seg_100016` → `{1,3}`. Its category name is read from the catalogue. |
| 3 | `dashboard_result_v0` / sparse cell tables | `(build_id,org_id,retail_scope,metric_id,dimension_ids)` logical key; result, provenance | B21/ALL/ALL/`member_count` → 2 consented distinct people. `missing` is a real dimension bucket. |
| 3 | `customer_monthly_summary_v0` | `(month,org_id,retail_scope,metric_id,revision)`; `source_build_id`, value, published/current state | Corrected B22 creates a new revision for the same month, then replaces the visible revision at publication. |

`org_id=ALL` is an explicit reserved scope key, not SQL `NULL`; a real caller can use it only with the eventual privileged grant. The bitmap payload may be PostgreSQL `BYTEA` or an object reference after measurement. Requests and exports retain the pinned build and definition versions; an export also records its current-consent check. The example patch count 1234 is arbitrary and must not be seeded without an approved source.

### Small worked example (illustrative, not seed data)

Assume B21 has `source_date=2026-10-21`, all three members are in the permitted organisation scope, and the dashboard's default population is build-consented members. These rows show only columns needed for this example. `seg_100016` (Female), `seg_100015` (Male), `seg_100004` (age 26–35), and `seg_100005` (age 36–45) are current catalogue IDs. No Income Band option ID is assumed.

| `segment_build_v0` build | Status | Required outputs frozen for this example | Source date |
| --- | --- | --- | --- |
| B20 | READY, current until B21 publishes | Its own recorded manifest | 2026-10-20 |
| B21 | LOADING → READY after validation | Source + SQL counts in Slice 2; source + SQL counts + bitmaps + dashboard results in Slice 3 | 2026-10-21 |

| `member_facts_v0` build/member | Gender option | Age option derived at source date | Build consent | Illustrative ordinal |
| --- | --- | --- | --- | ---: |
| B21 / M001 | `seg_100016` | `seg_100004` | true | 1 |
| B21 / M002 | `seg_100015` | `seg_100004` | true | 2 |
| B21 / M003 | `seg_100016` | `seg_100005` | false | 3 |

| `member_retail_v0` build/member/retail | Last transaction | Interpretation |
| --- | --- | --- |
| B21 / M001 / big | 2026-10-09 | Active in the illustrative 30-day window |
| B21 / M001 / co | 2026-10-09 | Same person, second retail |
| B21 / M002 / big | 2026-10-12 | Active; fixes the original `big` mismatch |
| B21 / M003 / bank | 2025-12-01 | Inactive and without build consent |

In Slice 2, SQL computes counts from B21. In Slice 3, the backend produces matching bitmaps. `member_ordinal_v0` resolves ordinal 1 to M001. A B-tree finds each bitmap by `(build_id, predicate_kind, predicate_id)`; Roaring intersects the sets.

| `segment_build_count_v0` build/segment/scope | Raw distinct members | Build-consented members |
| --- | ---: | ---: |
| B21 / `seg_100016` / ALL | 2 (M001, M003) | 1 (M001) |
| B21 / `seg_100004` / ALL | 2 (M001, M002) | 2 (M001, M002) |

| `member_bitmap_v0` key | Ordinals | Meaning |
| --- | --- | --- |
| B21 / option / `seg_100016` | `{1,3}` | Female option membership before consent |
| B21 / option / `seg_100004` | `{1,2}` | Age 26–35 option membership before consent |
| B21 / consent / `yes` | `{1,2}` | Build-consented members |
| B21 / retail / `big` | `{1,2}` | Distinct `big` members |
| B21 / activity / `active_30d` | `{1,2}` | Active members at B21 source date |

| Flow/use case | Query and result |
| --- | --- |
| Listing | `segments_v0` labels Female `seg_100016`; a Slice 1 patched display count needs explicit provenance/population. A draft custom row has no count. |
| Estimate | `base=all_members`, Female AND age 26–35 AND build consent: `{1,3} ∩ {1,2} ∩ {1,2} = {1}`; response is count 1, B21/source date, and no request row. |
| Confirm/export | Confirm pins B21 and definition versions in a durable request. The worker selects M001; before exporting it checks M001 against the authoritative current-consent source. If unavailable, export fails. |
| Dashboard retail scope | For the build-consented population, `big=2` (M001, M002), `co=1` (M001), `bank=0`, and `ALL=2` distinct members. `big ∪ co=2`, not 3. The full eligible-retail set and `ALL` are one logical scope. |
| Dashboard cells | Disjoint gender × age cells contain Female/26–35=1 and Male/26–35=1; Female/36–45=0 after consent. Each axis also has `missing` for null classifications. The cells sum to `ALL=2`. |

The bitmap rows are build-scoped; the permanent ordinal map is not. The dashboard can only serve these B21 results when its outputs were included in B21's frozen required-output manifest. An estimate on B21 and a later request on a corrected B22 can legitimately differ; a confirmed B21 export remains pinned to B21 and still rechecks current consent.
The one-person result demonstrates set correctness only; production small-group rules may hide its count and forbid member-ID drill-down/export.

## Carried-forward v0 release work

These are the remaining items from the v0 implementation record. This is release hygiene and has its own gate, separate from the scale architecture below, so a delay here is not a delay to the bitmap design. They belong to the database or API/job priority below; completing them does not claim that the 5M–20M SQL design is finished.

| Priority | Open work and acceptance |
| --- | --- |
| Database and catalogue | Inventory local, dev, and UAT segment IDs, sequence values, definitions, custom rows, field-catalogue rows, synthetic members, memberships, requests/recipes, JSON IDs, active jobs, and exports before loading another environment. Check target-ID collisions and duplicate/test rows. Approve a real `segment_field_catalog_v0` seed separately from local fixtures. Apply existing migrations through `000004` on a fresh and a populated target with segment switches off; reconcile 27 visible attributes, one hidden base, all references, and collision-free custom IDs. Dev/UAT connections were not available during local work. |
| Database and feed | Agree the real member ID format, source writers, consent meaning, org/retail coverage, completed-load signal, and snapshot publication. Keep local synthetic members and `seg_90000*` scale rows out of shared environments. Decide whether historical ContextIQ requests or finalized exports must remain accessible after cutover. Schema deployment can precede the real feed; traffic activation cannot. |
| API and job local gate | Add the missing GCS adapter test for writer-close failure and object metadata/size verification. Repeat live HTTP smoke for the revised 27-option catalogue, then rebuild service and migration images from the final reviewed revision. The earlier local integration, retry, migration, million-member, and route checks passed, but they do not close this adapter gate. |
| API and job environment gate | With switches still off, verify stable Partner Portal subject for `TEXT` caller columns, real GCS writer and metadata permissions, actual container memory, gateway prefix/rewrite, route/error/status compatibility, and any legacy in-flight jobs. Then enable one worker cohort and the five routes. Preserve request-ID-only status polling as the approved v0 contract. Monitor queue age, retries, stuck stages, export duration, object counts, and rollout reconciliation. |

Do not load synthetic fixtures into dev/UAT merely to exercise the catalogue; any end-to-end shared-environment sample needs an explicit, isolated member snapshot. Do not rewrite completed request JSON or delete historical segment rows without reconciling consumers and export objects. Migration `000004` refuses rollback while catalogue rows exist; correct populated environments with reviewed forward changes.

## Priority 1 — database, bitmap builder, and publication

### A. Establish a correct snapshot

1. Inventory writers for `segment_field_catalog_v0`, `segments_v0`, `member_cookbook_v0`, `segment_members_v0`, and application-owned requests, jobs, recipes, members, and usage. Identify the real loader for the sample's `member_retail_v0`, `member_org_v0`, and load-completion signal; those tables are not in the current segment migration.
2. Introduce `build_id` as the identity for a completed source load; retain `source_date` as business metadata for age and reporting. Keep the data team's `load_log` for "source loaded" and a separate backend publication record for "safe to read". Version the underlying member facts, classifications, retail/org visibility, and other query inputs by build; a READY record next to the live, member-ID-keyed `member_cookbook_v0` does not freeze it. Validate counts and references before backend publication. A corrected load on the same date gets a new build ID.
3. Pin all reads in one request to the current READY build. Keep B20 serving until all outputs on B21's frozen required-output manifest finish and reconcile; failure leaves B20 current. Dashboards cannot block a Slice 2 build because they are not yet required. Set a maximum acceptable build delay and an alert when freshness exceeds it. Put `build_id` on versioned source rows, segment membership, dashboard results, bitmaps, recipes, requests, and export metadata as applicable; keep `source_date` for business meaning. Migrate current request `snapshot_date`/`snapshot_dates` history compatibly. Decide whether source rows are retained as immutable per-build partitions or another measured versioned form; a transaction snapshot does not persist across a later worker run.
4. Specify age boundaries, proposed income bands, consent, organisation visibility, missing values, multi-valued categories, mutually exclusive options, and correction/replay behavior. Approved option IDs are categorical identities; raw gender, birth date, and annual income remain source facts when supplied.
5. Finish the API transition to stable category codes already referenced from `segments_v0` in local `000004`; `category_name` remains temporarily for the current list/get query and must be removed after the join-based read is deployed. Keep `segments_v0.id` as the option identity. Canonical `kind`, definition owners, and version are in the local schema; migrate tag-dependent API behavior before retiring reserved tags. The visible-attribute check now permits an upstream-computed option without `fixed_rule`. Update list/get and custom-definition API behavior before activating Slice 1. Use forward migrations once this schema has been deployed outside local development.
6. Define privacy and erasure behaviour for retained source rows, bitmaps, cached results, stored counts, dashboards, exports, and the permanent ordinal map. Tombstoning an ordinal prevents ID resolution but leaves it in historical bitmap cardinalities, so either rebuild/invalidate affected outputs or apply a suppression set at every read and recompute counts. A withdrawn or erased member must not reappear through an old pinned build, cache, or export. Never reuse an ordinal. Define how already finalized export objects are handled. Current-consent checking covers revocation at export but does not by itself implement deletion.

### B. Optimize SQL as evaluator and correctness oracle

1. Build a workload matrix: common and rare options; `AND`/`OR` combinations; fixed rules plus numeric thresholds; org scope and consent; empty and overlapping sets; unknown values; historical snapshots. Preserve the current ordered, left-associative semantics unless the API contract is deliberately revised.
2. Evaluate counts and member selection in PostgreSQL rather than collecting millions of candidate IDs in Go maps. Compare query forms such as joins, `EXISTS`, `INTERSECT`/`UNION`, and candidate-first filtering with `EXPLAIN (ANALYZE, BUFFERS)`. Verify distinct-person counts after retail and multi-valued joins.
3. Add B-tree indexes for measured predicates: snapshot/build, org scope, option/segment ID, and member ID, with column order based on actual plans. Existing primary keys and `idx_segment_members_v0_snapshot` are a starting point. Avoid speculative indexes on every low-cardinality column. Evaluate partitioning only after measuring retention, load cost, and pruning. Keep database constraints for small catalogues and build metadata; benchmark foreign-key cost on large partitions before deciding whether job validation can replace any of those constraints.
4. Decide build retention before choosing partitions. Size full daily member facts and `segment_members_v0`: members/options × retained builds, including indexes; 14 full builds at 5M members mean 70M member-fact rows before indexes. Test partition-per-build and coarser intervals against load time, pruning, and partition-management cost. Materialize only memberships the feed or measured workload requires. Preserve request and recipe history for agreed retention. Never expire a build referenced by an active job; if that is impossible, durably materialize the confirmed IDs before expiry.
5. Benchmark SQL and Roaring separately and together at 5M, 10M, and 20M, including skew, sparse options, org overlap, retained snapshots, and concurrent users. Agree targets first. Measure p50/p95/p99 count and first-page latency, full export throughput, CPU, memory, I/O, temp files, index/table size, load/publication time, and plan stability. Treat a full ID export as asynchronous throughput work.

**Database gate:** a complete build is validated and all outputs on its frozen required-output manifest publish together. Slice 2 proves SQL IDs/counts; Slice 3 proves bitmap/SQL exact-ID and count parity for supported predicates. Pending jobs can replay retained builds. Performance meets the agreed targets at 5M/10M/20M. Test fresh and populated upgrades, retries, partial-load rejection, source reconciliation, and erasure.

### C. Build Roaring bitmaps in phase 1

1. Keep SQL as the correctness oracle and explicit fallback for unsupported predicates such as free ranges. State which filters are bitmap-backed (approved options, agreed bands and fixed thresholds such as spend above 50, 100, 200) and which stay SQL-only (true free ranges and sums, a rare and separately benchmarked case). Do not pass large bitmap results into SQL filters; if a hybrid is needed, specify how the two results are combined and benchmark it. Roaring helps prepared integer-set operations; it does not solve source freshness or export volume.
2. Add `member_ordinal_v0` in phase 1. Backend assigns stable, never-reused ordinals with unique member-to-ordinal and ordinal-to-member lookups. Decide 32-bit versus 64-bit capacity and retirement rules before allocation; allocation has one owner. The backend-built path uses its own validated PostgreSQL rows, so no daily mapping handoff is needed.
3. Backend builds immutable bitmaps for approved attribute option IDs, organisation visibility, build consent, retail/activity, approved predefined segments, and a bounded eager set of recently used custom segments from the same **unpublished** build. Define the eager-custom selection, on-demand computation, cache key, invalidation, and limits with the data team and product before coding. Attribute bitmap identity is build ID plus `segments_v0.id`; retail, consent, and organisation predicates use typed IDs. Define encoding, checksum, cardinality, source/definition versions, and complete-build manifest. If an external producer is later chosen, supply a versioned ordinal mapping artifact and reject every mismatched or unknown ordinal before publication.
4. Choose PostgreSQL `BYTEA` versus object storage from measured size, fetch latency, and cache behavior; storage location does not decide the producer. A B-tree on `(build_id, predicate_kind, predicate_id)` locates bitmap rows or metadata; B-trees on ordinal and member ID resolve results. A Go Roaring library performs set `AND`/`OR` and combined cardinality in this design; do not assume a PostgreSQL Roaring extension. Individual stored cardinalities cannot be added to produce a combined count. PostgreSQL's Bitmap Heap Scan is unrelated to this stored-payload path.
5. Validate cardinalities and exact IDs against SQL on the same immutable build: deterministic boundary, zero, overlap, org, consent, and correction cases plus sampled requests. A fixed number of random cases alone is insufficient. Publish one complete build atomically, pin build and definition versions in requests, reject mixed/partial builds, and retain mapping/history for queued jobs. Recheck consent against an authoritative **current** source at export; fail the export if that source is unavailable. Unsupported dynamic filters require an explicit, benchmarked SQL or hybrid path.

### D. Daily build job order

1. Reserve one build ID, freeze its slice-specific required-output manifest, and create its LOADING publication record before loading rows. Keep foreign keys on small catalogue and publication tables; benchmark their cost on large source partitions before replacing any with equivalent pre-publication job validation. Data team loads all required versioned source tables and marks each complete in its `load_log`; the backend claims the build only when every required source is complete. Unpublished rows remain invisible to readers.
2. Validate source uniqueness and referential integrity, raw/option contracts by computation owner, source-date rules, expected counts, and required org/product/retail coverage. Derive backend-owned options such as Age Band from raw facts.
3. Produce SQL memberships and `segment_build_count_v0` from validated **unpublished** rows in Slice 2. In Slice 3, also assign ordinals to new members once and build approved predicate bitmaps, predefined memberships, and the bounded eager-custom set.
4. Reconcile counts and exact IDs with the versioned SQL source. In Slice 3, reconcile bitmap results with SQL and compute dashboard results and a hidden monthly revision only when those outputs are in this build's manifest. A mismatch in any required output fails the build while the previous READY build remains current.
5. In one database transaction, mark the build READY, advance the current-build pointer, and expose any required monthly revision after every output on the build's required-outputs list is complete. After publication each serving instance reads the pointer and pins each request to one build; in Slice 3, it also warms or lazily loads that build's bitmaps within a per-instance memory budget. Record timing, sizes, and errors; alert on failure or freshness delay.
6. Retain source partitions and bitmap artifacts while any request/job pins the build. After the agreed period and reference check, retire them while preserving recipe/export and monthly-summary provenance.

## Priority 2 — segment API and jobs

1. In Slice 1, serve list/get and draft-only create; enable estimate/confirm only with the versioned SQL build in Slice 2. Expose stable category codes and option IDs. A saved custom segment retains its own segment ID and references selected option IDs or raw field/operator/threshold predicates. Validate scope and allowed filters/operators. Define an explicit `base` in the request (`all_members` or a segment ID) so an unsaved selection such as Race = Malay can start from all members, and keep the three user-facing groups (predefined segments, explore by attribute, custom segments) backed by one request shape. Define a bounded ID-page contract separately from a full export; avoid unbounded synchronous ID responses. Real member-ID responses and export stay disabled until the Slice 3 org guard passes.
2. Run estimates as SQL counts in Slice 2 and bitmap or benchmarked SQL/hybrid counts in Slice 3 against one READY build. Apply **build-scoped** consent in both slices; apply org visibility only when the Slice 3 guard is enabled. Return the as-of date and build identity in response metadata so same-day corrections are distinguishable. Estimate stays synchronous and creates no request row unless a separate product decision approves history. Cover empty sets, duplicates, mixed `AND`/`OR`, dynamic ranges, broad/selective options, and definition changes. Set query timeouts and errors for unavailable builds. Add guardrails: a maximum number of filters, result and page size limits, rate limits, and small-group hiding across counts, ID pages, drill-downs, and exports. Decide on row-level security with the org guard.
3. Pin one `build_id` and definition versions on confirm, then use that build throughout worker re-evaluation and export. Protect it from retention cleanup while a job is pending or running; materialize candidate IDs durably before expiry only if a longer job must outlive its build. Process jobs in bounded batches or streams with deterministic ordering, retry/idempotency, progress, and no whole-result Go map. Recheck authoritative **current** consent before final export, fail closed when that source is unavailable, store `consent_checked_at`, reconcile CSV rows and object metadata, and report both build-consented and final export counts when they differ.
4. Verify exact IDs and counts across SQL query strategies and, in Slice 3, SQL/Roaring. Test concurrency, long exports, worker restarts, expired snapshots, load failure, org isolation, and rollback at 5M/10M/20M. Audit consumers before retiring `recipe_members_v0` or `segment_members_v0`.

**API/job gate:** interactive operations meet their targets; jobs finish and retry within budget; IDs/counts match the selected snapshot; consent remains correct under concurrent loads and requests. Slice 2 is internal with non-production member data. Production member-ID access requires the Slice 3 org authorization and small-group guardrails to pass.

## Priority 3 — dashboards

1. Define widget metrics, population/scope, dimensions, allowed filters, freshness, and drill-down. Share approved category option IDs and source snapshot identity with segmentation. Dashboard code can ship separately, but once its outputs are required for READY they publish with the segment outputs from the same build.
2. Prepare daily aggregates for repeated charts and totals. Size `customer_daily` dimension combinations and `agg_*` tables before implementation; avoid a dense cross-product of gender, age, income, retail, and org. Store the finest disjoint cells (for example gender x age band x income band, each with a `missing` value so totals reconcile with the unified total) and compute ratios from summed counts, never by averaging percentages. Define a canonical sorted retail-set scope for each requested union, counting distinct members once. `ALL` means the union of every eligible retail in the agreed population, not a sum of retail counts; alias the full-set scope to `ALL` instead of storing two copies. Four retails permit 15 non-empty scopes, six permit 63, and eight permit 255; benchmark the scope count multiplied by dimensions and orgs before deciding which subsets to precompute. Above about eight retails, precompute singles and `ALL`, then compute other combinations live and cache them. Carry the complete scope in cache keys. Reference approved option IDs/codes and derive display labels from the catalogue.
3. Membership and distinct-person counts use the same build-pinned segment evaluator, with SQL as the cross-check. Sums, ratios, and trends use reconciled SQL facts/aggregates for that build. Once dashboards are on the required-outputs list, complete every required dashboard output before the single READY publication. Reconcile totals and drill-downs with the matching segment build and scope; expose the as-of date on every widget and preserve the build ID in response metadata for same-day corrections. Benchmark refresh time, storage, page latency, concurrency, and correction/rebuild behavior at 5M/10M/20M. If dashboard time breaches the agreed freshness budget, revisit the one-READY decision explicitly.

**Dashboard gate:** each widget reads a complete build, its counts match agreed definitions, drill-down respects org visibility, and refresh/query costs fit budget.

## Decisions to close before implementation

1. **Data-team handoff first:** where is authoritative current consent read at export time, how fresh is it, and what is its unavailable/revocation behavior? The planned behavior is to fail the export if the check cannot complete.
2. **Custom policy second:** which customs are eagerly built each load, which are computed on demand, and what are the cache, definition-version, and retention limits? Record the definition and computation owner for each approved option.
3. What are the p95/p99 latency and concurrency targets for counts and first ID pages, the maximum single-READY build delay, and the deadlines for exports and dashboard refresh? Suggested starting target to confirm: p95 estimate within 1 to 2 seconds at 5M members for an agreed filter shape, concurrent load, and warm/cold-cache condition.
4. Which raw facts and approved option IDs arrive from the data team; which options are backend-derived; what are the real org, retail, and product sources; and which income bands, multi-valued categories, and dynamic filters are approved?
5. How many immutable builds are retained, which partition interval performs best, and what prevents expiry while a request/job pins a build? Which production predefined IDs replace local fixture IDs?
6. Which bitmap encoding/ordinal width and payload location meet measured size and latency budgets? If an external producer is deliberately chosen later, how is the versioned ordinal map delivered and verified?
7. **Required outputs per build:** which outputs must exist before READY for slice 2 and slice 3, and when dashboards join the list.
8. **Privacy and erasure:** the deletion and consent-withdrawal rule across retained builds and the ordinal map.
9. **Bitmap-backed versus SQL-only filters:** the approved list, including bands and thresholds.
10. **Slice 1 patch data:** the approved production IDs, names, categories, owners and patched counts, with their source.
11. **Org guard timing:** when it is enabled, who can use the API until then, and the audit kept meanwhile.

## Fixture correction before migration drafting

The supplied B21/M001–M003 rows are reference data. Before turning them into a checked-in fixture, make `big` membership and its bitmap agree; adding M002 with a qualifying transaction also makes the sample's active bitmap plausible. The credit-card/Female intersection contains M003, whose build consent is false, so its consented count is zero. Replace local-only `seg_000001` with an approved production predefined ID. Keep the estimate example as a response only, since v0 does not persist estimate requests. Verify all org, retail, product, activity, dashboard, and monthly counts against the corrected three-member truth table; compare exact IDs, not just totals.

**Implementation order:** approve IDs, patched count provenance, and the draft-only API contract before the Slice 1 migration. Settle the versioned feed, option computation owners, and authoritative current-consent source before the Slice 2 migration and jobs. Define eager-custom and bitmap cache policy before Slice 3. Correct and approve the reference fixture before checking it in or using it to validate any slice. No migration should encode an unresolved owner or consent source.
