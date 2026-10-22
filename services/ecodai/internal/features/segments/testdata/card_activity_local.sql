-- Disposable local playground. Run once on a freshly migrated database.
-- This keeps the three-member B21 regression fixture unchanged.
-- seg_000001 is a local fixture ID, not an approved production segment ID.
-- After publication: 120 members, 90 predefined raw, 80 consented;
-- income >= 100000: 40, Malay: 40, both filters: 10 consented.
BEGIN;

INSERT INTO segment_field_catalog_v0
    (field_key,label,data_type,allowed_operators,source_mapping)
VALUES
    ('annual_income','Annual income','NUMBER',ARRAY['>='],'{"column":"annual_income"}'),
    ('race','Race','STRING',ARRAY['in'],'{"column":"race"}')
ON CONFLICT (field_key) DO UPDATE SET
    label=excluded.label,data_type=excluded.data_type,
    allowed_operators=excluded.allowed_operators,source_mapping=excluded.source_mapping;

INSERT INTO segments_v0
    (id,name,tags,kind,definition_status,description,filter_criteria)
VALUES
    ('seg_000001','Active card members with recent retail activity',
     ARRAY['predefine','card','local_fixture'],'predefined','approved',
     'Active card members with at least one retail transaction in the 30 days ending on the build source date.',
     '{"filters":[{"key":"annual_income","operation":">=","values":[50000,100000]},{"key":"race","operation":"in","values":["malay","chinese","indians","other"]}]}');

INSERT INTO segment_build_v0
    (build_id,source_date,status,required_outputs,source_member_count)
VALUES
    ('CARD_PLAY_01',DATE '2026-10-21','LOADING',
     ARRAY['source','sql_counts','bitmaps','dashboards'],120);

-- Each of these twelve profiles is repeated ten times (120 members total).
-- P001-P009 qualify for the predefined rule. P009 lacks consent;
-- P010 has an old transaction, P011 has no active card, P012 has no retail row.
INSERT INTO member_facts_v0
    (build_id,member_id,birth_date,gender,race,annual_income,
     basket_value_last_30_days,cc_active_flag,build_consent)
VALUES
    ('CARD_PLAY_01','P001',DATE '1992-06-10','Female','malay',40000,120,TRUE,TRUE),
    ('CARD_PLAY_01','P002',DATE '1988-02-01','Male','chinese',60000,240,TRUE,TRUE),
    ('CARD_PLAY_01','P003',DATE '1996-03-02','Female','malay',120000,350,TRUE,TRUE),
    ('CARD_PLAY_01','P004',DATE '1985-07-18','Female','indians',150000,500,TRUE,TRUE),
    ('CARD_PLAY_01','P005',DATE '2000-01-12','Male','malay',90000,210,TRUE,TRUE),
    ('CARD_PLAY_01','P006',DATE '1977-09-25','Male','chinese',110000,300,TRUE,TRUE),
    ('CARD_PLAY_01','P007',DATE '1994-04-07','Female','malay',75000,180,TRUE,TRUE),
    ('CARD_PLAY_01','P008',DATE '1969-11-03','Male','other',200000,450,TRUE,TRUE),
    ('CARD_PLAY_01','P009',DATE '1991-05-14','Female','chinese',130000,350,TRUE,FALSE),
    ('CARD_PLAY_01','P010',DATE '1983-08-20','Male','malay',80000,90,TRUE,TRUE),
    ('CARD_PLAY_01','P011',DATE '1998-12-05','Female','chinese',90000,250,FALSE,TRUE),
    ('CARD_PLAY_01','P012',DATE '1972-01-30','Male','indians',70000,0,TRUE,TRUE);

INSERT INTO member_facts_v0
    (build_id,member_id,birth_date,gender,race,annual_income,
     basket_value_last_30_days,cc_active_flag,build_consent)
SELECT m.build_id,m.member_id || '_' || lpad(n::text,2,'0'),m.birth_date,
       m.gender,m.race,m.annual_income,m.basket_value_last_30_days,
       m.cc_active_flag,m.build_consent
FROM member_facts_v0 m CROSS JOIN generate_series(2,10) AS copies(n)
WHERE m.build_id='CARD_PLAY_01';

INSERT INTO member_org_v0 (build_id,member_id,org_id)
SELECT build_id,member_id,'org_1' FROM member_facts_v0
WHERE build_id='CARD_PLAY_01';

-- Days ago is relative to the single source_date above. P004 visits two
-- retails but must count as one person in a union scope.
INSERT INTO member_retail_v0 (build_id,member_id,retail_id,last_txn_date)
SELECT b.build_id,v.member_id,v.retail_id,b.source_date-v.days_ago
FROM segment_build_v0 b
CROSS JOIN (VALUES
    ('P001','big',5),('P002','big',2),('P003','co',4),
    ('P004','big',1),('P004','co',3),('P005','co',20),
    ('P006','big',29),('P007','big',0),('P008','co',8),
    ('P009','big',6),('P010','big',45),('P011','big',1)
) AS v(member_id,retail_id,days_ago)
WHERE b.build_id='CARD_PLAY_01';

INSERT INTO member_retail_v0 (build_id,member_id,retail_id,last_txn_date)
SELECT r.build_id,r.member_id || '_' || lpad(n::text,2,'0'),
       r.retail_id,r.last_txn_date
FROM member_retail_v0 r CROSS JOIN generate_series(2,10) AS copies(n)
WHERE r.build_id='CARD_PLAY_01';

-- The local seed computes the predefined membership exactly once, before
-- publication. A production feed/job must own this computation.
INSERT INTO member_predefined_v0 (build_id,member_id,segment_id)
SELECT DISTINCT m.build_id,m.member_id,'seg_000001'
FROM member_facts_v0 m
JOIN member_retail_v0 r ON r.build_id=m.build_id AND r.member_id=m.member_id
JOIN segment_build_v0 b ON b.build_id=m.build_id
WHERE m.build_id='CARD_PLAY_01'
  AND m.cc_active_flag IS TRUE
  AND r.last_txn_date BETWEEN b.source_date-29 AND b.source_date;

INSERT INTO member_current_consent_v0 (member_id,consent,refreshed_at)
SELECT member_id,build_consent,now() FROM member_facts_v0
WHERE build_id='CARD_PLAY_01';

-- The load log is separate from backend publication. Counts come from the
-- rows just inserted so a future fixture edit cannot leave stale log totals.
INSERT INTO segment_source_load_v0
    (build_id,source_name,source_version,row_count,completed_at)
SELECT 'CARD_PLAY_01','member_facts_v0','card-play-1',COUNT(*),now()
FROM member_facts_v0 WHERE build_id='CARD_PLAY_01'
UNION ALL
SELECT 'CARD_PLAY_01','member_org_v0','card-play-1',COUNT(*),now()
FROM member_org_v0 WHERE build_id='CARD_PLAY_01'
UNION ALL
SELECT 'CARD_PLAY_01','member_retail_v0','card-play-1',COUNT(*),now()
FROM member_retail_v0 WHERE build_id='CARD_PLAY_01'
UNION ALL
SELECT 'CARD_PLAY_01','member_product_v0','card-play-1',COUNT(*),now()
FROM member_product_v0 WHERE build_id='CARD_PLAY_01'
UNION ALL
SELECT 'CARD_PLAY_01','member_predefined_v0','card-play-1',COUNT(*),now()
FROM member_predefined_v0 WHERE build_id='CARD_PLAY_01';

COMMIT;
