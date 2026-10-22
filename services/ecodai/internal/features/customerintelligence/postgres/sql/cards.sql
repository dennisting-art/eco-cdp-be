-- Static reads for the Customer Intelligence Cards endpoint.

-- name: GetCardsScope :one
SELECT scope_code, is_available
FROM ecosystem_scope
WHERE scope_code = $1;

-- name: ListCardsFeatures :many
SELECT f.feature_code, f.display_name, f.feature_type, esf.is_enabled
FROM ecosystem_scope_feature AS esf
JOIN feature_summary AS f ON f.feature_code = esf.feature_code
WHERE esf.scope_code = $1
  AND f.feature_context = 'CARDS'
ORDER BY f.display_order, f.feature_code;

-- name: ListCardsDistributionBuckets :many
SELECT f.feature_code, b.bucket_code, b.display_name, b.is_rollup,
       b.rollup_group, d.as_of_date,
       d.customer_count, d.ratio
FROM ecosystem_scope_feature AS esf
JOIN feature_summary AS f ON f.feature_code = esf.feature_code
JOIN feature_bucket_dim AS b ON b.feature_code = f.feature_code
LEFT JOIN member_daily_distribution AS d
  ON d.scope_code = esf.scope_code
 AND d.feature_code = b.feature_code
 AND d.bucket_code = b.bucket_code
 AND d.as_of_date = (
     SELECT MAX(latest.as_of_date)
     FROM member_daily_distribution AS latest
     WHERE latest.scope_code = esf.scope_code
       AND latest.feature_code = f.feature_code
       AND latest.as_of_date BETWEEN $2 AND $3
 )
WHERE esf.scope_code = $1
  AND esf.is_enabled = TRUE
  AND f.feature_context = 'CARDS'
  AND f.feature_type = 'DISTRIBUTION'
ORDER BY f.display_order, f.feature_code, b.display_order, b.bucket_code;
