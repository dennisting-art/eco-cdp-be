# Segment v0 local implementation record

This is the record of the locally implemented v0 migration and export flow as of 2026-10-21. **All unfinished release, data-feed, scale, API/job, and dashboard work is now tracked in [the segment and dashboard scale plan](segment-bitmap-builder-plan.md).** This record is not the activation checklist. No production segment schema, route, worker, or real member feed has been deployed.

## Implemented contract

- Local migrations `000003_segment_v0` and `000004_attribute_segments` create the v0 segment tables, category and patch-count tables, extend cookbook and attribute rules, and seed 27 visible attribute segment rows plus hidden base `seg_100000`. Visible categories still use one of `predefine`, `custom`, or `attribute` as a compatibility tag. `type` is replaced by canonical `kind` in `000004`.
- The approved attribute IDs are `seg_100001`–`seg_100027`. The five Age Band options are 25 and below, 26–35, 36–45, 46–55, and 56 or above. The eight display groups are AEON Credit Card Status, Age Band, Generation, Nationality, Gender, Marital Status, Race, and Membership Tenure. They live in `segments_v0`; the hidden base holds one shared membership snapshot and visible options evaluate `fixed_rule` against it. Income Band is not among the 27.
- The five upstream routes are list, get, create, estimate/confirm, and request-status polling. Four protected operations use `cdphub.cust_intel.view`. By the approved v0 contract, anyone with a request ID can poll its status without a token, requester check, permission, or route-specific rate limit. The served OpenAPI is `services/ecodai/docs/openapi.yaml`.
- Confirm persists a request, recipe, and durable job; the worker records progress, renews and recovers leases, evaluates members, applies current consent, writes one `members.csv`, uploads it locally or to GCS, verifies the object, and publishes `READY`. API and worker switches default off. Caller identity is stored as `TEXT`; request idempotency is scoped to that caller.
- The current evaluator pins each selected segment's `computed_date`, then joins to live `member_cookbook_v0` and combines member-ID sets in Go. This is the implemented v0 behavior, **not** the immutable `build_id` design in the active plan. A later source update can alter a worker's result. The worker persists `recipe_members_v0` only after successful export.

## Local verification and fixtures

| Check | Result |
| --- | --- |
| PostgreSQL migration | An earlier version 5 was applied locally and fresh `000001`–`000005` plus a populated v4-to-v5 upgrade passed on disposable databases. The local-only migration source has since been consolidated into `000003`–`000004`; those earlier results do not verify the revised files. Rebuild a disposable database to check the current sequence. The existing local database will not replay edited versions automatically. `000004` refuses rollback while attribute catalogue rows exist. |
| Attribute fixture | 27 visible entries and eight groups passed the local repository/export integration test. Boundary cases include age 25/26 and four race values. The local attribute fixture supplies 14 synthetic members and a shared snapshot. |
| API and worker | Local tests passed estimate, confirm, idempotency, consent, CSV contents, empty results, expired-lease recovery, retry to terminal failure, route validation, permission cases, and public polling. A live server smoke passed before the final 27-option catalogue revision; the revised live HTTP smoke remains in the active plan. |
| Million-member fixture | A repeatable fixture loaded 1,000,000 cookbook members and 2,087,569 membership rows across three synthetic predefined segments. The five-route local export test passed. Observed estimate time was about 1.0–1.7 seconds, confirm 0.38–1.04 seconds, worker 11.8–13.7 seconds, and peak resident memory about 213 MiB on the development machine. These are not production capacity results. |
| Images | Local service and migration images were built. Rebuilding from the final reviewed revision is in the active release gate. |

Synthetic cookbook members and predefined `seg_000001`/`seg_000002` and `seg_900001`–`seg_900003` rows belong to local fixtures, not the attribute migration or a shared-environment feed. The local database migration left 27 numeric-ID attributes, one numeric-ID hidden base, no legacy attribute IDs, and the existing local requests. Dev/UAT connections were unavailable during this work.

## Historical source and schema decisions

The source was the ContextIQ v0 segment implementation and its nine-table `000056_segment_v0` schema. The v0 API retains the five `/api/v0/segments/...` paths, `seg_...` and `segreq_...` IDs, and request states `INIT`, `PROCESSING`, `READY`, and `FAILED`. The older *Segmentations Phase 0 (Temp)* PDF omits create and uses older paths/states; the implemented API and OpenAPI are authoritative for local v0 behavior.

The original table contract assigned `segment_field_catalog_v0`, `segments_v0`, `member_cookbook_v0`, and `segment_members_v0` to data-team ingestion/maintenance, and requests, jobs, recipes, recipe members, and usage to application operations. `segments_v0` is a shared-write boundary: upstream predefined rows and backend custom rows must coexist. The active plan revisits source ownership, immutable builds, and real-data publication before production activation.

The current nine-table contract accepts synthetic 32-character member IDs locally. Do not infer the production member-ID format from that fixture. The backend stores the authenticated caller subject separately from member IDs. Source and migration provenance for the wider ecodai service is recorded in [migration-status.md](migration-status.md).
