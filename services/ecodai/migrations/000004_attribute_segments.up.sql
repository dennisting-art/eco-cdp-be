BEGIN;

ALTER TABLE member_cookbook_v0
    ADD COLUMN birth_date DATE,
    ADD COLUMN nationality_code TEXT,
    ADD COLUMN gender TEXT,
    ADD COLUMN marital_status TEXT,
    ADD COLUMN membership_application_date DATE;

ALTER TABLE segments_v0
    ADD COLUMN category_code TEXT REFERENCES segment_attribute_category_v0(code),
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'custom' CHECK (kind IN ('base', 'option', 'predefined', 'custom')),
    ADD COLUMN definition_version INT NOT NULL DEFAULT 1 CHECK (definition_version > 0),
    ADD COLUMN definition_owner TEXT NOT NULL DEFAULT 'backend',
    ADD COLUMN computation_owner TEXT NOT NULL DEFAULT 'backend',
    ADD COLUMN definition_status TEXT NOT NULL DEFAULT 'draft' CHECK (definition_status IN ('draft', 'approved', 'retired')),
    ADD COLUMN is_visible BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN base_segment_id VARCHAR(32) REFERENCES segments_v0(id),
    ADD COLUMN fixed_rule JSONB;

INSERT INTO segment_attribute_category_v0 (code,display_name,sort_order) VALUES
    ('AEON_CREDIT_CARD_STATUS','AEON Credit Card Status',1),
    ('AGE_BAND','Age Band',2),
    ('GENERATION','Generation',3),
    ('NATIONALITY','Nationality',4),
    ('GENDER','Gender',5),
    ('MARITAL_STATUS','Marital Status',6),
    ('RACE','Race',7),
    ('MEMBERSHIP_TENURE','Membership Tenure',8);

-- The old type only classified rows for display. Preserve that classification
-- as a reserved tag before removing the redundant field.
UPDATE segments_v0
SET kind = CASE WHEN type = 'Custom' THEN 'custom' ELSE 'predefined' END,
    definition_status = CASE WHEN type = 'Custom' THEN 'draft' ELSE 'approved' END,
    tags = array_append(
    array_remove(array_remove(array_remove(tags,'predefine'),'custom'),'attribute'),
    CASE WHEN type = 'Custom' THEN 'custom' ELSE 'predefine' END
);

ALTER TABLE segments_v0 DROP COLUMN type;
ALTER TABLE segments_v0 ADD CONSTRAINT segments_v0_category_tag_check CHECK (
    NOT is_visible OR
    (('predefine' = ANY(tags))::int + ('custom' = ANY(tags))::int + ('attribute' = ANY(tags))::int) = 1
);
ALTER TABLE segments_v0 ADD CONSTRAINT segments_v0_attribute_rule_check CHECK (
    NOT is_visible OR NOT ('attribute' = ANY(tags)) OR
    (base_segment_id IS NOT NULL AND category_code IS NOT NULL)
);
ALTER TABLE segments_v0 ADD CONSTRAINT segments_v0_option_category_check CHECK (
    kind <> 'option' OR category_code IS NOT NULL
);


-- Stable catalogue IDs; synthetic members and snapshots stay in testdata.
CREATE TEMP TABLE _attribute_seed (
    old_id TEXT PRIMARY KEY, new_id TEXT UNIQUE NOT NULL, name TEXT NOT NULL,
    tags TEXT[] NOT NULL, category_name TEXT NOT NULL, description TEXT,
    base_segment_id TEXT NOT NULL, fixed_rule JSONB NOT NULL,
    filter_criteria JSONB NOT NULL
) ON COMMIT DROP;

INSERT INTO _attribute_seed
    (old_id,new_id,name,tags,category_name,description,base_segment_id,fixed_rule,filter_criteria)
