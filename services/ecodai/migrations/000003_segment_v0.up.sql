BEGIN;

CREATE TABLE IF NOT EXISTS segment_field_catalog_v0 (
    field_key TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    data_type TEXT NOT NULL,
    allowed_operators TEXT[] NOT NULL,
    enum_values TEXT[],
    min_value NUMERIC,
    max_value NUMERIC,
    source_mapping JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS segment_v0_element_sequence START WITH 4;

CREATE TABLE IF NOT EXISTS segments_v0 (
    id VARCHAR(32) PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('Predefined', 'Custom')),
    tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    description TEXT,
    filter_criteria JSONB NOT NULL DEFAULT '{}'::jsonb,
    estimated_member_count BIGINT NOT NULL DEFAULT 0 CHECK (estimated_member_count >= 0),
    last_updated_at TIMESTAMPTZ,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_segments_v0_tags ON segments_v0 USING GIN (tags);
CREATE INDEX IF NOT EXISTS idx_segments_v0_name_lower ON segments_v0 (LOWER(name));

-- Category and patched-count tables are separate from member build results.
-- An absent patch means the listing has no count, not a zero count.
CREATE TABLE IF NOT EXISTS segment_attribute_category_v0 (
    code TEXT PRIMARY KEY,
    display_name TEXT NOT NULL UNIQUE,
    sort_order INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'retired'))
);

CREATE TABLE IF NOT EXISTS segment_count_patch_v0 (
    patch_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    segment_id VARCHAR(32) NOT NULL REFERENCES segments_v0(id) ON DELETE CASCADE,
    org_id TEXT NOT NULL DEFAULT 'ALL',
    member_count BIGINT NOT NULL CHECK (member_count >= 0),
    population_basis TEXT NOT NULL,
    as_of_date DATE NOT NULL,
    source_ref TEXT NOT NULL,
    reviewed_by TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'replaced')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_segment_count_patch_v0_active
    ON segment_count_patch_v0 (segment_id, org_id) WHERE status = 'active';

-- Source-of-truth member attributes used by the temporary v0 rule evaluator.
-- distinct from segment_members_v0
CREATE TABLE IF NOT EXISTS member_cookbook_v0 (
    member_id VARCHAR(32) PRIMARY KEY,
    cc_active_flag BOOLEAN NOT NULL DEFAULT FALSE,
    local_or_foreigner TEXT,
    annual_income NUMERIC,
    race TEXT,
    avg_basket_value_last_30_days NUMERIC,
    basket_value_last_30_days NUMERIC,
    basket_count_last_30_days INTEGER,
    age_distribution TEXT,
    consent_flag BOOLEAN,
    consent_refreshed_at TIMESTAMPTZ,
    ingested_date DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_member_cookbook_v0_race ON member_cookbook_v0 (race);
CREATE INDEX IF NOT EXISTS idx_member_cookbook_v0_consent ON member_cookbook_v0 (consent_flag);
CREATE INDEX IF NOT EXISTS idx_member_cookbook_v0_age_distribution ON member_cookbook_v0 (age_distribution);
CREATE INDEX IF NOT EXISTS idx_member_cookbook_v0_basket_value_30d ON member_cookbook_v0 (basket_value_last_30_days);

CREATE TABLE IF NOT EXISTS segment_members_v0 (
    segment_id VARCHAR(32) NOT NULL REFERENCES segments_v0(id) ON DELETE CASCADE,
    member_id VARCHAR(32) NOT NULL REFERENCES member_cookbook_v0(member_id),
    computed_date DATE NOT NULL,
    PRIMARY KEY (segment_id, member_id, computed_date)
);
CREATE INDEX IF NOT EXISTS idx_segment_members_v0_snapshot ON segment_members_v0 (segment_id, computed_date);

CREATE TABLE IF NOT EXISTS segment_requests_v0 (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id TEXT NOT NULL UNIQUE,
    requested_by TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('estimate', 'confirm')),
    segments_payload JSONB NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('INIT', 'PROCESSING', 'READY', 'FAILED')),
    total_count BIGINT,
    consent_count BIGINT,
    bucket TEXT,
    snapshot_date DATE,
    snapshot_dates JSONB NOT NULL DEFAULT '{}'::jsonb,
    progress_stage TEXT NOT NULL DEFAULT 'QUEUED',
    files_completed INT NOT NULL DEFAULT 0,
    files_total INT NOT NULL DEFAULT 1,
    bytes_uploaded BIGINT NOT NULL DEFAULT 0,
    error_code TEXT,
    error_message TEXT,
    idempotency_key TEXT,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,
    UNIQUE (requested_by, idempotency_key)
);

