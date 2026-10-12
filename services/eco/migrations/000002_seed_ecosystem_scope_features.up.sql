BEGIN;

INSERT INTO ecosystem_scope (scope_code, scope_type, display_name, display_order, is_available)
VALUES
    ('AEON', 'GROUP', 'AEON GROUP', 1, TRUE),
    ('ABG', 'OPCO', 'AEON BIG', 1, TRUE),
    ('AMY', 'OPCO', 'AEON CO', 2, TRUE),
    ('ACSM', 'OPCO', 'AEON CREDIT', 3, FALSE),
    ('ADB', 'OPCO', 'AEON BANK', 4, FALSE)
ON CONFLICT (scope_code) DO UPDATE SET
    scope_type = EXCLUDED.scope_type,
    display_name = EXCLUDED.display_name,
    display_order = EXCLUDED.display_order,
    is_available = EXCLUDED.is_available,
    updated_at = now();

INSERT INTO feature_summary (
    feature_code, display_name, description, display_order, feature_type, feature_context
)
VALUES
    ('SEX', 'Gender Distribution', '', 1, 'DISTRIBUTION', 'CARDS'),
    ('AGE', 'Age Distribution', '', 1, 'DISTRIBUTION', 'CARDS'),
    ('TIER', 'Member Tier Distribution', '', 1, 'DISTRIBUTION', 'CARDS'),
    ('TENURE', 'Member Tenure Distribution', '', 1, 'DISTRIBUTION', 'CARDS'),
    ('MVC', 'Member Value Centre', '', 1, 'DISTRIBUTION', 'CARDS'),
    ('PENETRATION_OPCO', 'Member Penetration', '', 1, 'COMBO_TREND', 'CARDS'),
    ('ACTIVE_OPCO', 'Active vs Inactive', '', 1, 'TREND', 'CARDS'),
    ('ENT_CNT', 'Member Entity Count Distribution', '', 1, 'DISTRIBUTION', 'UNIVERSE'),
    ('MEMBER_MATRIX_ACTIVITY', 'Member Matrix Activities',
     'Group member overlap by entity', 1, 'MATRIX', 'UNIVERSE')
ON CONFLICT (feature_code) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    display_order = EXCLUDED.display_order,
    feature_type = EXCLUDED.feature_type,
    feature_context = EXCLUDED.feature_context;

INSERT INTO feature_bucket_dim (
    feature_code, bucket_code, display_name, display_order, is_rollup, rollup_group
)
VALUES
    ('SEX', 1, 'Male', 1, FALSE, ''),
    ('SEX', 2, 'Female', 2, FALSE, ''),
    ('SEX', 99, 'Not Specified', 99, FALSE, ''),
    ('AGE', 1, 'Generation Z (14-29)', 1, FALSE, ''),
    ('AGE', 2, 'Millennials Gen Y (30-45)', 2, FALSE, ''),
    ('AGE', 3, 'Generation X (46-61)', 3, FALSE, ''),
    ('AGE', 4, 'Baby Boomers (62-80)', 4, FALSE, ''),
    ('AGE', 5, 'The Silent Generation (81-98)', 5, FALSE, ''),
    ('AGE', 99, 'Not Specified', 99, FALSE, ''),
    ('TIER', 1, '1-Star', 1, FALSE, ''),
    ('TIER', 2, '2-Star', 2, FALSE, ''),
    ('TIER', 3, '3-Star', 3, FALSE, ''),
    ('TIER', 4, '4-Star', 4, FALSE, ''),
    ('TENURE', 1, 'New (0-12mo)', 1, FALSE, ''),
    ('TENURE', 2, 'Established (1-3yr)', 2, FALSE, ''),
    ('TENURE', 3, 'Trusted (3-7yr)', 3, FALSE, ''),
    ('TENURE', 4, 'Veteran (7yr+)', 4, FALSE, ''),
    ('TENURE', 99, 'Not Specified', 99, FALSE, ''),
    ('MVC', 1, 'New', 1, FALSE, ''),
    ('MVC', 2, 'Activated', 2, TRUE, 'MVC_ACTIVE'),
    ('MVC', 3, 'Engaged', 3, TRUE, 'MVC_ACTIVE'),
    ('MVC', 4, 'Loyal', 4, TRUE, 'MVC_ACTIVE'),
    ('MVC', 5, 'Declining', 5, TRUE, 'MVC_RISK'),
    ('MVC', 6, 'Churned', 6, TRUE, 'MVC_RISK'),
    ('MVC', 99, 'Unacquired', 6, TRUE, ''),
    ('ENT_CNT', 1, '1 Entity Only', 1, FALSE, ''),
    ('ENT_CNT', 2, '2 Entities', 2, FALSE, ''),
    ('ENT_CNT', 3, '3+ Entities', 3, FALSE, '')
ON CONFLICT (feature_code, bucket_code) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    display_order = EXCLUDED.display_order,
    is_rollup = EXCLUDED.is_rollup,
    rollup_group = EXCLUDED.rollup_group;

INSERT INTO ecosystem_scope_feature (scope_code, feature_code, is_enabled)
VALUES
    ('AEON', 'PENETRATION_OPCO', TRUE),
    ('AEON', 'ENT_CNT', TRUE),
    ('ABG', 'ACTIVE_OPCO', TRUE),
    ('AMY', 'ACTIVE_OPCO', TRUE),
    ('AEON', 'MVC', FALSE),
    ('AEON', 'ACTIVE_OPCO', TRUE),
    ('AEON', 'MEMBER_MATRIX_ACTIVITY', TRUE)
ON CONFLICT (scope_code, feature_code) DO UPDATE SET
    is_enabled = EXCLUDED.is_enabled,
    updated_at = now();

COMMIT;
