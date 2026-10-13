BEGIN;

DROP VIEW vw_monthly_member_penetration RESTRICT;

DROP TRIGGER trg_member_daily_monthly_sync ON member_daily_summary;
DROP TRIGGER trg_member_daily_touch ON member_daily_summary;

DROP FUNCTION write_member_daily_summary(JSONB) RESTRICT;
DROP FUNCTION sync_member_monthly_summary() RESTRICT;
DROP FUNCTION touch_member_daily_summary() RESTRICT;
DROP FUNCTION refresh_member_monthly_summary(TEXT, DATE) RESTRICT;

DROP TABLE transaction_daily_summary RESTRICT;
DROP TABLE member_monthly_summary RESTRICT;
DROP TABLE member_daily_summary RESTRICT;
DROP TABLE member_entity_daily RESTRICT;
DROP TABLE annual_member_acquisition RESTRICT;
DROP TABLE member_daily_distribution RESTRICT;
DROP TABLE customer_daily_summary RESTRICT;
DROP TABLE ecosystem_scope_feature RESTRICT;
DROP TABLE feature_bucket_dim RESTRICT;
DROP TABLE feature_summary RESTRICT;
DROP TABLE ecosystem_scope RESTRICT;

COMMIT;
