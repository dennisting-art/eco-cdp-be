-- Synthetic local-only member IDs. Never use this file for a production load.
BEGIN;
INSERT INTO segment_field_catalog_v0(field_key,label,data_type,allowed_operators,enum_values,source_mapping) VALUES
 ('cc_active_flag','Active card','BOOLEAN',ARRAY['=','!='],NULL,'{"column":"cc_active_flag"}'),
 ('annual_income','Annual income','NUMBER',ARRAY['=','>=','>','<','<='],NULL,'{"column":"annual_income"}'),
 ('race','Race','STRING',ARRAY['=','in'],ARRAY['malay','chinese','indians','other'],'{"column":"race"}'),
 ('gender','Gender','STRING',ARRAY['=','in'],ARRAY['Male','Female'],'{"column":"gender"}'),
 ('marital_status','Marital status','STRING',ARRAY['=','in'],ARRAY['Single','Married'],'{"column":"marital_status"}'),
 ('nationality_code','Nationality','STRING',ARRAY['=','in'],ARRAY['MY','SG'],'{"column":"nationality_code"}')
ON CONFLICT(field_key) DO UPDATE SET label=excluded.label,data_type=excluded.data_type,allowed_operators=excluded.allowed_operators,enum_values=excluded.enum_values,source_mapping=excluded.source_mapping;

INSERT INTO segments_v0(id,name,tags,kind,definition_status,filter_criteria,estimated_member_count,last_updated_at) VALUES
 ('seg_000001','Active card members',ARRAY['predefine','card'],'predefined','approved','{"filters":[{"key":"cc_active_flag","operation":"=","values":[true,false]},{"key":"race","operation":"in","values":["malay","chinese","indians","other"]},{"key":"gender","operation":"in","values":["Male","Female"]},{"key":"marital_status","operation":"in","values":["Single","Married"]}]}',3,current_timestamp),
 ('seg_000002','Higher income',ARRAY['predefine','income'],'predefined','approved','{"filters":[{"key":"annual_income","operation":">=","values":[50000,100000]}]}',2,current_timestamp)
ON CONFLICT(id) DO UPDATE SET name=excluded.name,tags=excluded.tags,kind=excluded.kind,definition_status=excluded.definition_status,filter_criteria=excluded.filter_criteria,estimated_member_count=excluded.estimated_member_count,last_updated_at=excluded.last_updated_at;

INSERT INTO member_cookbook_v0(member_id,cc_active_flag,annual_income,race,consent_flag,birth_date,nationality_code,gender,marital_status,membership_application_date) VALUES
 ('11111111111141118111111111111111',true,80000,'malay',true,DATE '2002-10-20','MY','Male','Single',DATE '2026-09-20'),
 ('22222222222242228222222222222222',true,120000,'chinese',true,DATE '1990-06-15','MY','Female','Married',DATE '2026-08-06'),
 ('33333333333343338333333333333333',true,30000,'malay',false,DATE '1980-01-01','SG','Male','Married',DATE '2026-06-22'),
 ('44444444444444448444444444444444',false,70000,'chinese',true,DATE '1960-01-01','MY','Female','Single',DATE '2025-09-15')
ON CONFLICT(member_id) DO UPDATE SET cc_active_flag=excluded.cc_active_flag,annual_income=excluded.annual_income,race=excluded.race,consent_flag=excluded.consent_flag,
 birth_date=excluded.birth_date,nationality_code=excluded.nationality_code,gender=excluded.gender,marital_status=excluded.marital_status,membership_application_date=excluded.membership_application_date;

INSERT INTO segment_members_v0(segment_id,member_id,computed_date) VALUES
 ('seg_000001','11111111111141118111111111111111',current_date),
 ('seg_000001','22222222222242228222222222222222',current_date),
 ('seg_000001','33333333333343338333333333333333',current_date),
 ('seg_000002','22222222222242228222222222222222',current_date),
 ('seg_000002','44444444444444448444444444444444',current_date)
ON CONFLICT DO NOTHING;
COMMIT;
