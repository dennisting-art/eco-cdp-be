-- Static read queries for the Customer Intelligence Universe endpoint.

-- name: GetUniverseGroupScope :one
SELECT scope_code
FROM ecosystem_scope
WHERE scope_type = 'GROUP' AND is_available = TRUE;

-- name: ListUniverseOpcoScopes :many
SELECT scope_code, display_name, is_available
FROM ecosystem_scope
WHERE scope_type = 'OPCO'
ORDER BY display_order, scope_code;

-- name: GetUniverseMatrixFeatureEnabled :one
SELECT EXISTS (
    SELECT 1 FROM ecosystem_scope_feature AS esf
    JOIN feature_summary AS f ON f.feature_code = esf.feature_code
    WHERE esf.scope_code = $1
      AND esf.feature_code = 'MEMBER_MATRIX_ACTIVITY'
      AND esf.is_enabled = TRUE
      AND f.feature_context = 'UNIVERSE'
      AND f.feature_type = 'MATRIX'
) AS is_enabled;

-- name: GetUniverseEntityCountFeatureEnabled :one
SELECT EXISTS (
    SELECT 1 FROM ecosystem_scope_feature AS esf
    JOIN feature_summary AS f ON f.feature_code = esf.feature_code
    WHERE esf.scope_code = $1
      AND esf.feature_code = 'ENT_CNT'
      AND esf.is_enabled = TRUE
      AND f.feature_context = 'UNIVERSE'
      AND f.feature_type = 'DISTRIBUTION'
) AS is_enabled;

-- name: ListUniverseLatestSummaries :many
SELECT s.scope_code, s.as_of_date, s.total_member_count, s.active_member_count,
       s.inactive_member_count, s.total_retail_count, s.retail_member_count,
       s.retail_non_member_count
FROM customer_daily_summary AS s
JOIN (
    SELECT source.scope_code, MAX(source.as_of_date) AS as_of_date
    FROM customer_daily_summary AS source
    WHERE source.as_of_date BETWEEN $2 AND $3
    GROUP BY source.scope_code
) AS latest USING (scope_code, as_of_date)
WHERE s.scope_code = $1
   OR s.scope_code IN (
       SELECT scope_code FROM ecosystem_scope
       WHERE scope_type = 'OPCO' AND is_available = TRUE
   );

-- name: ListUniverseLatestAcquisition :many
SELECT a.as_of_date, a.acquisition_year, a.member_count, a.new_member_count, a.compare_count
FROM annual_member_acquisition AS a
WHERE a.scope_code = $1
  AND a.acquisition_year >= 2018
  AND a.acquisition_year BETWEEN EXTRACT(YEAR FROM $2::date)::integer - 1 AND EXTRACT(YEAR FROM $3::date)::integer
  AND a.as_of_date = (
      SELECT MAX(latest.as_of_date)
      FROM annual_member_acquisition AS latest
      WHERE latest.scope_code = $1
        AND latest.as_of_date BETWEEN $2 AND $3
  )
ORDER BY acquisition_year;

-- name: ListUniverseEntityBuckets :many
SELECT b.bucket_code, b.display_name, d.as_of_date, d.customer_count,
       d.ratio
FROM feature_bucket_dim AS b
LEFT JOIN member_daily_distribution AS d
  ON d.feature_code = b.feature_code
 AND d.bucket_code = b.bucket_code
 AND d.scope_code = $1
 AND d.as_of_date = (
     SELECT MAX(source.as_of_date)
     FROM member_daily_distribution AS source
     WHERE source.scope_code = $1 AND source.feature_code = 'ENT_CNT' AND source.as_of_date BETWEEN $2 AND $3
 )
WHERE b.feature_code = 'ENT_CNT'
ORDER BY b.display_order, b.bucket_code;

-- name: GetUniverseLatestMatrixDate :one
SELECT as_of_date
FROM member_entity_daily
WHERE scope_code = $1
  AND as_of_date BETWEEN $2 AND $3
ORDER BY as_of_date DESC
LIMIT 1;

-- name: ListUniverseMatrixCells :many
SELECT m.primary_entity_code, m.related_entity_code, m.member_count,
       m.ratio
FROM member_entity_daily AS m
WHERE m.scope_code = $1 AND m.as_of_date = $2
  AND m.related_entity_code IS NOT NULL;

-- name: ListUniverseMatrixTotals :many
SELECT m.primary_entity_code, m.member_count
FROM member_entity_daily AS m
WHERE m.scope_code = $1 AND m.as_of_date = $2
  AND m.related_entity_code IS NULL;
