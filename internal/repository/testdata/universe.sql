-- Local-only synthetic data for POST /api/v0/customer-intelligence/universe.
-- Run after migration 000057. Rerunning updates the same sample scope/date rows.
-- This intentionally overwrites matching sample rows; never run against shared data.
-- From the repository root:
-- docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U admin -d cdp_db < scripts/seed_customer_intelligence_universe_local.sql

BEGIN;

CREATE TEMP TABLE universe_seed_opcos (
    scope_code TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    display_order INTEGER NOT NULL,
    matrix_members BIGINT NOT NULL,
    latest_retail BIGINT NOT NULL,
    latest_retail_members BIGINT NOT NULL,
    latest_date DATE NOT NULL
) ON COMMIT DROP;

INSERT INTO universe_seed_opcos VALUES
    ('ABG', 'AEON BIG', 1, 1500000, 5500000, 4600000, '2026-09-27'),
    ('ACO', 'AEON Co.', 2, 1900000, 6200000, 2900000, '2026-09-26'),
    ('ACSM', 'AEON Credit', 3, 800000, 2400000, 1450000, '2026-09-27'),
    ('ABK', 'AEON Bank', 4, 450000, 980000, 510000, '2026-09-25');

INSERT INTO feature_summary (
    feature_code, display_name, description, display_order, feature_type, feature_context
)
VALUES (
    'ENT_CNT', 'Entities per member',
    'Distribution of members by number of AEON entities used.',
    0, 'DISTRIBUTION', 'UNIVERSE'
)
ON CONFLICT (feature_code) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    display_order = EXCLUDED.display_order,
    feature_type = EXCLUDED.feature_type,
    feature_context = EXCLUDED.feature_context;

INSERT INTO feature_bucket_dim (feature_code, bucket_code, display_name, display_order)
VALUES ('ENT_CNT', 1, '1 Entity Only', 1),
       ('ENT_CNT', 2, '2 Entities', 2),
       ('ENT_CNT', 3, '3+ Entities', 3)
ON CONFLICT (feature_code, bucket_code) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    display_order = EXCLUDED.display_order;

INSERT INTO ecosystem_scope (scope_code, scope_type, display_name, display_order, is_available)
VALUES ('AEON', 'GROUP', 'AEON Group', 0, TRUE)
ON CONFLICT (scope_code) DO UPDATE SET
    scope_type = EXCLUDED.scope_type,
    display_name = EXCLUDED.display_name,
    display_order = EXCLUDED.display_order,
    is_available = EXCLUDED.is_available,
    updated_at = now();

INSERT INTO ecosystem_scope_feature (scope_code, feature_code, is_enabled)
VALUES ('AEON', 'MEMBER_MATRIX_ACTIVITY', TRUE)
ON CONFLICT (scope_code, feature_code) DO UPDATE SET
    is_enabled = EXCLUDED.is_enabled,
    updated_at = now();

INSERT INTO ecosystem_scope (scope_code, scope_type, display_name, display_order, is_available)
SELECT scope_code, 'OPCO', display_name, display_order, TRUE
FROM universe_seed_opcos
ON CONFLICT (scope_code) DO UPDATE SET
    scope_type = EXCLUDED.scope_type,
    display_name = EXCLUDED.display_name,
    display_order = EXCLUDED.display_order,
    is_available = EXCLUDED.is_available,
    updated_at = now();

-- Group and card snapshots deliberately have different latest dates.
INSERT INTO customer_daily_summary (
    scope_code, as_of_date, total_member_count, active_member_count,
    inactive_member_count, total_retail_count, retail_member_count,
    retail_non_member_count
) VALUES
    ('AEON', '2026-09-20', 4400000, 2750000, 1650000, 15800000, 8000000, 7800000),
    ('AEON', '2026-09-27', 4600000, 2900000, 1700000, 17000000, 8500000, 8500000)
ON CONFLICT (scope_code, as_of_date) DO UPDATE SET
    total_member_count = EXCLUDED.total_member_count,
    active_member_count = EXCLUDED.active_member_count,
    inactive_member_count = EXCLUDED.inactive_member_count,
    total_retail_count = EXCLUDED.total_retail_count,
    retail_member_count = EXCLUDED.retail_member_count,
    retail_non_member_count = EXCLUDED.retail_non_member_count,
    updated_at = now();

INSERT INTO customer_daily_summary (
    scope_code, as_of_date, total_member_count, active_member_count,
    inactive_member_count, total_retail_count, retail_member_count,
    retail_non_member_count
)
SELECT o.scope_code, COALESCE(v.as_of_date, o.latest_date), NULL, NULL, NULL,
       x.retail, x.retail_members, x.retail - x.retail_members
