package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
)

// BuildPublisher validates a complete, unpublished prototype feed and advances
// the current-build pointer only after every required SQL output is ready.
type BuildPublisher struct{ db *database.PostgresDB }

func NewBuildPublisher(db *database.PostgresDB) *BuildPublisher { return &BuildPublisher{db: db} }

var prototypeSources = []string{
	"member_facts_v0", "member_org_v0", "member_retail_v0",
	"member_product_v0", "member_predefined_v0",
}

func (p *BuildPublisher) Publish(ctx context.Context, buildID string) error {
	err := p.publish(ctx, buildID)
	if err != nil {
		_, _ = p.db.Pool.Exec(ctx, `UPDATE segment_build_v0 SET status='FAILED',error_message=$2
			WHERE build_id=$1 AND status='LOADING'`, buildID, err.Error())
	}
	return err
}

func (p *BuildPublisher) publish(ctx context.Context, buildID string) error {
	tx, err := p.db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('segment_build_v0_publish'))`); err != nil {
		return err
	}
	var sourceDate time.Time
	var status string
	var required []string
	var expected *int64
	err = tx.QueryRow(ctx, `SELECT source_date,status,required_outputs,source_member_count
		FROM segment_build_v0 WHERE build_id=$1 FOR UPDATE`, buildID).
		Scan(&sourceDate, &status, &required, &expected)
	if err != nil {
		return err
	}
	if status != "LOADING" {
		return fmt.Errorf("build %s is %s, expected LOADING", buildID, status)
	}
	outputs := map[string]bool{}
	for _, name := range required {
		if outputs[name] {
			return fmt.Errorf("duplicate required output %q", name)
		}
		outputs[name] = true
		if name != "source" && name != "sql_counts" && name != "bitmaps" && name != "dashboards" {
			return fmt.Errorf("output %q is not implemented by the SQL publisher", name)
		}
	}
	if !outputs["source"] || !outputs["sql_counts"] {
		return fmt.Errorf("build %s requires source and sql_counts outputs", buildID)
	}
	if outputs["dashboards"] && !outputs["bitmaps"] {
		return fmt.Errorf("dashboard output requires bitmap output in this slice")
	}
	for _, source := range prototypeSources {
		var logged int64
		if err := tx.QueryRow(ctx, `SELECT row_count FROM segment_source_load_v0
			WHERE build_id=$1 AND source_name=$2 AND completed_at IS NOT NULL`, buildID, source).Scan(&logged); err != nil {
			return fmt.Errorf("source %s has no completed load: %w", source, err)
		}
		var actual int64
		query := "SELECT COUNT(*) FROM " + source + " WHERE build_id=$1"
		if err := tx.QueryRow(ctx, query, buildID).Scan(&actual); err != nil {
			return err
		}
		if actual != logged {
			return fmt.Errorf("source %s has %d rows, load log says %d", source, actual, logged)
		}
		if source == "member_facts_v0" && expected != nil && actual != *expected {
			return fmt.Errorf("source has %d members, build expects %d", actual, *expected)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE member_facts_v0 SET
		gender_option_id=CASE gender WHEN 'Male' THEN 'seg_100015' WHEN 'Female' THEN 'seg_100016' ELSE NULL END,
		race_option_id=CASE race WHEN 'malay' THEN 'seg_100019' WHEN 'chinese' THEN 'seg_100020'
			WHEN 'indians' THEN 'seg_100021' WHEN 'other' THEN 'seg_100022' ELSE NULL END,
		age_option_id=CASE
			WHEN birth_date IS NULL OR birth_date>$2::date THEN NULL
			WHEN EXTRACT(YEAR FROM AGE($2::date,birth_date))::int <= 25 THEN 'seg_100003'
			WHEN EXTRACT(YEAR FROM AGE($2::date,birth_date))::int <= 35 THEN 'seg_100004'
			WHEN EXTRACT(YEAR FROM AGE($2::date,birth_date))::int <= 45 THEN 'seg_100005'
			WHEN EXTRACT(YEAR FROM AGE($2::date,birth_date))::int <= 55 THEN 'seg_100006'
			ELSE 'seg_100007' END
		WHERE build_id=$1`, buildID, sourceDate); err != nil {
		return fmt.Errorf("derive backend-owned options: %w", err)
	}
	var invalid int64
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM member_predefined_v0 mp
		JOIN segments_v0 s ON s.id=mp.segment_id
		WHERE mp.build_id=$1 AND (s.kind<>'predefined' OR s.definition_status<>'approved')`, buildID).Scan(&invalid); err != nil {
		return err
	}
	if invalid != 0 {
		return fmt.Errorf("%d predefined membership rows reference unapproved definitions", invalid)
	}
	rows, err := tx.Query(ctx, `SELECT id,kind,definition_version,fixed_rule FROM segments_v0
		WHERE is_visible AND definition_status='approved' AND kind IN ('option','predefined') ORDER BY id`)
	if err != nil {
		return err
	}
	type definition struct {
		id, kind string
		version  int
		rule     json.RawMessage
	}
	definitions := []definition{}
	for rows.Next() {
		var d definition
		if err := rows.Scan(&d.id, &d.kind, &d.version, &d.rule); err != nil {
			rows.Close()
			return err
		}
		definitions = append(definitions, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, d := range definitions {
		var raw, consented int64
		if d.kind == "predefined" {
			err = tx.QueryRow(ctx, `SELECT COUNT(*),COUNT(*) FILTER (WHERE m.build_consent)
				FROM member_predefined_v0 mp JOIN member_facts_v0 m
				ON m.build_id=mp.build_id AND m.member_id=mp.member_id
				WHERE mp.build_id=$1 AND mp.segment_id=$2`, buildID, d.id).Scan(&raw, &consented)
		} else {
			if len(d.rule) == 0 {
				return fmt.Errorf("option %s has no SQL rule", d.id)
			}
			args := []interface{}{buildID, sourceDate}
			next := 3
			condition, ruleErr := attributeRuleCondition(d.rule, &args, &next)
			if ruleErr != nil {
				return fmt.Errorf("option %s: %w", d.id, ruleErr)
			}
			query := `SELECT COUNT(*),COUNT(*) FILTER (WHERE m.build_consent)
				FROM member_facts_v0 m WHERE m.build_id=$1 AND $2::date IS NOT NULL AND ` + condition
			err = tx.QueryRow(ctx, query, args...).Scan(&raw, &consented)
		}
		if err != nil {
			return fmt.Errorf("count %s: %w", d.id, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO segment_build_count_v0
			(build_id,segment_id,definition_version,org_id,raw_count,build_consented_count)
			VALUES ($1,$2,$3,'ALL',$4,$5)`, buildID, d.id, d.version, raw, consented); err != nil {
			return err
		}
	}
	if outputs["bitmaps"] {
		if err := buildBitmaps(ctx, tx, buildID, sourceDate); err != nil {
			return err
		}
	}
	if outputs["dashboards"] {
		if err := buildDashboard(ctx, tx, buildID, sourceDate); err != nil {
			return err
		}
	}
	var memberCount int64
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM member_facts_v0 WHERE build_id=$1`, buildID).Scan(&memberCount); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE segment_build_v0 SET status='READY',source_member_count=$2,
		published_at=now(),error_message=NULL WHERE build_id=$1`, buildID, memberCount); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO segment_current_build_v0 (singleton,build_id)
		VALUES (TRUE,$1) ON CONFLICT (singleton) DO UPDATE SET build_id=excluded.build_id,updated_at=now()`, buildID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
