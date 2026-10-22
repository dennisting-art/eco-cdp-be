-- Local-only attribute catalogue and all-member snapshot, pinned to 2026-10-20.
-- Run local.sql and optionally scale_1m.sql first. Never use this as a production feed.
\set ON_ERROR_STOP on
BEGIN;

-- Boundary rows make every age, generation, and tenure band testable even
-- when the optional million-member fixture has not been loaded.
WITH samples(n,birth_date,tenure_days) AS (
    VALUES (1,DATE '2001-10-20',0), (2,DATE '1991-10-20',60),
           (3,DATE '1981-10-20',61), (4,DATE '1971-10-20',90),
           (5,DATE '1970-10-20',91), (6,DATE '1945-06-01',180),
           (7,DATE '2012-01-01',181), (8,DATE '1946-01-01',365),
           (9,DATE '1997-01-01',366), (10,DATE '1981-01-01',730),
           (11,DATE '1965-01-01',1000), (12,DATE '1996-01-01',120),
           (13,DATE '2000-10-20',121), (14,DATE '2002-10-20',122)
)
INSERT INTO member_cookbook_v0
    (member_id,cc_active_flag,birth_date,nationality_code,gender,marital_status,race,membership_application_date,consent_flag,ingested_date)
SELECT md5('ecodai-attribute-boundary-' || n::text),n % 2 = 0,birth_date,
       CASE WHEN n % 3 = 0 THEN 'SG' ELSE 'MY' END,
       CASE WHEN n % 2 = 0 THEN 'Female' ELSE 'Male' END,
       CASE WHEN n % 3 = 0 THEN 'Married' ELSE 'Single' END,
       CASE n % 4 WHEN 1 THEN 'malay' WHEN 2 THEN 'chinese' WHEN 3 THEN 'indians' ELSE 'other' END,
       DATE '2026-10-20' - tenure_days,TRUE,DATE '2026-10-20'
FROM samples
ON CONFLICT(member_id) DO UPDATE SET
    cc_active_flag=excluded.cc_active_flag,birth_date=excluded.birth_date,
    nationality_code=excluded.nationality_code,gender=excluded.gender,
    marital_status=excluded.marital_status,race=excluded.race,
    membership_application_date=excluded.membership_application_date,
    consent_flag=excluded.consent_flag;

-- Catalogue definitions and the hidden base are seeded by migration 000004.

INSERT INTO segment_members_v0(segment_id,member_id,computed_date)
SELECT 'seg_100000',member_id,DATE '2026-10-20' FROM member_cookbook_v0
ON CONFLICT DO NOTHING;

