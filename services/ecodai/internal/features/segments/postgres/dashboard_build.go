package postgres

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type retailScope struct {
	key string
	ids []string
}

func buildDashboard(ctx context.Context, tx pgx.Tx, buildID string, sourceDate time.Time) error {
	rows, err := tx.Query(ctx, `SELECT DISTINCT retail_id FROM member_retail_v0
		WHERE build_id=$1 ORDER BY retail_id`, buildID)
	if err != nil {
		return err
	}
	retails := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		retails = append(retails, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	scopes := dashboardScopes(retails)
	var allCount int64
	for _, scope := range scopes {
		var count, active int64
		err := tx.QueryRow(ctx, `SELECT COUNT(DISTINCT m.member_id),
			COUNT(DISTINCT m.member_id) FILTER
			(WHERE r.last_txn_date BETWEEN $3::date-29 AND $3::date)
			FROM member_retail_v0 r JOIN member_facts_v0 m
			ON m.build_id=r.build_id AND m.member_id=r.member_id
			WHERE r.build_id=$1 AND r.retail_id=ANY($2::text[]) AND m.build_consent`,
			buildID, scope.ids, sourceDate).Scan(&count, &active)
		if err != nil {
			return err
		}
		if scope.key == "ALL" {
			allCount = count
		}
		for metric, value := range map[string]int64{"member_count": count, "active_30d_count": active} {
			if _, err := tx.Exec(ctx, `INSERT INTO dashboard_result_v0
				(build_id,org_id,retail_scope,metric_id,dimension_key,value)
				VALUES ($1,'ALL',$2,$3,'ALL',$4)`, buildID, scope.key, metric, value); err != nil {
				return err
			}
		}
		cells, err := tx.Query(ctx, `SELECT COALESCE(m.gender_option_id,'missing'),
			COALESCE(m.age_option_id,'missing'),COUNT(DISTINCT m.member_id)
			FROM member_retail_v0 r JOIN member_facts_v0 m
			ON m.build_id=r.build_id AND m.member_id=r.member_id
			WHERE r.build_id=$1 AND r.retail_id=ANY($2::text[]) AND m.build_consent
			GROUP BY COALESCE(m.gender_option_id,'missing'),COALESCE(m.age_option_id,'missing')`,
			buildID, scope.ids)
		if err != nil {
			return err
		}
		var cellTotal int64
		type cell struct {
			key   string
			count int64
		}
		pending := []cell{}
		for cells.Next() {
			var gender, age string
			var cellCount int64
			if err := cells.Scan(&gender, &age, &cellCount); err != nil {
				cells.Close()
				return err
			}
			cellTotal += cellCount
			key := "gender=" + gender + "|age=" + age
			pending = append(pending, cell{key: key, count: cellCount})
		}
		err = cells.Err()
		cells.Close()
		if err != nil {
			return err
		}
		for _, cell := range pending {
			if _, err := tx.Exec(ctx, `INSERT INTO dashboard_result_v0
				(build_id,org_id,retail_scope,metric_id,dimension_key,value)
				VALUES ($1,'ALL',$2,'member_count',$3,$4)`, buildID, scope.key, cell.key, cell.count); err != nil {
				return err
			}
		}
		if cellTotal != count {
			return fmt.Errorf("dashboard scope %s cells total %d, expected %d", scope.key, cellTotal, count)
		}
	}
	month := time.Date(sourceDate.Year(), sourceDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	var revision int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(revision),0)+1 FROM customer_monthly_summary_v0
		WHERE month=$1 AND org_id='ALL' AND retail_scope='ALL' AND metric_id='member_count'`, month).Scan(&revision); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE customer_monthly_summary_v0 SET is_current=FALSE
		WHERE month=$1 AND org_id='ALL' AND retail_scope='ALL' AND metric_id='member_count'
		AND is_current`, month); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO customer_monthly_summary_v0
		(month,org_id,retail_scope,metric_id,revision,source_build_id,value,is_current)
		VALUES ($1,'ALL','ALL','member_count',$2,$3,$4,TRUE)`, month, revision, buildID, allCount)
	return err
}

func dashboardScopes(retails []string) []retailScope {
	retails = append([]string(nil), retails...)
	sort.Strings(retails)
	scopes := []retailScope{{key: "ALL", ids: retails}}
	if len(retails) <= 8 {
		for mask := 1; mask < (1<<len(retails))-1; mask++ {
			ids := []string{}
			for i, id := range retails {
				if mask&(1<<i) != 0 {
					ids = append(ids, id)
				}
			}
			scopes = append(scopes, retailScope{key: strings.Join(ids, "+"), ids: ids})
		}
	} else {
		for _, id := range retails {
			scopes = append(scopes, retailScope{key: id, ids: []string{id}})
		}
	}
	return scopes
}
