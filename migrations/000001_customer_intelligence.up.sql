BEGIN;

CREATE TABLE ecosystem_scope (
    scope_code VARCHAR(30) PRIMARY KEY,
    scope_type VARCHAR(20) NOT NULL CHECK (scope_type IN ('GROUP', 'OPCO')),
    display_name VARCHAR(100) NOT NULL,
    display_order SMALLINT NOT NULL,
    is_available BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ecosystem_scope_one_group
    ON ecosystem_scope (scope_type)
    WHERE scope_type = 'GROUP';

CREATE TABLE feature_summary (
    feature_code VARCHAR(30) PRIMARY KEY,
    display_name VARCHAR(100) NOT NULL,
    description VARCHAR(255),
    display_order SMALLINT NOT NULL,
    feature_type TEXT CHECK (
        feature_type IN ('TREND', 'DISTRIBUTION', 'COMBO_TREND', 'MATRIX')
    ),
    feature_context TEXT NOT NULL DEFAULT 'CARDS'
        CHECK (feature_context IN ('CARDS', 'UNIVERSE'))
);

INSERT INTO feature_summary (
    feature_code, display_name, description, display_order, feature_type, feature_context
) VALUES (
    'MEMBER_MATRIX_ACTIVITY', 'Member Matrix Activities',
    'Group member overlap by entity', 1, 'MATRIX', 'UNIVERSE'
);

CREATE TABLE feature_bucket_dim (
    feature_code VARCHAR(30) NOT NULL REFERENCES feature_summary(feature_code),
    bucket_code SMALLINT NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    display_order SMALLINT NOT NULL,
    is_rollup BOOLEAN NOT NULL DEFAULT FALSE,
    rollup_group VARCHAR(20),
    PRIMARY KEY (feature_code, bucket_code)
);

CREATE TABLE ecosystem_scope_feature (
    scope_code VARCHAR(30) NOT NULL REFERENCES ecosystem_scope(scope_code),
    feature_code VARCHAR(30) NOT NULL REFERENCES feature_summary(feature_code),
    is_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_code, feature_code)
);