WITH population AS (
    SELECT m.*,
           EXTRACT(YEAR FROM AGE(DATE '2026-10-20',m.birth_date))::int AS age_years,
           EXTRACT(YEAR FROM m.birth_date)::int AS birth_year,
           DATE '2026-10-20' - m.membership_application_date AS tenure_days
    FROM segment_members_v0 sm
    JOIN member_cookbook_v0 m ON m.member_id=sm.member_id
    WHERE sm.segment_id='seg_100000' AND sm.computed_date=DATE '2026-10-20'
), counts AS (
    SELECT count(*) FILTER (WHERE cc_active_flag) AS cc_active,
           count(*) FILTER (WHERE NOT cc_active_flag) AS cc_inactive,
           count(*) FILTER (WHERE age_years BETWEEN 0 AND 25) AS age_under25,
           count(*) FILTER (WHERE age_years BETWEEN 26 AND 35) AS age_25_35,
           count(*) FILTER (WHERE age_years BETWEEN 36 AND 45) AS age_36_45,
           count(*) FILTER (WHERE age_years BETWEEN 46 AND 55) AS age_46_55,
           count(*) FILTER (WHERE age_years >= 56) AS age_56_plus,
           count(*) FILTER (WHERE birth_year BETWEEN 1997 AND 2012 AND birth_date <= DATE '2026-10-20') AS gen_z,
           count(*) FILTER (WHERE birth_year BETWEEN 1981 AND 1996 AND birth_date <= DATE '2026-10-20') AS gen_millennial,
           count(*) FILTER (WHERE birth_year BETWEEN 1965 AND 1980 AND birth_date <= DATE '2026-10-20') AS gen_x,
           count(*) FILTER (WHERE birth_year BETWEEN 1946 AND 1964 AND birth_date <= DATE '2026-10-20') AS gen_boomer,
           count(*) FILTER (WHERE birth_year <= 1945 AND birth_date <= DATE '2026-10-20') AS gen_silent,
           count(*) FILTER (WHERE nationality_code='MY') AS national_my,
           count(*) FILTER (WHERE nationality_code<>'MY') AS national_foreign,
           count(*) FILTER (WHERE gender='Male') AS gender_male,
           count(*) FILTER (WHERE gender='Female') AS gender_female,
           count(*) FILTER (WHERE marital_status='Single') AS marital_single,
           count(*) FILTER (WHERE marital_status='Married') AS marital_married,
           count(*) FILTER (WHERE race='malay') AS race_malay,
           count(*) FILTER (WHERE race='chinese') AS race_chinese,
           count(*) FILTER (WHERE race='indians') AS race_indian,
           count(*) FILTER (WHERE race='other') AS race_others,
           count(*) FILTER (WHERE tenure_days BETWEEN 0 AND 60) AS tenure_0_60,
           count(*) FILTER (WHERE tenure_days BETWEEN 61 AND 90) AS tenure_61_90,
           count(*) FILTER (WHERE tenure_days BETWEEN 91 AND 180) AS tenure_91_180,
           count(*) FILTER (WHERE tenure_days BETWEEN 181 AND 365) AS tenure_181_365,
           count(*) FILTER (WHERE tenure_days >= 366) AS tenure_366_plus
    FROM population
)
UPDATE segments_v0 s SET estimated_member_count=v.member_count,last_updated_at=CURRENT_TIMESTAMP
FROM counts c CROSS JOIN LATERAL (VALUES
    ('seg_100001',c.cc_active),('seg_100002',c.cc_inactive),
    ('seg_100003',c.age_under25),('seg_100004',c.age_25_35),
    ('seg_100005',c.age_36_45),('seg_100006',c.age_46_55),
    ('seg_100007',c.age_56_plus),('seg_100008',c.gen_z),
    ('seg_100009',c.gen_millennial),('seg_100010',c.gen_x),
    ('seg_100011',c.gen_boomer),('seg_100012',c.gen_silent),
    ('seg_100013',c.national_my),('seg_100014',c.national_foreign),
    ('seg_100015',c.gender_male),('seg_100016',c.gender_female),
    ('seg_100017',c.marital_single),('seg_100018',c.marital_married),
    ('seg_100019',c.race_malay),('seg_100020',c.race_chinese),
    ('seg_100021',c.race_indian),('seg_100022',c.race_others),
    ('seg_100023',c.tenure_0_60),('seg_100024',c.tenure_61_90),
    ('seg_100025',c.tenure_91_180),('seg_100026',c.tenure_181_365),
    ('seg_100027',c.tenure_366_plus)
) AS v(id,member_count)
WHERE s.id=v.id;

-- Local-only counts are explicitly labelled; the API never infers a count
-- from the legacy segments_v0.estimated_member_count compatibility column.
INSERT INTO segment_count_patch_v0
    (segment_id,org_id,member_count,population_basis,as_of_date,source_ref,reviewed_by)
SELECT id,'ALL',estimated_member_count,'raw',CURRENT_DATE,'local-attribute-fixture','local-test'
FROM segments_v0 WHERE kind='option'
ON CONFLICT (segment_id,org_id) WHERE status='active'
DO UPDATE SET member_count=excluded.member_count,as_of_date=excluded.as_of_date,
    source_ref=excluded.source_ref,reviewed_by=excluded.reviewed_by;

COMMIT;

SELECT c.display_name AS category_name,count(*) AS entries,sum(p.member_count) AS total_memberships
FROM segments_v0 s
JOIN segment_attribute_category_v0 c ON c.code=s.category_code
JOIN segment_count_patch_v0 p ON p.segment_id=s.id AND p.org_id='ALL' AND p.status='active'
WHERE s.kind='option' GROUP BY c.display_name ORDER BY c.display_name;
