package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/jackc/pgx/v5"
)

func buildBitmaps(ctx context.Context, tx pgx.Tx, buildID string, sourceDate time.Time) error {
	if _, err := tx.Exec(ctx, `INSERT INTO member_ordinal_v0 (member_id)
		SELECT member_id FROM member_facts_v0 WHERE build_id=$1 ORDER BY member_id
		ON CONFLICT (member_id) DO NOTHING`, buildID); err != nil {
		return fmt.Errorf("assign ordinals: %w", err)
	}
	var highest int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(o.ordinal),0) FROM member_ordinal_v0 o
		JOIN member_facts_v0 m ON m.member_id=o.member_id AND m.build_id=$1`, buildID).Scan(&highest); err != nil {
		return err
	}
	if highest > math.MaxUint32 {
		return fmt.Errorf("ordinal %d exceeds 32-bit bitmap capacity", highest)
	}
	rows, err := tx.Query(ctx, `SELECT id,kind,category_code,definition_version,fixed_rule
		FROM segments_v0 WHERE is_visible AND definition_status='approved'
		AND kind IN ('option','predefined') ORDER BY id`)
	if err != nil {
		return err
	}
	type definition struct {
		id, kind string
		category *string
		version  int
		rule     json.RawMessage
	}
	definitions := []definition{}
	for rows.Next() {
		var d definition
		if err := rows.Scan(&d.id, &d.kind, &d.category, &d.version, &d.rule); err != nil {
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
		var query string
		args := []any{buildID, sourceDate}
		if d.kind == "predefined" {
			query = `SELECT DISTINCT o.ordinal FROM member_predefined_v0 mp
				JOIN member_ordinal_v0 o ON o.member_id=mp.member_id
				WHERE mp.build_id=$1 AND mp.segment_id=$3`
			args = append(args, d.id)
		} else if d.category != nil && (*d.category == "GENDER" || *d.category == "AGE_BAND" || *d.category == "RACE") {
			column := map[string]string{"GENDER": "gender_option_id", "AGE_BAND": "age_option_id", "RACE": "race_option_id"}[*d.category]
			query = `SELECT o.ordinal FROM member_facts_v0 m JOIN member_ordinal_v0 o ON o.member_id=m.member_id
				WHERE m.build_id=$1 AND m.` + column + `=$3`
			args = append(args, d.id)
		} else {
			if len(d.rule) == 0 {
				return fmt.Errorf("option %s has no bitmap rule", d.id)
			}
			next := 3
			condition, err := attributeRuleCondition(d.rule, &args, &next)
			if err != nil {
				return err
			}
			query = `SELECT o.ordinal FROM member_facts_v0 m JOIN member_ordinal_v0 o ON o.member_id=m.member_id
				WHERE m.build_id=$1 AND ` + condition
		}
		// The source date is a bound input even for predicates that do not
		// use it; this keeps one query shape for all option definitions.
		query += ` AND $2::date IS NOT NULL`
		if err := storeBitmap(ctx, tx, buildID, d.kind, d.id, d.version, query, args); err != nil {
			return err
		}
		var expected int64
		if err := tx.QueryRow(ctx, `SELECT raw_count FROM segment_build_count_v0
			WHERE build_id=$1 AND segment_id=$2 AND definition_version=$3 AND org_id='ALL'`,
			buildID, d.id, d.version).Scan(&expected); err != nil {
			return err
		}
		var actual int64
		if err := tx.QueryRow(ctx, `SELECT cardinality FROM member_bitmap_v0
			WHERE build_id=$1 AND predicate_kind=$2 AND predicate_id=$3`,
			buildID, d.kind, d.id).Scan(&actual); err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("bitmap %s has %d members, SQL has %d", d.id, actual, expected)
		}
	}
	if err := storeBitmap(ctx, tx, buildID, "consent", "yes", 1,
		`SELECT o.ordinal FROM member_facts_v0 m JOIN member_ordinal_v0 o ON o.member_id=m.member_id
		WHERE m.build_id=$1 AND m.build_consent`, []any{buildID}); err != nil {
		return err
	}
	for _, relation := range []struct{ table, column, kind string }{
		{"member_org_v0", "org_id", "org"},
		{"member_retail_v0", "retail_id", "retail"},
		{"member_product_v0", "product_id", "product"},
	} {
		ids, err := tx.Query(ctx, "SELECT DISTINCT "+relation.column+" FROM "+relation.table+" WHERE build_id=$1", buildID)
		if err != nil {
			return err
		}
		values := []string{}
		for ids.Next() {
			var value string
			if err := ids.Scan(&value); err != nil {
				ids.Close()
				return err
			}
			values = append(values, value)
		}
		err = ids.Err()
		ids.Close()
		if err != nil {
			return err
		}
		for _, value := range values {
			query := "SELECT DISTINCT o.ordinal FROM " + relation.table +
				" r JOIN member_ordinal_v0 o ON o.member_id=r.member_id WHERE r.build_id=$1 AND r." + relation.column + "=$2"
			if err := storeBitmap(ctx, tx, buildID, relation.kind, value, 1, query, []any{buildID, value}); err != nil {
				return err
			}
		}
	}
	return storeBitmap(ctx, tx, buildID, "activity", "active_30d", 1,
		`SELECT DISTINCT o.ordinal FROM member_retail_v0 r JOIN member_ordinal_v0 o ON o.member_id=r.member_id
		WHERE r.build_id=$1 AND r.last_txn_date BETWEEN $2::date-29 AND $2::date`,
		[]any{buildID, sourceDate})
}

func storeBitmap(ctx context.Context, tx pgx.Tx, buildID, kind, id string, version int,
	query string, args []any) error {
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bitmap %s/%s: %w", kind, id, err)
	}
	bitmap := roaring.New()
	for rows.Next() {
		var ordinal int64
		if err := rows.Scan(&ordinal); err != nil {
			rows.Close()
			return err
		}
		if ordinal < 1 || ordinal > math.MaxUint32 {
			rows.Close()
			return fmt.Errorf("bitmap ordinal %d is out of range", ordinal)
		}
		bitmap.Add(uint32(ordinal))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	bitmap.RunOptimize()
	payload, err := bitmap.MarshalBinary()
	if err != nil {
		return err
	}
	verified := roaring.New()
	if err := verified.UnmarshalBinary(payload); err != nil || !verified.Equals(bitmap) {
		return fmt.Errorf("bitmap %s/%s failed exact round-trip verification: %v", kind, id, err)
	}
	digest := sha256.Sum256(payload)
	_, err = tx.Exec(ctx, `INSERT INTO member_bitmap_v0
		(build_id,predicate_kind,predicate_id,definition_version,encoding_version,cardinality,checksum,payload)
		VALUES ($1,$2,$3,$4,1,$5,$6,$7)`, buildID, kind, id, version,
		bitmap.GetCardinality(), hex.EncodeToString(digest[:]), payload)
	return err
}