CREATE TABLE IF NOT EXISTS segment_request_jobs_v0 (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    segment_request_id UUID NOT NULL UNIQUE REFERENCES segment_requests_v0(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PROCESSING', 'DONE', 'FAILED')),
    attempt_count INT NOT NULL DEFAULT 0,
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    locked_at TIMESTAMPTZ,
    locked_by TEXT,
    lease_until TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_segment_request_jobs_v0_due ON segment_request_jobs_v0 (status, next_run_at) WHERE status IN ('PENDING', 'PROCESSING');
CREATE INDEX IF NOT EXISTS idx_segment_request_jobs_v0_lease ON segment_request_jobs_v0 (lease_until) WHERE status='PROCESSING';

CREATE TABLE IF NOT EXISTS recipes_v0 (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    segment_request_id UUID NOT NULL UNIQUE REFERENCES segment_requests_v0(id) ON DELETE CASCADE,
    segments_payload JSONB NOT NULL,
    additional_rules JSONB NOT NULL DEFAULT '[]'::jsonb,
    member_count BIGINT,
    computed_at TIMESTAMPTZ,
    recipe_status TEXT NOT NULL DEFAULT 'TEMPORARY' CHECK (recipe_status IN ('TEMPORARY', 'PROMOTED', 'DISMISSED')),
    promoted_segment_id VARCHAR(32) REFERENCES segments_v0(id),
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS recipe_members_v0 (
    recipe_id UUID NOT NULL REFERENCES recipes_v0(id) ON DELETE CASCADE,
    member_id VARCHAR(32) NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (recipe_id, member_id)
);

-- Event history for future recent/trending segment and recipe views.
CREATE TABLE IF NOT EXISTS segment_usage_v0 (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    segment_id VARCHAR(32) REFERENCES segments_v0(id) ON DELETE SET NULL,
    recipe_id UUID REFERENCES recipes_v0(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('REQUEST_CONFIRMED', 'RECIPE_SAVED', 'RECIPE_PROMOTED')),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_segment_usage_v0_recent ON segment_usage_v0 (occurred_at DESC);

-- Prototype feed and publication contract. Source rows remain invisible until
-- the backend atomically publishes a fully reconciled READY build.
CREATE TABLE IF NOT EXISTS segment_build_v0 (
    build_id TEXT PRIMARY KEY,
    source_date DATE NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('LOADING','FAILED','READY')),
    required_outputs TEXT[] NOT NULL,
    manifest_version INT NOT NULL DEFAULT 1,
    source_member_count BIGINT,
    supersedes_build_id TEXT REFERENCES segment_build_v0(build_id),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS segment_current_build_v0 (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    build_id TEXT NOT NULL REFERENCES segment_build_v0(build_id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Local prototype counterpart of the data team's separate load_log.
CREATE TABLE IF NOT EXISTS segment_source_load_v0 (
    build_id TEXT NOT NULL REFERENCES segment_build_v0(build_id),
    source_name TEXT NOT NULL,
    source_version TEXT NOT NULL,
    row_count BIGINT NOT NULL CHECK (row_count >= 0),
    completed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (build_id,source_name)
);

CREATE TABLE IF NOT EXISTS member_facts_v0 (
    build_id TEXT NOT NULL REFERENCES segment_build_v0(build_id),
    member_id VARCHAR(32) NOT NULL,
    birth_date DATE,
    gender TEXT,
    gender_option_id VARCHAR(32) REFERENCES segments_v0(id),
    age_option_id VARCHAR(32) REFERENCES segments_v0(id),
    nationality_code TEXT,
    marital_status TEXT,
    membership_application_date DATE,
    race TEXT,
    race_option_id VARCHAR(32) REFERENCES segments_v0(id),
    local_or_foreigner TEXT,
    annual_income NUMERIC,
    avg_basket_value_last_30_days NUMERIC,
    basket_value_last_30_days NUMERIC,
    basket_count_last_30_days INTEGER,
    age_distribution TEXT,
    spend_last_30_days NUMERIC,
    cc_active_flag BOOLEAN,
    build_consent BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (build_id,member_id)
);
CREATE INDEX IF NOT EXISTS idx_member_facts_v0_gender ON member_facts_v0 (build_id,gender_option_id,member_id);
CREATE INDEX IF NOT EXISTS idx_member_facts_v0_age ON member_facts_v0 (build_id,age_option_id,member_id);
CREATE INDEX IF NOT EXISTS idx_member_facts_v0_race ON member_facts_v0 (build_id,race_option_id,member_id);

CREATE TABLE IF NOT EXISTS member_org_v0 (
    build_id TEXT NOT NULL,
    member_id VARCHAR(32) NOT NULL,
    org_id TEXT NOT NULL,
    PRIMARY KEY (build_id,member_id,org_id),
    FOREIGN KEY (build_id,member_id) REFERENCES member_facts_v0(build_id,member_id)
);
CREATE INDEX IF NOT EXISTS idx_member_org_v0_org ON member_org_v0 (build_id,org_id,member_id);
CREATE TABLE IF NOT EXISTS member_retail_v0 (
    build_id TEXT NOT NULL,
    member_id VARCHAR(32) NOT NULL,
    retail_id TEXT NOT NULL,
    last_txn_date DATE,
    PRIMARY KEY (build_id,member_id,retail_id),
    FOREIGN KEY (build_id,member_id) REFERENCES member_facts_v0(build_id,member_id)
);
CREATE INDEX IF NOT EXISTS idx_member_retail_v0_retail ON member_retail_v0 (build_id,retail_id,member_id);
CREATE TABLE IF NOT EXISTS member_product_v0 (
    build_id TEXT NOT NULL,
    member_id VARCHAR(32) NOT NULL,
    product_id TEXT NOT NULL,
    PRIMARY KEY (build_id,member_id,product_id),
    FOREIGN KEY (build_id,member_id) REFERENCES member_facts_v0(build_id,member_id)
);
CREATE INDEX IF NOT EXISTS idx_member_product_v0_product ON member_product_v0 (build_id,product_id,member_id);
CREATE TABLE IF NOT EXISTS member_predefined_v0 (
    build_id TEXT NOT NULL,
    member_id VARCHAR(32) NOT NULL,
    segment_id VARCHAR(32) NOT NULL REFERENCES segments_v0(id),
    PRIMARY KEY (build_id,member_id,segment_id),
    FOREIGN KEY (build_id,member_id) REFERENCES member_facts_v0(build_id,member_id)
);
CREATE INDEX IF NOT EXISTS idx_member_predefined_v0_segment ON member_predefined_v0 (build_id,segment_id,member_id);

CREATE TABLE IF NOT EXISTS member_current_consent_v0 (
    member_id VARCHAR(32) PRIMARY KEY,
    consent BOOLEAN NOT NULL,
    refreshed_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS segment_build_count_v0 (
    build_id TEXT NOT NULL REFERENCES segment_build_v0(build_id),
    segment_id VARCHAR(32) NOT NULL REFERENCES segments_v0(id),
    definition_version INT NOT NULL CHECK (definition_version > 0),
    org_id TEXT NOT NULL DEFAULT 'ALL',
    raw_count BIGINT NOT NULL CHECK (raw_count >= 0),
    build_consented_count BIGINT NOT NULL CHECK (build_consented_count >= 0),
    computed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (build_id,segment_id,definition_version,org_id)
);

ALTER TABLE segment_requests_v0
    ADD COLUMN build_id TEXT REFERENCES segment_build_v0(build_id),
    ADD COLUMN org_id TEXT NOT NULL DEFAULT 'ALL',
    ADD COLUMN definition_versions JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN build_consent_count BIGINT,
    ADD COLUMN consent_checked_at TIMESTAMPTZ;
ALTER TABLE recipes_v0
    ADD COLUMN build_id TEXT REFERENCES segment_build_v0(build_id),
    ADD COLUMN org_id TEXT NOT NULL DEFAULT 'ALL';

-- Slice 3 storage is exercised by the local synthetic prototype. The org guard
-- remains a separate release gate. Ordinals are global across builds.
CREATE TABLE IF NOT EXISTS member_ordinal_v0 (
    ordinal BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    member_id VARCHAR(32) NOT NULL UNIQUE,
    retired_at TIMESTAMPTZ,
    CHECK (ordinal BETWEEN 1 AND 4294967295)
);
CREATE TABLE IF NOT EXISTS member_bitmap_v0 (
    build_id TEXT NOT NULL REFERENCES segment_build_v0(build_id),
    predicate_kind TEXT NOT NULL,
    predicate_id TEXT NOT NULL,
    definition_version INT NOT NULL DEFAULT 1,
    encoding_version INT NOT NULL DEFAULT 1,
    cardinality BIGINT NOT NULL CHECK (cardinality >= 0),
    checksum TEXT NOT NULL,
    payload BYTEA NOT NULL,
    PRIMARY KEY (build_id,predicate_kind,predicate_id)
);
CREATE TABLE IF NOT EXISTS dashboard_result_v0 (
    build_id TEXT NOT NULL REFERENCES segment_build_v0(build_id),
    org_id TEXT NOT NULL DEFAULT 'ALL',
    retail_scope TEXT NOT NULL DEFAULT 'ALL',
    metric_id TEXT NOT NULL,
    dimension_key TEXT NOT NULL DEFAULT 'ALL',
    value NUMERIC NOT NULL,
    PRIMARY KEY (build_id,org_id,retail_scope,metric_id,dimension_key)
);
CREATE TABLE IF NOT EXISTS customer_monthly_summary_v0 (
    month DATE NOT NULL,
    org_id TEXT NOT NULL,
    retail_scope TEXT NOT NULL,
    metric_id TEXT NOT NULL,
    revision INT NOT NULL CHECK (revision > 0),
    source_build_id TEXT NOT NULL REFERENCES segment_build_v0(build_id),
    value NUMERIC NOT NULL,
    is_current BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (month,org_id,retail_scope,metric_id,revision)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_monthly_summary_v0_current
    ON customer_monthly_summary_v0 (month,org_id,retail_scope,metric_id) WHERE is_current;

-- The required outputs and business date are fixed when a load is reserved.
-- READY builds and their source/output rows cannot be edited after publication.
CREATE FUNCTION segment_build_manifest_guard_v0() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.required_outputs IS DISTINCT FROM OLD.required_outputs
       OR NEW.source_date IS DISTINCT FROM OLD.source_date
       OR NEW.manifest_version IS DISTINCT FROM OLD.manifest_version THEN
        RAISE EXCEPTION 'segment build manifest is immutable';
    END IF;
    IF OLD.status <> 'LOADING' THEN
        RAISE EXCEPTION 'published or failed segment build is immutable';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER segment_build_manifest_guard_v0
    BEFORE UPDATE ON segment_build_v0 FOR EACH ROW
    EXECUTE FUNCTION segment_build_manifest_guard_v0();

CREATE FUNCTION segment_build_rows_guard_v0() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    row_build_id TEXT;
    build_status TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        row_build_id := OLD.build_id;
    ELSE
        row_build_id := NEW.build_id;
        IF TG_OP = 'UPDATE' AND OLD.build_id <> NEW.build_id THEN
            RAISE EXCEPTION 'moving segment rows between builds is forbidden';
        END IF;
    END IF;
    SELECT status INTO build_status FROM segment_build_v0
        WHERE build_id=row_build_id FOR SHARE;
    IF build_status IS DISTINCT FROM 'LOADING' THEN
        RAISE EXCEPTION 'segment build % rows are immutable in status %', row_build_id, build_status;
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;
DO $$
DECLARE table_name TEXT;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'segment_source_load_v0','member_facts_v0','member_org_v0',
        'member_retail_v0','member_product_v0','member_predefined_v0',
        'segment_build_count_v0','member_bitmap_v0','dashboard_result_v0'
    ] LOOP
        EXECUTE format('CREATE TRIGGER segment_build_rows_guard_v0 BEFORE INSERT OR UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION segment_build_rows_guard_v0()', table_name);
    END LOOP;
END;
$$;

COMMIT;