FROM universe_seed_opcos AS o
CROSS JOIN (VALUES
    ('2026-08-31'::date, 0.94::numeric),
    ('2026-09-24'::date, 1.00::numeric),
    (NULL::date, 1.03::numeric)
) AS v(as_of_date, factor)
CROSS JOIN LATERAL (
    SELECT round(o.matrix_members * v.factor)::bigint AS members,
           round(o.latest_retail * v.factor)::bigint AS retail,
           round(o.latest_retail_members * v.factor)::bigint AS retail_members
) AS x
ON CONFLICT (scope_code, as_of_date) DO UPDATE SET
    total_member_count = NULL,
    active_member_count = NULL,
    inactive_member_count = NULL,
    total_retail_count = EXCLUDED.total_retail_count,
    retail_member_count = EXCLUDED.retail_member_count,
    retail_non_member_count = EXCLUDED.retail_non_member_count,
    updated_at = now();

-- Eight years exceed the three years in the PDF example. The latest 2026
-- comparison is negative so the API's growth calculation is easy to inspect.
INSERT INTO annual_member_acquisition (
    scope_code, as_of_date, acquisition_year, member_count,
    new_member_count, compare_count
) VALUES
    ('AEON', '2026-08-31', 2025, 2200000, 680000, 130000),
    ('AEON', '2026-08-31', 2026, 2650000, 450000, -230000),
    ('AEON', '2026-09-26', 2019, 50000, 50000, NULL),
    ('AEON', '2026-09-26', 2020, 150000, 100000, 50000),
    ('AEON', '2026-09-26', 2021, 350000, 200000, 100000),
    ('AEON', '2026-09-26', 2022, 650000, 300000, 100000),
    ('AEON', '2026-09-26', 2023, 1050000, 400000, 100000),
    ('AEON', '2026-09-26', 2024, 1600000, 550000, 150000),
    ('AEON', '2026-09-26', 2025, 2290000, 690000, 140000),
    ('AEON', '2026-09-26', 2026, 2770000, 480000, -210000)
ON CONFLICT (scope_code, as_of_date, acquisition_year) DO UPDATE SET
    member_count = EXCLUDED.member_count,
    new_member_count = EXCLUDED.new_member_count,
    compare_count = EXCLUDED.compare_count,
    updated_at = now();

INSERT INTO member_daily_distribution (
    scope_code, feature_code, bucket_code, as_of_date, customer_count, ratio
) VALUES
    ('AEON', 'ENT_CNT', 1, '2026-08-31', 2800000, 0.636364),
    ('AEON', 'ENT_CNT', 2, '2026-08-31', 1050000, 0.238636),
    ('AEON', 'ENT_CNT', 3, '2026-08-31', 550000, 0.125000),
    ('AEON', 'ENT_CNT', 1, '2026-09-25', 1760000, 0.382609),
    ('AEON', 'ENT_CNT', 2, '2026-09-25', 1460000, 0.317391),
    ('AEON', 'ENT_CNT', 3, '2026-09-25', 1380000, 0.300000)
ON CONFLICT (scope_code, feature_code, bucket_code, as_of_date) DO UPDATE SET
    customer_count = EXCLUDED.customer_count,
    ratio = EXCLUDED.ratio,
    updated_at = now();

-- Primary totals and complete 4 x 4 matrices on two dates. Diagonal cells equal their row totals;
-- off-diagonal cells use deterministic overlap values between 8% and 14%.
INSERT INTO member_entity_daily (
    scope_code, as_of_date, primary_entity_code, related_entity_code,
    member_count, ratio
)
SELECT 'AEON', v.as_of_date, p.scope_code, NULL,
       round(p.matrix_members * v.factor)::bigint, NULL
FROM universe_seed_opcos AS p
CROSS JOIN (VALUES
    ('2026-08-31'::date, 0.94::numeric),
    ('2026-09-24'::date, 1.00::numeric)
) AS v(as_of_date, factor)
ON CONFLICT (scope_code, as_of_date, primary_entity_code)
WHERE related_entity_code IS NULL
DO UPDATE SET member_count = EXCLUDED.member_count,
              ratio = NULL,
              updated_at = now();

WITH matrix_counts AS (
    SELECT v.as_of_date, p.scope_code AS primary_entity_code,
           r.scope_code AS related_entity_code,
           round(p.matrix_members * v.factor)::bigint AS primary_total,
           CASE WHEN p.scope_code = r.scope_code
                THEN round(p.matrix_members * v.factor)::bigint
                ELSE round(
                    least(p.matrix_members, r.matrix_members) * v.factor
                    * (8 + (p.display_order + r.display_order) % 7) / 100
                )::bigint
           END AS member_count
    FROM universe_seed_opcos AS p
    CROSS JOIN universe_seed_opcos AS r
    CROSS JOIN (VALUES
        ('2026-08-31'::date, 0.94::numeric),
        ('2026-09-24'::date, 1.00::numeric)
    ) AS v(as_of_date, factor)
)
INSERT INTO member_entity_daily (
    scope_code, as_of_date, primary_entity_code, related_entity_code,
    member_count, ratio
)
SELECT 'AEON', as_of_date, primary_entity_code, related_entity_code,
       member_count, round(member_count::numeric / primary_total, 6)
FROM matrix_counts
ON CONFLICT (scope_code, as_of_date, primary_entity_code, related_entity_code)
WHERE related_entity_code IS NOT NULL
DO UPDATE SET member_count = EXCLUDED.member_count,
              ratio = EXCLUDED.ratio,
              updated_at = now();

COMMIT;
