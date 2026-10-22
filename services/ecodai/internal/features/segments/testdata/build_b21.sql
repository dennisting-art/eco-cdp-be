-- Disposable integration fixture only. M001-M003 are synthetic IDs.
INSERT INTO segment_build_v0
    (build_id,source_date,status,required_outputs,source_member_count)
VALUES ('B21',DATE '2026-10-21','LOADING',ARRAY['source','sql_counts'],3);

INSERT INTO member_facts_v0
    (build_id,member_id,birth_date,gender,race,annual_income,cc_active_flag,build_consent)
VALUES
    ('B21','M001',DATE '1992-06-10','Female','malay',80000,FALSE,TRUE),
    ('B21','M002',DATE '1996-03-02','Male','chinese',120000,FALSE,TRUE),
    ('B21','M003',DATE '1985-07-18','Female','other',30000,TRUE,FALSE);

INSERT INTO member_org_v0 (build_id,member_id,org_id) VALUES
    ('B21','M001','org_1'),('B21','M002','org_1'),('B21','M003','org_1');
INSERT INTO member_retail_v0 (build_id,member_id,retail_id,last_txn_date) VALUES
    ('B21','M001','big',DATE '2026-10-09'),
    ('B21','M001','co',DATE '2026-10-09'),
    ('B21','M002','big',DATE '2026-10-12'),
    ('B21','M003','bank',DATE '2025-12-01');
INSERT INTO member_current_consent_v0 (member_id,consent,refreshed_at) VALUES
    ('M001',TRUE,now()),('M002',TRUE,now()),('M003',FALSE,now());
INSERT INTO segment_source_load_v0
    (build_id,source_name,source_version,row_count,completed_at)
VALUES
    ('B21','member_facts_v0','fixture-1',3,now()),
    ('B21','member_org_v0','fixture-1',3,now()),
    ('B21','member_retail_v0','fixture-1',4,now()),
    ('B21','member_product_v0','fixture-1',0,now()),
    ('B21','member_predefined_v0','fixture-1',0,now());
