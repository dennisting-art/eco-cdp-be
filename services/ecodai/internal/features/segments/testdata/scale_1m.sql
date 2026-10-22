-- Local scale fixture: exactly 1,000,000 synthetic members and three overlapping
-- predefined snapshots. Run with `make -C services/ecodai seed-segments-scale-local`.
-- It is repeatable and does not touch the small seg_000001/seg_000002 fixture.
\set ON_ERROR_STOP on
\timing on

CREATE TEMP TABLE segment_scale_members AS
WITH digests AS MATERIALIZED (
    SELECT n, md5('ecodai-segment-scale-v1:' || n::text) AS digest
    FROM generate_series(1, 1000000) AS g(n)
)
SELECT n,
       'ec0d0001' || substr(digest, 1, 4) || '4' || substr(digest, 5, 3) || '8' || substr(digest, 8, 15) AS member_id,
       (n % 2 = 0) AS cc_active_flag,
       (30000 + (n * 37) % 170000)::numeric AS annual_income,
       (n % 10 < 8) AS consent_flag,
       n % 20 AS basket_count_last_30_days,
       make_date(1940 + n % 83, n % 12 + 1, 15) AS birth_date,
       CASE WHEN n % 5 = 0 THEN 'SG' ELSE 'MY' END AS nationality_code,
       CASE WHEN n % 2 = 0 THEN 'Female' ELSE 'Male' END AS gender,
       CASE WHEN n % 3 = 0 THEN 'Married' ELSE 'Single' END AS marital_status,
       DATE '2026-10-20' - (n % 900) AS membership_application_date
FROM digests;

DO $$ BEGIN
    IF (SELECT count(*) FROM segment_scale_members) <> 1000000
       OR (SELECT count(DISTINCT member_id) FROM segment_scale_members) <> 1000000
       OR (SELECT min(length(member_id)) FROM segment_scale_members) <> 32
       OR (SELECT max(length(member_id)) FROM segment_scale_members) <> 32 THEN
        RAISE EXCEPTION 'scale member generation failed uniqueness or length checks';
    END IF;
END $$;

INSERT INTO segment_field_catalog_v0(field_key,label,data_type,allowed_operators,enum_values,source_mapping,sort_order) VALUES
 ('cc_active_flag','Active card','BOOLEAN',ARRAY['=','!='],NULL,'{"column":"cc_active_flag"}',1),
 ('local_or_foreigner','Local or foreigner','STRING',ARRAY['=','in'],ARRAY['Local','Foreigner'],'{"column":"local_or_foreigner"}',2),
 ('annual_income','Annual income','NUMBER',ARRAY['=','!=','>','>=','<','<='],NULL,'{"column":"annual_income"}',3),
 ('race','Race','STRING',ARRAY['=','in'],ARRAY['malay','chinese','indians','other'],'{"column":"race"}',4),
 ('avg_basket_value_last_30_days','Average basket value (30d)','NUMBER',ARRAY['=','>','>=','<','<='],NULL,'{"column":"avg_basket_value_last_30_days"}',5),
 ('basket_value_last_30_days','Basket value (30d)','NUMBER',ARRAY['=','>','>=','<','<='],NULL,'{"column":"basket_value_last_30_days"}',6),
 ('basket_count_last_30_days','Basket count (30d)','INTEGER',ARRAY['=','>','>=','<','<='],NULL,'{"column":"basket_count_last_30_days"}',7),
 ('age_distribution','Age distribution','STRING',ARRAY['=','in'],ARRAY['Generation Z','Millennials','Generation X','Baby Boomers'],'{"column":"age_distribution"}',8)
ON CONFLICT(field_key) DO UPDATE SET label=excluded.label,data_type=excluded.data_type,
 allowed_operators=excluded.allowed_operators,enum_values=excluded.enum_values,
 source_mapping=excluded.source_mapping,sort_order=excluded.sort_order,updated_at=current_timestamp;

INSERT INTO segments_v0(id,name,tags,kind,definition_status,description,filter_criteria,estimated_member_count,last_updated_at) VALUES
 ('seg_900001','Scale: all members',ARRAY['predefine','scale','all'],'predefined','approved','One million synthetic members.',
  '{"filters":[{"key":"cc_active_flag","operation":"=","values":[true,false]},{"key":"annual_income","operation":">=","values":[50000,100000,150000]},{"key":"race","operation":"in","values":["malay","chinese","indians","other"]},{"key":"basket_count_last_30_days","operation":">=","values":[5,10,15]}]}',1000000,current_timestamp),
 ('seg_900002','Scale: active card',ARRAY['predefine','scale','card'],'predefined','approved','Overlapping active-card members.',
  '{"filters":[{"key":"annual_income","operation":">=","values":[50000,100000,150000]},{"key":"local_or_foreigner","operation":"in","values":["Local","Foreigner"]},{"key":"age_distribution","operation":"in","values":["Generation Z","Millennials","Generation X","Baby Boomers"]}]}',500000,current_timestamp),
 ('seg_900003','Scale: higher income',ARRAY['predefine','scale','income'],'predefined','approved','Overlapping members with income at least 100,000.',
  '{"filters":[{"key":"cc_active_flag","operation":"=","values":[true,false]},{"key":"race","operation":"in","values":["malay","chinese","indians","other"]},{"key":"avg_basket_value_last_30_days","operation":">=","values":[100,200,300]}]}',0,current_timestamp)
