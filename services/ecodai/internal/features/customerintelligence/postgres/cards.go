package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/postgres/sqlc"
)

const cardsDBTimeout = 3 * time.Second

type cardsActivityQueryRow struct {
	Date     time.Time `db:"as_of_date"`
	Active   int64     `db:"active_member_count"`
	Inactive int64     `db:"inactive_member_count"`
}

type cardsPenetrationQueryRow struct {
	Month     time.Time `db:"report_month"`
	Date      time.Time `db:"as_of_date"`
	Days      int64     `db:"day_count"`
	Member    int64     `db:"member_net_sales"`
	NonMember int64     `db:"non_member_net_sales"`
}

type CardsRepository struct{ db *database.PostgresDB }

func NewCardsRepository(db *database.PostgresDB) *CardsRepository {
	return &CardsRepository{db: db}
}

func (r *CardsRepository) GetSnapshot(ctx context.Context, scopeCode string, requested *model.ReportingRange) (snap model.CardsSnapshot, err error) {
	ctx, cancel := context.WithTimeout(ctx, cardsDBTimeout)
	defer cancel()
	tx, err := r.db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return snap, cardsQueryError(ctx, "BeginCardsRead", err)
	}
	defer tx.Rollback(ctx)
	q := sqlc.New(tx)
	snap.Features = []model.CardsFeature{}
	snap.Activity = []model.CardsActivityRow{}
	snap.Penetration = []model.CardsPenetrationRow{}
	snap.Distributions = []model.CardsDistributionRow{}

	scope, err := q.GetCardsScope(ctx, scopeCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return snap, nil
	}
	if err != nil {
		return snap, cardsQueryError(ctx, "GetCardsScope", err)
	}
	snap.ScopeFound, snap.ScopeAvailable = true, scope.IsAvailable
	if !scope.IsAvailable {
		return snap, nil
	}
	features, err := q.ListCardsFeatures(ctx, scopeCode)
	if err != nil {
		return snap, cardsQueryError(ctx, "ListCardsFeatures", err)
	}
	needActivity, needPenetration, needDistribution := false, false, false
	for _, row := range features {
		snap.Features = append(snap.Features, model.CardsFeature{
			Code: row.FeatureCode, Name: row.DisplayName, Type: row.FeatureType, Enabled: row.IsEnabled,
		})
		if row.IsEnabled {
			switch row.FeatureCode {
			case "ACTIVE_OPCO":
				needActivity = true
			case "PENETRATION_OPCO":
				needPenetration = true
			default:
				if row.FeatureType == "DISTRIBUTION" {
					needDistribution = true
				}
			}
		}
	}
	if requested != nil {
		snap.Range = *requested
	} else {
		var latest pgtype.Date
		err = tx.QueryRow(ctx, `SELECT MAX(source_date) FROM (
			SELECT MAX(as_of_date) AS source_date FROM member_daily_summary WHERE scope_code=$1 AND $2
			UNION ALL SELECT MAX(as_of_date) FROM transaction_daily_summary WHERE scope_code=$1 AND $3
			UNION ALL SELECT MAX(d.as_of_date) FROM member_daily_distribution d
			JOIN ecosystem_scope_feature esf ON esf.scope_code=d.scope_code AND esf.feature_code=d.feature_code AND esf.is_enabled
			JOIN feature_summary f ON f.feature_code=d.feature_code AND f.feature_context='CARDS' AND f.feature_type='DISTRIBUTION'
			WHERE d.scope_code=$1 AND $4
		) dates`, scopeCode, needActivity, needPenetration, needDistribution).Scan(&latest)
		if err != nil {
			return snap, cardsQueryError(ctx, "GetCardsReportingDate", err)
		}
		if latest.Valid {
			end := latest.Time
			first := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -5, 0)
			snap.Range = model.ReportingRange{Start: first, End: end}
		}
	}
	if snap.Range.End.IsZero() {
		if err := tx.Commit(ctx); err != nil {
			return snap, cardsQueryError(ctx, "CommitCardsRead", err)
		}
		return snap, nil
	}
	date := func(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }
	if needActivity {
		previousYearMonth := time.Date(snap.Range.Start.Year()-1, snap.Range.Start.Month(), 1, 0, 0, 0, 0, time.UTC)
		rows, err := tx.Query(ctx, `SELECT DISTINCT ON (date_trunc('month', as_of_date))
			as_of_date, active_member_count, inactive_member_count
			FROM member_daily_summary WHERE scope_code=$1 AND as_of_date BETWEEN $2::date AND $3::date
			ORDER BY date_trunc('month', as_of_date), as_of_date DESC`, scopeCode, previousYearMonth, snap.Range.End)
		if err != nil {
			return snap, cardsQueryError(ctx, "ListCardsActivity", err)
		}
		activity, err := pgx.CollectRows(rows, pgx.RowToStructByName[cardsActivityQueryRow])
		if err != nil {
			return snap, cardsQueryError(ctx, "ListCardsActivity", err)
		}
		for _, row := range activity {
			month := time.Date(row.Date.Year(), row.Date.Month(), 1, 0, 0, 0, 0, time.UTC)
			snap.Activity = append(snap.Activity, model.CardsActivityRow{
				Month: month, Date: row.Date, Active: row.Active, Inactive: row.Inactive,
			})
		}
	}
	if needPenetration {
		rows, err := tx.Query(ctx, `SELECT date_trunc('month', as_of_date)::date AS report_month, MAX(as_of_date) AS as_of_date, COUNT(*)::bigint AS day_count,
			SUM(member_net_sales)::bigint AS member_net_sales, SUM(non_member_net_sales)::bigint AS non_member_net_sales
			FROM transaction_daily_summary WHERE scope_code=$1 AND as_of_date BETWEEN $2::date AND $3::date
			GROUP BY 1 ORDER BY 1`, scopeCode, snap.Range.Start, snap.Range.End)
		if err != nil {
			return snap, cardsQueryError(ctx, "ListCardsPenetration", err)
		}
		penetration, err := pgx.CollectRows(rows, pgx.RowToStructByName[cardsPenetrationQueryRow])
		if err != nil {
			return snap, cardsQueryError(ctx, "ListCardsPenetration", err)
		}
		for _, row := range penetration {
			first, last := row.Month, row.Month.AddDate(0, 1, 0).AddDate(0, 0, -1)
			if snap.Range.Start.After(first) {
				first = snap.Range.Start
			}
			if snap.Range.End.Before(last) {
				last = snap.Range.End
			}
			complete := row.Days == int64(last.Sub(first).Hours()/24)+1
			total := row.Member + row.NonMember
			percent := 0.0
			if total > 0 {
				percent = math.Round(float64(row.Member)*10000/float64(total)) / 100
			}
			snap.Penetration = append(snap.Penetration, model.CardsPenetrationRow{
				Month: row.Month, Date: row.Date, Complete: complete, TotalSales: total, Percent100: percent,
			})
		}
	}
	if needDistribution {
		rows, err := q.ListCardsDistributionBuckets(ctx, sqlc.ListCardsDistributionBucketsParams{
			ScopeCode: scopeCode, AsOfDate: date(snap.Range.Start), AsOfDate_2: date(snap.Range.End),
		})
		if err != nil {
			return snap, cardsQueryError(ctx, "ListCardsDistributionBuckets", err)
		}
		for _, row := range rows {
			ratio, err := nullableFloat64(row.Ratio)
			if err != nil {
				return snap, cardsQueryError(ctx, "ListCardsDistributionBuckets", err)
			}
			var dateValue *time.Time
			if row.AsOfDate.Valid {
				value := row.AsOfDate.Time
				dateValue = &value
			}
			rollupGroup := ""
			if row.RollupGroup.Valid {
				rollupGroup = row.RollupGroup.String
			}
			snap.Distributions = append(snap.Distributions, model.CardsDistributionRow{
				FeatureCode: row.FeatureCode, BucketCode: fmt.Sprint(row.BucketCode), Name: row.DisplayName,
				IsRollup: row.IsRollup, RollupGroup: rollupGroup,
				Date: dateValue, Count: nullableInt64(row.CustomerCount), Ratio: ratio,
			})
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return snap, cardsQueryError(ctx, "CommitCardsRead", err)
	}
	return snap, nil
}

func cardsQueryError(ctx context.Context, query string, err error) error {
	return queryError(ctx, "cards query failed", "CardsRepository", query, err)
}
