package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/domain"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/ports"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/repository/sqlc"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/platform/contextkeys"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/platform/database"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/platform/logger"
)

const cardsDBTimeout = 3 * time.Second

type cardsRepository struct{ db *database.PostgresDB }

func NewCardsRepository(db *database.PostgresDB) ports.CardsRepository {
	return &cardsRepository{db: db}
}

func (r *cardsRepository) GetSnapshot(ctx context.Context, scopeCode string, firstMonth, lastMonth time.Time) (snap domain.CardsSnapshot, err error) {
	ctx, cancel := context.WithTimeout(ctx, cardsDBTimeout)
	defer cancel()
	tx, err := r.db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return snap, cardsQueryError(ctx, "BeginCardsRead", err)
	}
	defer tx.Rollback(ctx)
	q := sqlc.New(tx)
	snap.Features = []domain.CardsFeature{}
	snap.Activity = []domain.CardsActivityRow{}
	snap.Penetration = []domain.CardsPenetrationRow{}
	snap.Distributions = []domain.CardsDistributionRow{}

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
		kind := ""
		if row.FeatureType.Valid {
			kind = row.FeatureType.String
		}
		snap.Features = append(snap.Features, domain.CardsFeature{
			Code: row.FeatureCode, Name: row.DisplayName, Type: kind, Enabled: row.IsEnabled,
		})
		if row.IsEnabled {
			switch row.FeatureCode {
			case "ACTIVE_OPCO":
				needActivity = true
			case "PENETRATION_OPCO":
				needPenetration = true
			default:
				if kind == "DISTRIBUTION" {
					needDistribution = true
				}
			}
		}
	}
	date := func(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }
	if needActivity {
		rows, err := q.ListCardsMonthlyActivity(ctx, sqlc.ListCardsMonthlyActivityParams{
			ScopeCode: scopeCode, ReportMonth: date(firstMonth), ReportMonth_2: date(lastMonth),
			ReportMonth_3: date(firstMonth.AddDate(-1, 0, 0)), ReportMonth_4: date(lastMonth.AddDate(-1, 0, 0)),
		})
		if err != nil {
			return snap, cardsQueryError(ctx, "ListCardsMonthlyActivity", err)
		}
		for _, row := range rows {
			snap.Activity = append(snap.Activity, domain.CardsActivityRow{
				Month: row.ReportMonth.Time, Active: row.ActiveMemberCount, Inactive: row.InactiveMemberCount,
			})
		}
	}
	if needPenetration {
		rows, err := q.ListCardsMonthlyPenetration(ctx, sqlc.ListCardsMonthlyPenetrationParams{
			ScopeCode: scopeCode, ReportMonth: date(firstMonth), ReportMonth_2: date(lastMonth),
		})
		if err != nil {
			return snap, cardsQueryError(ctx, "ListCardsMonthlyPenetration", err)
		}
		for _, row := range rows {
			snap.Penetration = append(snap.Penetration, domain.CardsPenetrationRow{
				Month: row.ReportMonth.Time, TotalSales: row.TotalNetSales, Percent100: row.MemberPenetrationPercentage,
			})
		}
	}
	if needDistribution {
		rows, err := q.ListCardsDistributionBuckets(ctx, scopeCode)
		if err != nil {
			return snap, cardsQueryError(ctx, "ListCardsDistributionBuckets", err)
		}
		for _, row := range rows {
			ratio, err := nullableUniverseNumeric(row.Ratio)
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
			snap.Distributions = append(snap.Distributions, domain.CardsDistributionRow{
				FeatureCode: row.FeatureCode, BucketCode: fmt.Sprint(row.BucketCode), Name: row.DisplayName,
				IsRollup: row.IsRollup, RollupGroup: rollupGroup,
				Date: dateValue, Count: nullableUniverseInt(row.CustomerCount), Ratio: ratio,
			})
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return snap, cardsQueryError(ctx, "CommitCardsRead", err)
	}
	return snap, nil
}

func cardsQueryError(ctx context.Context, query string, err error) error {
	category := "database"
	if errors.Is(err, context.DeadlineExceeded) {
		category = "timeout"
	} else if errors.Is(err, context.Canceled) {
		category = "canceled"
	}
	traceID, _ := ctx.Value(contextkeys.TraceIdKey).(string)
	logger.Error("cards query failed", "service", "eco-cdp-be", "repository", "CardsRepository", "query_name", query, "category", category, "trace_id", traceID)
	return fmt.Errorf("%s: %w", query, err)
}