VALUES
('seg_attr_cc_active','seg_100001','Active AEON Credit Card',ARRAY['attribute'],'AEON Credit Card Status','Member currently holds an active AEON credit card.','seg_100000','{"key":"cc_active_flag","equal":"true"}','{}'),
('seg_attr_cc_inactive','seg_100002','Non-Active AEON Credit Card',ARRAY['attribute'],'AEON Credit Card Status','Member has no active AEON credit card, including never-held and former cardholders.','seg_100000','{"key":"cc_active_flag","equal":"false"}','{}'),
('seg_attr_age_under25','seg_100003','25 years old and below',ARRAY['attribute'],'Age Band','Aged 25 or below.','seg_100000','{"key":"age_years","min":0,"max":25}','{}'),
('seg_attr_age_25_35','seg_100004','26 - 35 years old',ARRAY['attribute'],'Age Band','Aged 26 to 35.','seg_100000','{"key":"age_years","min":26,"max":35}','{}'),
('seg_attr_age_36_45','seg_100005','36 - 45 years old',ARRAY['attribute'],'Age Band','Aged 36 to 45.','seg_100000','{"key":"age_years","min":36,"max":45}','{}'),
('seg_attr_age_46_55','seg_100006','46 - 55 years old',ARRAY['attribute'],'Age Band','Aged 46 to 55.','seg_100000','{"key":"age_years","min":46,"max":55}','{}'),
('seg_attr_age_56_plus','seg_100007','56 years old and above',ARRAY['attribute'],'Age Band','Aged 56 or above.','seg_100000','{"key":"age_years","min":56}','{}'),
('seg_attr_gen_z','seg_100008','Generation Z (Born 1997–2012)',ARRAY['attribute'],'Generation','Born between 1997 and 2012.','seg_100000','{"key":"birth_year","min":1997,"max":2012}','{}'),
('seg_attr_gen_millennial','seg_100009','Millennials (Born 1981–1996)',ARRAY['attribute'],'Generation','Born between 1981 and 1996.','seg_100000','{"key":"birth_year","min":1981,"max":1996}','{}'),
('seg_attr_gen_x','seg_100010','Generation X (Born 1965–1980)',ARRAY['attribute'],'Generation','Born between 1965 and 1980.','seg_100000','{"key":"birth_year","min":1965,"max":1980}','{}'),
('seg_attr_gen_boomer','seg_100011','Baby Boomers (Born 1946–1964)',ARRAY['attribute'],'Generation','Born between 1946 and 1964.','seg_100000','{"key":"birth_year","min":1946,"max":1964}','{}'),
('seg_attr_gen_silent','seg_100012','Silent Generation (Born 1945 or earlier)',ARRAY['attribute'],'Generation','Born in 1945 or earlier.','seg_100000','{"key":"birth_year","max":1945}','{}'),
('seg_attr_national_my','seg_100013','Malaysian',ARRAY['attribute'],'Nationality','Member is a Malaysian.','seg_100000','{"key":"nationality_code","equal":"MY"}','{}'),
('seg_attr_national_foreign','seg_100014','Foreigner',ARRAY['attribute'],'Nationality','Member is not Malaysian.','seg_100000','{"key":"nationality_code","not_equal":"MY"}','{}'),
('seg_attr_gender_male','seg_100015','Male',ARRAY['attribute'],'Gender','Member''s registered gender is male.','seg_100000','{"key":"gender","equal":"Male"}','{}'),
('seg_attr_gender_female','seg_100016','Female',ARRAY['attribute'],'Gender','Member''s registered gender is female.','seg_100000','{"key":"gender","equal":"Female"}','{}'),
('seg_attr_marital_single','seg_100017','Single',ARRAY['attribute'],'Marital Status','Member''s registered marital status is single.','seg_100000','{"key":"marital_status","equal":"Single"}','{}'),
('seg_attr_marital_married','seg_100018','Married',ARRAY['attribute'],'Marital Status','Member''s registered marital status is married.','seg_100000','{"key":"marital_status","equal":"Married"}','{}'),
('seg_attr_race_malay','seg_100019','Malay',ARRAY['attribute'],'Race','Member''s registered race is Malay.','seg_100000','{"key":"race","equal":"malay"}','{}'),
('seg_attr_race_chinese','seg_100020','Chinese',ARRAY['attribute'],'Race','Member''s registered race is Chinese.','seg_100000','{"key":"race","equal":"chinese"}','{}'),
('seg_attr_race_indian','seg_100021','Indian',ARRAY['attribute'],'Race','Member''s registered race is Indian.','seg_100000','{"key":"race","equal":"indians"}','{}'),
('seg_attr_race_others','seg_100022','Others',ARRAY['attribute'],'Race','Member''s registered race is Other.','seg_100000','{"key":"race","equal":"other"}','{}'),
('seg_attr_tenure_0_60','seg_100023','Joined ≤ 60 days',ARRAY['attribute'],'Membership Tenure','0 to 60 days since membership application.','seg_100000','{"key":"tenure_days","min":0,"max":60}','{}'),
('seg_attr_tenure_61_90','seg_100024','Joined 61 – 90 days',ARRAY['attribute'],'Membership Tenure','61 to 90 days since membership application.','seg_100000','{"key":"tenure_days","min":61,"max":90}','{}'),
('seg_attr_tenure_91_180','seg_100025','Joined 91 – 180 days',ARRAY['attribute'],'Membership Tenure','91 to 180 days since membership application.','seg_100000','{"key":"tenure_days","min":91,"max":180}','{}'),
('seg_attr_tenure_181_365','seg_100026','Joined 181 – 365 days',ARRAY['attribute'],'Membership Tenure','181 to 365 days since membership application.','seg_100000','{"key":"tenure_days","min":181,"max":365}','{}'),
('seg_attr_tenure_366_plus','seg_100027','Member > 1 year',ARRAY['attribute'],'Membership Tenure','More than 365 days since membership application.','seg_100000','{"key":"tenure_days","min":366}','{}');

CREATE TEMP TABLE _segment_id_map (old_id TEXT PRIMARY KEY, new_id TEXT UNIQUE NOT NULL) ON COMMIT DROP;
INSERT INTO _segment_id_map VALUES ('seg_all_members','seg_100000');
INSERT INTO _segment_id_map SELECT old_id,new_id FROM _attribute_seed;

DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM segments_v0 s JOIN _segment_id_map m ON s.id=m.new_id) THEN
        RAISE EXCEPTION 'attribute catalogue target ID already exists';
    END IF;
    IF EXISTS (SELECT 1 FROM segments_v0 s WHERE s.id='seg_all_members' AND s.is_visible) THEN
        RAISE EXCEPTION 'legacy all-members base is visible';
    END IF;
    IF EXISTS (
        SELECT 1 FROM segments_v0 s JOIN _attribute_seed c ON s.id=c.old_id
        WHERE s.name<>c.name OR s.tags<>c.tags
           OR s.fixed_rule IS DISTINCT FROM c.fixed_rule
           OR s.base_segment_id IS DISTINCT FROM 'seg_all_members'
    ) THEN
        RAISE EXCEPTION 'legacy attribute definition differs from approved catalogue';
    END IF;
END $$;

INSERT INTO segments_v0 (id,name,tags,kind,definition_status,description,is_visible,filter_criteria,
    estimated_member_count,last_updated_at,created_by,created_at,updated_at)
SELECT 'seg_100000',COALESCE(old.name,'All members base'),ARRAY[]::TEXT[],'base','approved',
    COALESCE(old.description,'Internal attribute population.'),FALSE,
    COALESCE(old.filter_criteria,'{}'::JSONB),COALESCE(old.estimated_member_count,0),
    old.last_updated_at,old.created_by,COALESCE(old.created_at,now()),COALESCE(old.updated_at,now())
FROM (SELECT 1) anchor LEFT JOIN segments_v0 old ON old.id='seg_all_members';

INSERT INTO segments_v0 (id,name,tags,category_code,kind,definition_status,
    description,base_segment_id,fixed_rule,filter_criteria,
    estimated_member_count,last_updated_at,created_by,created_at,updated_at)
SELECT c.new_id,c.name,c.tags,cat.code,'option','approved',
    c.description,c.base_segment_id,c.fixed_rule,c.filter_criteria,
    COALESCE(old.estimated_member_count,0),old.last_updated_at,old.created_by,
    COALESCE(old.created_at,now()),COALESCE(old.updated_at,now())
FROM _attribute_seed c
JOIN segment_attribute_category_v0 cat ON cat.display_name=c.category_name
LEFT JOIN segments_v0 old ON old.id=c.old_id;

UPDATE segment_members_v0 sm SET segment_id=m.new_id FROM _segment_id_map m WHERE sm.segment_id=m.old_id;
UPDATE segments_v0 s SET base_segment_id=m.new_id FROM _segment_id_map m WHERE s.base_segment_id=m.old_id;
UPDATE recipes_v0 r SET promoted_segment_id=m.new_id FROM _segment_id_map m WHERE r.promoted_segment_id=m.old_id;
UPDATE segment_usage_v0 u SET segment_id=m.new_id FROM _segment_id_map m WHERE u.segment_id=m.old_id;

-- Requests and recipes also retain selected IDs inside JSON.
UPDATE segment_requests_v0 r SET
    segments_payload=(SELECT COALESCE(jsonb_agg(
        CASE WHEN m.new_id IS NULL THEN e.item ELSE jsonb_set(e.item,'{id}',to_jsonb(m.new_id)) END
        ORDER BY e.ordinality),'[]'::JSONB)
        FROM jsonb_array_elements(r.segments_payload) WITH ORDINALITY e(item,ordinality)
        LEFT JOIN _segment_id_map m ON m.old_id=e.item->>'id'),
    snapshot_dates=(SELECT COALESCE(jsonb_object_agg(COALESCE(m.new_id,e.key),e.value),'{}'::JSONB)
        FROM jsonb_each(r.snapshot_dates) e LEFT JOIN _segment_id_map m ON m.old_id=e.key)
WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(r.segments_payload) e JOIN _segment_id_map m ON m.old_id=e->>'id')
   OR EXISTS (SELECT 1 FROM jsonb_object_keys(r.snapshot_dates) e(key) JOIN _segment_id_map m ON m.old_id=e.key);

UPDATE recipes_v0 r SET segments_payload=(SELECT COALESCE(jsonb_agg(
    CASE WHEN m.new_id IS NULL THEN e.item ELSE jsonb_set(e.item,'{id}',to_jsonb(m.new_id)) END
    ORDER BY e.ordinality),'[]'::JSONB)
    FROM jsonb_array_elements(r.segments_payload) WITH ORDINALITY e(item,ordinality)
    LEFT JOIN _segment_id_map m ON m.old_id=e.item->>'id')
WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(r.segments_payload) e JOIN _segment_id_map m ON m.old_id=e->>'id');

DELETE FROM segments_v0 s USING _segment_id_map m WHERE s.id=m.old_id;
SELECT setval('segment_v0_element_sequence',GREATEST(
    (SELECT last_value FROM segment_v0_element_sequence),100027),TRUE);

COMMIT;
