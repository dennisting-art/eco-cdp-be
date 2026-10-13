BEGIN;

DELETE FROM ecosystem_scope_feature
WHERE (scope_code, feature_code) IN (
    ('AEON', 'PENETRATION_OPCO'),
    ('AEON', 'ENT_CNT'),
    ('ABG', 'ACTIVE_OPCO'),
    ('AMY', 'ACTIVE_OPCO'),
    ('AEON', 'MVC'),
    ('AEON', 'ACTIVE_OPCO'),
    ('AEON', 'MEMBER_MATRIX_ACTIVITY')
);

DELETE FROM feature_bucket_dim
WHERE (feature_code, bucket_code) IN (
    ('SEX', 1), ('SEX', 2), ('SEX', 99),
    ('AGE', 1), ('AGE', 2), ('AGE', 3), ('AGE', 4), ('AGE', 5), ('AGE', 99),
    ('TIER', 1), ('TIER', 2), ('TIER', 3), ('TIER', 4),
    ('TENURE', 1), ('TENURE', 2), ('TENURE', 3), ('TENURE', 4), ('TENURE', 99),
    ('MVC', 1), ('MVC', 2), ('MVC', 3), ('MVC', 4), ('MVC', 5), ('MVC', 6), ('MVC', 99),
    ('ENT_CNT', 1), ('ENT_CNT', 2), ('ENT_CNT', 3)
);

-- MEMBER_MATRIX_ACTIVITY belongs to migration 000001.
DELETE FROM feature_summary
WHERE feature_code IN (
    'SEX', 'AGE', 'TIER', 'TENURE', 'MVC',
    'PENETRATION_OPCO', 'ACTIVE_OPCO', 'ENT_CNT'
);

DELETE FROM ecosystem_scope
WHERE scope_code IN ('AEON', 'ABG', 'AMY', 'ACSM', 'ADB');

COMMIT;