ON CONFLICT(id) DO UPDATE SET name=excluded.name,tags=excluded.tags,kind=excluded.kind,definition_status=excluded.definition_status,
 description=excluded.description,filter_criteria=excluded.filter_criteria,
 estimated_member_count=excluded.estimated_member_count,last_updated_at=excluded.last_updated_at;

\echo Loading 1,000,000 cookbook rows
INSERT INTO member_cookbook_v0(member_id,cc_active_flag,local_or_foreigner,annual_income,race,
 avg_basket_value_last_30_days,basket_value_last_30_days,basket_count_last_30_days,
 age_distribution,consent_flag,consent_refreshed_at,ingested_date,birth_date,
 nationality_code,gender,marital_status,membership_application_date)
SELECT member_id,cc_active_flag,
       CASE WHEN n % 5 = 0 THEN 'Foreigner' ELSE 'Local' END,
       annual_income,
       CASE n % 4 WHEN 0 THEN 'malay' WHEN 1 THEN 'chinese' WHEN 2 THEN 'indians' ELSE 'other' END,
       (20 + (n * 17) % 500)::numeric,
       ((20 + (n * 17) % 500) * basket_count_last_30_days)::numeric,
       basket_count_last_30_days,
       CASE WHEN EXTRACT(YEAR FROM birth_date) BETWEEN 1997 AND 2012 THEN 'Generation Z'
            WHEN EXTRACT(YEAR FROM birth_date) BETWEEN 1981 AND 1996 THEN 'Millennials'
            WHEN EXTRACT(YEAR FROM birth_date) BETWEEN 1965 AND 1980 THEN 'Generation X'
            WHEN EXTRACT(YEAR FROM birth_date) BETWEEN 1946 AND 1964 THEN 'Baby Boomers'
            WHEN EXTRACT(YEAR FROM birth_date) <= 1945 THEN 'Silent Generation' END,
       consent_flag,current_timestamp,DATE '2026-10-20',birth_date,
       nationality_code,gender,marital_status,membership_application_date
FROM segment_scale_members
ON CONFLICT(member_id) DO UPDATE SET birth_date=excluded.birth_date,
 nationality_code=excluded.nationality_code,gender=excluded.gender,
 marital_status=excluded.marital_status,membership_application_date=excluded.membership_application_date,
 age_distribution=excluded.age_distribution;

\echo Loading all-member snapshot
INSERT INTO segment_members_v0(segment_id,member_id,computed_date)
SELECT 'seg_900001',member_id,DATE '2026-10-20' FROM segment_scale_members
ON CONFLICT DO NOTHING;

\echo Loading active-card snapshot
INSERT INTO segment_members_v0(segment_id,member_id,computed_date)
SELECT 'seg_900002',member_id,DATE '2026-10-20' FROM segment_scale_members WHERE cc_active_flag
ON CONFLICT DO NOTHING;

\echo Loading higher-income snapshot
INSERT INTO segment_members_v0(segment_id,member_id,computed_date)
SELECT 'seg_900003',member_id,DATE '2026-10-20' FROM segment_scale_members WHERE annual_income >= 100000
ON CONFLICT DO NOTHING;

UPDATE segments_v0 s SET estimated_member_count=(
 SELECT count(*) FROM segment_members_v0 sm WHERE sm.segment_id=s.id AND sm.computed_date=DATE '2026-10-20'
),last_updated_at=current_timestamp WHERE s.id IN ('seg_900001','seg_900002','seg_900003');

ANALYZE member_cookbook_v0;
ANALYZE segment_members_v0;

SELECT s.id,s.name,s.estimated_member_count AS snapshot_members,
       count(*) FILTER (WHERE m.consent_flag) AS consented_members
FROM segments_v0 s
JOIN segment_members_v0 sm ON sm.segment_id=s.id AND sm.computed_date=DATE '2026-10-20'
JOIN member_cookbook_v0 m ON m.member_id=sm.member_id
WHERE s.id IN ('seg_900001','seg_900002','seg_900003')
GROUP BY s.id,s.name,s.estimated_member_count ORDER BY s.id;