CREATE TABLE customer_daily_summary (
    as_of_date DATE NOT NULL,
    scope_code VARCHAR(30) NOT NULL REFERENCES ecosystem_scope(scope_code),
    total_member_count BIGINT CHECK (total_member_count >= 0),
    active_member_count BIGINT CHECK (active_member_count >= 0),
    inactive_member_count BIGINT CHECK (inactive_member_count >= 0),
    total_retail_count BIGINT CHECK (total_retail_count >= 0),
    retail_member_count BIGINT NOT NULL CHECK (retail_member_count >= 0),
    retail_non_member_count BIGINT NOT NULL CHECK (retail_non_member_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (as_of_date, scope_code)
);

CREATE INDEX idx_customer_daily_summary_scope_date
    ON customer_daily_summary (scope_code, as_of_date DESC);

CREATE TABLE member_daily_distribution (
    as_of_date DATE NOT NULL,
    scope_code VARCHAR(30) NOT NULL REFERENCES ecosystem_scope(scope_code),
    feature_code VARCHAR(30) NOT NULL,
    bucket_code SMALLINT NOT NULL,
    customer_count BIGINT NOT NULL CHECK (customer_count >= 0),
    ratio NUMERIC(12, 6) NOT NULL CHECK (ratio >= 0 AND ratio <= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (as_of_date, scope_code, feature_code, bucket_code),
    FOREIGN KEY (feature_code, bucket_code)
        REFERENCES feature_bucket_dim(feature_code, bucket_code)
);

CREATE INDEX idx_member_daily_distribution_scope_feature_date
    ON member_daily_distribution (scope_code, feature_code, as_of_date DESC);

CREATE TABLE annual_member_acquisition (
    as_of_date DATE NOT NULL,
    scope_code VARCHAR(30) NOT NULL REFERENCES ecosystem_scope(scope_code),
    acquisition_year INTEGER NOT NULL CHECK (acquisition_year BETWEEN 1900 AND 9999),
    member_count BIGINT NOT NULL CHECK (member_count >= 0),
    new_member_count BIGINT NOT NULL CHECK (new_member_count >= 0),
    compare_count BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (as_of_date, scope_code, acquisition_year)
);

CREATE INDEX idx_annual_member_acquisition_scope_date_year
    ON annual_member_acquisition (scope_code, as_of_date DESC, acquisition_year);

CREATE TABLE member_entity_daily (
    scope_code VARCHAR(30) NOT NULL REFERENCES ecosystem_scope(scope_code),
    as_of_date DATE NOT NULL,
    primary_entity_code VARCHAR(30) NOT NULL REFERENCES ecosystem_scope(scope_code),
    related_entity_code VARCHAR(30) REFERENCES ecosystem_scope(scope_code),
    member_count BIGINT NOT NULL CHECK (member_count >= 0),
    ratio NUMERIC(12, 6) CHECK (ratio BETWEEN 0 AND 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    id BIGINT GENERATED ALWAYS AS IDENTITY,
    PRIMARY KEY (id),
    CONSTRAINT chk_member_entity_daily_row_shape CHECK (
        (related_entity_code IS NULL AND ratio IS NULL)
        OR
        (related_entity_code IS NOT NULL AND btrim(related_entity_code) <> ''
         AND ratio IS NOT NULL AND ratio BETWEEN 0 AND 1)
    )
);

CREATE INDEX member_entity_daily_latest
    ON member_entity_daily (scope_code, as_of_date DESC);

CREATE UNIQUE INDEX member_entity_daily_primary_total_unique
    ON member_entity_daily (scope_code, as_of_date, primary_entity_code)
    WHERE related_entity_code IS NULL;

CREATE UNIQUE INDEX member_entity_daily_relationship_unique
    ON member_entity_daily (
        scope_code, as_of_date, primary_entity_code, related_entity_code
    )
    WHERE related_entity_code IS NOT NULL;

CREATE TABLE member_daily_summary (
    as_of_date DATE NOT NULL,
    scope_code VARCHAR(30) NOT NULL REFERENCES ecosystem_scope(scope_code),
    active_member_count BIGINT NOT NULL CHECK (active_member_count >= 0),
    inactive_member_count BIGINT NOT NULL CHECK (inactive_member_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (as_of_date, scope_code)
);

CREATE INDEX idx_member_daily_summary_scope_date
    ON member_daily_summary (scope_code, as_of_date DESC);

CREATE TABLE member_monthly_summary (
    scope_code VARCHAR(30) NOT NULL REFERENCES ecosystem_scope(scope_code),
    report_month DATE NOT NULL,
    as_of_date DATE NOT NULL,
    active_member_count BIGINT NOT NULL CHECK (active_member_count >= 0),
    inactive_member_count BIGINT NOT NULL CHECK (inactive_member_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_code, report_month),
    CONSTRAINT chk_report_month_first_day
        CHECK (EXTRACT(DAY FROM report_month) = 1),
    CONSTRAINT chk_member_monthly_source_month CHECK (
        report_month = date_trunc('month', as_of_date::timestamp)::date
    )
);

CREATE TABLE transaction_daily_summary (
    as_of_date DATE NOT NULL,
    scope_code VARCHAR(30) NOT NULL REFERENCES ecosystem_scope(scope_code),
    member_net_sales BIGINT NOT NULL CHECK (member_net_sales >= 0),
    non_member_net_sales BIGINT NOT NULL CHECK (non_member_net_sales >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (as_of_date, scope_code)
);

CREATE VIEW vw_monthly_member_penetration AS
SELECT date_trunc('month', as_of_date)::date AS report_month,
       scope_code,
       SUM(member_net_sales)::BIGINT AS member_net_sales,
       SUM(non_member_net_sales)::BIGINT AS non_member_net_sales,
       (SUM(member_net_sales) + SUM(non_member_net_sales))::BIGINT AS total_net_sales,
       CASE
           WHEN SUM(member_net_sales) + SUM(non_member_net_sales) = 0
               THEN 0::NUMERIC
           ELSE ROUND(
               SUM(member_net_sales) * 100.0
               / (SUM(member_net_sales) + SUM(non_member_net_sales)),
               2
           )
       END AS member_penetration_percentage
FROM transaction_daily_summary
GROUP BY date_trunc('month', as_of_date)::date, scope_code;

CREATE FUNCTION refresh_member_monthly_summary(p_scope TEXT, p_month DATE)
RETURNS void LANGUAGE plpgsql VOLATILE AS $$
DECLARE
    latest member_daily_summary%ROWTYPE;
BEGIN
    SELECT * INTO latest
    FROM member_daily_summary
    WHERE scope_code = p_scope
      AND as_of_date >= p_month
      AND as_of_date < p_month + INTERVAL '1 month'
    ORDER BY as_of_date DESC
    LIMIT 1;

    IF FOUND THEN
        INSERT INTO member_monthly_summary AS monthly
            (scope_code, report_month, as_of_date, active_member_count, inactive_member_count)
        VALUES (p_scope, p_month, latest.as_of_date,
                latest.active_member_count, latest.inactive_member_count)
        ON CONFLICT (scope_code, report_month) DO UPDATE SET
            as_of_date = EXCLUDED.as_of_date,
            active_member_count = EXCLUDED.active_member_count,
            inactive_member_count = EXCLUDED.inactive_member_count,
            updated_at = clock_timestamp()
        WHERE (monthly.as_of_date, monthly.active_member_count, monthly.inactive_member_count)
            IS DISTINCT FROM
              (EXCLUDED.as_of_date, EXCLUDED.active_member_count, EXCLUDED.inactive_member_count);
    ELSE
        DELETE FROM member_monthly_summary
        WHERE scope_code = p_scope AND report_month = p_month;
    END IF;
END;
$$;

CREATE FUNCTION sync_member_monthly_summary()
RETURNS trigger LANGUAGE plpgsql VOLATILE AS $$
DECLARE
    old_month DATE;
    new_month DATE;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        old_month := date_trunc('month', OLD.as_of_date::timestamp)::date;
        PERFORM refresh_member_monthly_summary(OLD.scope_code, old_month);
    END IF;
    IF TG_OP <> 'DELETE' THEN
        new_month := date_trunc('month', NEW.as_of_date::timestamp)::date;
        IF TG_OP = 'INSERT' OR
           (NEW.scope_code, new_month) IS DISTINCT FROM (OLD.scope_code, old_month) THEN
            PERFORM refresh_member_monthly_summary(NEW.scope_code, new_month);
        END IF;
    END IF;
    RETURN NULL;
END;
$$;

CREATE FUNCTION touch_member_daily_summary()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.created_at := OLD.created_at;
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_member_daily_touch
BEFORE UPDATE ON member_daily_summary
FOR EACH ROW EXECUTE FUNCTION touch_member_daily_summary();

CREATE TRIGGER trg_member_daily_monthly_sync
AFTER INSERT OR UPDATE OR DELETE ON member_daily_summary
FOR EACH ROW EXECUTE FUNCTION sync_member_monthly_summary();

CREATE FUNCTION write_member_daily_summary(p_changes JSONB)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER AS $$
DECLARE
    change JSONB;
    scope TEXT;
    source_date DATE;
BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'member summary ingestion requires READ COMMITTED'
            USING ERRCODE = '25000';
    END IF;
    IF p_changes IS NULL OR jsonb_typeof(p_changes) <> 'array' THEN
        RAISE EXCEPTION 'member summary changes must be a JSON array'
            USING ERRCODE = '22023';
    END IF;

    FOR change IN SELECT value FROM jsonb_array_elements(p_changes) LOOP
        IF jsonb_typeof(change) IS DISTINCT FROM 'object'
           OR (change->>'operation') IS NULL
           OR (change->>'operation') NOT IN ('upsert', 'delete')
           OR jsonb_typeof(change->'scope_code') IS DISTINCT FROM 'string'
           OR length(change->>'scope_code') NOT BETWEEN 1 AND 30
           OR jsonb_typeof(change->'as_of_date') IS DISTINCT FROM 'string'
           OR (change->>'as_of_date') !~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$' THEN
            RAISE EXCEPTION 'invalid member summary operation, scope_code or as_of_date'
                USING ERRCODE = '22023';
        END IF;
        source_date := (change->>'as_of_date')::date;
        IF change->>'operation' = 'upsert' THEN
            IF jsonb_typeof(change->'active_member_count') IS DISTINCT FROM 'number'
               OR jsonb_typeof(change->'inactive_member_count') IS DISTINCT FROM 'number' THEN
                RAISE EXCEPTION 'member summary counts must be nonnegative integers'
                    USING ERRCODE = '22023';
            END IF;
            IF (change->>'active_member_count')::bigint < 0
               OR (change->>'inactive_member_count')::bigint < 0 THEN
                RAISE EXCEPTION 'member summary counts must be nonnegative integers'
                    USING ERRCODE = '22023';
            END IF;
        END IF;
    END LOOP;

    FOR scope IN
        SELECT DISTINCT value->>'scope_code'
        FROM jsonb_array_elements(p_changes)
        ORDER BY 1
    LOOP
        PERFORM 1 FROM ecosystem_scope WHERE scope_code = scope FOR UPDATE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'unknown member summary scope: %', scope
                USING ERRCODE = '23503';
        END IF;
    END LOOP;

    FOR change IN SELECT value FROM jsonb_array_elements(p_changes) LOOP
        IF change->>'operation' = 'delete' THEN
            DELETE FROM member_daily_summary
            WHERE scope_code = change->>'scope_code'
              AND as_of_date = (change->>'as_of_date')::date;
        ELSE
            INSERT INTO member_daily_summary AS daily
                (scope_code, as_of_date, active_member_count, inactive_member_count)
            VALUES (change->>'scope_code', (change->>'as_of_date')::date,
                    (change->>'active_member_count')::bigint,
                    (change->>'inactive_member_count')::bigint)
            ON CONFLICT (as_of_date, scope_code) DO UPDATE SET
                active_member_count = EXCLUDED.active_member_count,
                inactive_member_count = EXCLUDED.inactive_member_count
            WHERE (daily.active_member_count, daily.inactive_member_count)
                IS DISTINCT FROM (EXCLUDED.active_member_count, EXCLUDED.inactive_member_count);
        END IF;
    END LOOP;
END;
$$;

DO $$
DECLARE
    target_schema TEXT := current_schema();
    signature TEXT;
BEGIN
    FOREACH signature IN ARRAY ARRAY[
        'refresh_member_monthly_summary(text,date)',
        'sync_member_monthly_summary()',
        'touch_member_daily_summary()',
        'write_member_daily_summary(jsonb)'
    ] LOOP
        EXECUTE format('ALTER FUNCTION %I.%s SET search_path = pg_catalog, %I, pg_temp',
                       target_schema, signature, target_schema);
    END LOOP;
END;
$$;

REVOKE ALL ON FUNCTION refresh_member_monthly_summary(TEXT, DATE) FROM PUBLIC;
REVOKE ALL ON FUNCTION sync_member_monthly_summary() FROM PUBLIC;
REVOKE ALL ON FUNCTION touch_member_daily_summary() FROM PUBLIC;
REVOKE ALL ON FUNCTION write_member_daily_summary(JSONB) FROM PUBLIC;
REVOKE ALL ON member_daily_summary FROM PUBLIC;

COMMIT;
