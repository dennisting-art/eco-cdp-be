BEGIN;

-- Removing the rule columns would leave attribute rows selectable but empty.
-- Retire their requests and catalogue rows explicitly before rolling back.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM segments_v0 WHERE 'attribute' = ANY(tags) OR NOT is_visible OR base_segment_id IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot roll back attribute schema while attribute catalogue rows exist';
    END IF;
END $$;

ALTER TABLE segments_v0
    DROP CONSTRAINT segments_v0_attribute_rule_check,
    DROP CONSTRAINT segments_v0_category_tag_check,
    DROP CONSTRAINT segments_v0_option_category_check;

ALTER TABLE segments_v0 ADD COLUMN type TEXT;
UPDATE segments_v0 SET type = CASE WHEN 'custom' = ANY(tags) THEN 'Custom' ELSE 'Predefined' END;
ALTER TABLE segments_v0 ALTER COLUMN type SET NOT NULL;
ALTER TABLE segments_v0 ADD CONSTRAINT segments_v0_type_check CHECK (type IN ('Predefined', 'Custom'));

ALTER TABLE segments_v0
    DROP COLUMN fixed_rule,
    DROP COLUMN base_segment_id,
    DROP COLUMN is_visible,
    DROP COLUMN definition_status,
    DROP COLUMN computation_owner,
    DROP COLUMN definition_owner,
    DROP COLUMN definition_version,
    DROP COLUMN kind,
    DROP COLUMN category_code;

DELETE FROM segment_attribute_category_v0 WHERE code IN (
    'AEON_CREDIT_CARD_STATUS','AGE_BAND','GENERATION','NATIONALITY',
    'GENDER','MARITAL_STATUS','RACE','MEMBERSHIP_TENURE'
);

ALTER TABLE member_cookbook_v0
    DROP COLUMN membership_application_date,
    DROP COLUMN marital_status,
    DROP COLUMN gender,
    DROP COLUMN nationality_code,
    DROP COLUMN birth_date;

COMMIT;
