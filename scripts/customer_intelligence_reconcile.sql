-- Run separately against legacy and eco databases at an agreed snapshot point.
-- Compare counts, newest dates, reference keys, and representative API output.
WITH inventory(name, row_count, newest_date) AS (
    SELECT 'ecosystem_scope', count(*), NULL::date FROM ecosystem_scope
    UNION ALL SELECT 'feature_summary', count(*), NULL::date FROM feature_summary
    UNION ALL SELECT 'feature_bucket_dim', count(*), NULL::date FROM feature_bucket_dim
    UNION ALL SELECT 'ecosystem_scope_feature', count(*), NULL::date FROM ecosystem_scope_feature
    UNION ALL SELECT 'customer_daily_summary', count(*), max(as_of_date) FROM customer_daily_summary
    UNION ALL SELECT 'member_daily_distribution', count(*), max(as_of_date) FROM member_daily_distribution
    UNION ALL SELECT 'annual_member_acquisition', count(*), max(as_of_date) FROM annual_member_acquisition
    UNION ALL SELECT 'member_entity_daily', count(*), max(as_of_date) FROM member_entity_daily
    UNION ALL SELECT 'member_daily_summary', count(*), max(as_of_date) FROM member_daily_summary
    UNION ALL SELECT 'member_monthly_summary', count(*), max(as_of_date) FROM member_monthly_summary
    UNION ALL SELECT 'transaction_daily_summary', count(*), max(as_of_date) FROM transaction_daily_summary
)
SELECT * FROM inventory ORDER BY name;

SELECT scope_code, scope_type, display_name, is_available FROM ecosystem_scope ORDER BY scope_code;
SELECT feature_code, feature_type, feature_context FROM feature_summary ORDER BY feature_code;
