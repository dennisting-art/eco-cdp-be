package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/postgres/sqlc"
)

const universeDBTimeout = 3 * time.Second

type UniverseRepository struct{ db *database.PostgresDB }

func NewUniverseRepository(db *database.PostgresDB) *UniverseRepository {
	return &UniverseRepository{db: db}
}

func (r *UniverseRepository) GetLatestCharts(ctx context.Context, requested *model.ReportingRange) (snap model.UniverseSnapshot, err error) {
	ctx, cancel := context.WithTimeout(ctx, universeDBTimeout)
	defer cancel()
	tx, err := r.db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return snap, universeQueryError(ctx, "BeginUniverseRead", err)
	}
	defer tx.Rollback(ctx)
	q := sqlc.New(tx)
	snap.Scopes = []model.UniverseScope{}
	snap.InvalidCharts = map[string]string{}
	snap.Summaries = map[string]model.UniverseSummary{}
	snap.MatrixTotals = map[string]*int64{}
	snap.Acquisition = []model.UniverseAcquisition{}
	snap.Buckets = []model.UniverseBucket{}
	snap.Cells = []model.UniverseCell{}

	groupCode, err := q.GetUniverseGroupScope(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return snap, nil
	}
	if err != nil {
		return snap, universeQueryError(ctx, "GetUniverseGroupScope", err)
	}
	snap.GroupConfigured = true
	snap.GroupCode = groupCode
	snap.MatrixEnabled, err = q.GetUniverseMatrixFeatureEnabled(ctx, groupCode)
	if err != nil {
		return snap, universeQueryError(ctx, "GetUniverseMatrixFeatureEnabled", err)
	}
	snap.EntityCountEnabled, err = q.GetUniverseEntityCountFeatureEnabled(ctx, groupCode)
	if err != nil {
		return snap, universeQueryError(ctx, "GetUniverseEntityCountFeatureEnabled", err)
	}

	scopes, err := q.ListUniverseOpcoScopes(ctx)
	if err != nil {
		return snap, universeQueryError(ctx, "ListUniverseOpcoScopes", err)
	}
	for _, row := range scopes {
		snap.Scopes = append(snap.Scopes, model.UniverseScope{Code: row.ScopeCode, Name: row.DisplayName, Enabled: row.IsAvailable})
	}
	if requested != nil {
		snap.Range = *requested
	} else {
		var latest pgtype.Date
		err = tx.QueryRow(ctx, `SELECT MAX(source_date) FROM (
			SELECT MAX(s.as_of_date) AS source_date FROM customer_daily_summary s
			WHERE s.scope_code=$1 OR s.scope_code IN (SELECT scope_code FROM ecosystem_scope WHERE scope_type='OPCO' AND is_available)
			UNION ALL SELECT MAX(as_of_date) FROM annual_member_acquisition WHERE scope_code=$1
			UNION ALL SELECT MAX(as_of_date) FROM member_daily_distribution WHERE scope_code=$1 AND feature_code='ENT_CNT' AND $2
			UNION ALL SELECT MAX(as_of_date) FROM member_entity_daily WHERE scope_code=$1 AND $3
		) dates`, groupCode, snap.EntityCountEnabled, snap.MatrixEnabled).Scan(&latest)
		if err != nil {
			return snap, universeQueryError(ctx, "GetUniverseReportingDate", err)
		}
		if latest.Valid {
			first := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
			if latest.Time.Before(first) {
				first = latest.Time
			}
			snap.Range = model.ReportingRange{Start: first, End: latest.Time}
		}
	}
	if snap.Range.End.IsZero() {
		if err := tx.Commit(ctx); err != nil {
			return snap, universeQueryError(ctx, "CommitUniverseRead", err)
		}
		return snap, nil
	}
	date := func(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

	summaries, err := q.ListUniverseLatestSummaries(ctx, sqlc.ListUniverseLatestSummariesParams{ScopeCode: groupCode, AsOfDate: date(snap.Range.Start), AsOfDate_2: date(snap.Range.End)})
	if err != nil {
		return snap, universeQueryError(ctx, "ListUniverseLatestSummaries", err)
	}
	for _, row := range summaries {
		snap.Summaries[row.ScopeCode] = model.UniverseSummary{
			Date:         row.AsOfDate.Time,
			TotalMembers: nullableInt64(row.TotalMemberCount), ActiveMembers: nullableInt64(row.ActiveMemberCount),
			InactiveMembers: nullableInt64(row.InactiveMemberCount), TotalRetail: nullableInt64(row.TotalRetailCount),
			RetailMembers: universeInt(row.RetailMemberCount), RetailNonMembers: universeInt(row.RetailNonMemberCount),
		}
	}

	acquisition, err := q.ListUniverseLatestAcquisition(ctx, sqlc.ListUniverseLatestAcquisitionParams{ScopeCode: groupCode, Column2: date(snap.Range.Start), Column3: date(snap.Range.End)})
	if err != nil {
		return snap, universeQueryError(ctx, "ListUniverseLatestAcquisition", err)
	}
	for _, row := range acquisition {
		snap.AcquisitionDate = row.AsOfDate.Time
		snap.Acquisition = append(snap.Acquisition, model.UniverseAcquisition{
			Year: int(row.AcquisitionYear), Members: row.MemberCount, NewMembers: row.NewMemberCount,
			Compare: nullableInt64(row.CompareCount),
		})
	}

	buckets, err := q.ListUniverseEntityBuckets(ctx, sqlc.ListUniverseEntityBucketsParams{ScopeCode: groupCode, AsOfDate: date(snap.Range.Start), AsOfDate_2: date(snap.Range.End)})
	if err != nil {
		return snap, universeQueryError(ctx, "ListUniverseEntityBuckets", err)
	}
	for _, row := range buckets {
		if row.AsOfDate.Valid {
			snap.DistributionDate = row.AsOfDate.Time
		}
		ratio, err := nullableFloat64(row.Ratio)
		if err != nil {
			snap.InvalidCharts["entity_member_count"] = "invalid_bucket_ratio"
		}
		snap.Buckets = append(snap.Buckets, model.UniverseBucket{
			Code: fmt.Sprint(row.BucketCode), Name: row.DisplayName, Count: nullableInt64(row.CustomerCount), Ratio: ratio,
		})
	}

	matrixDate, err := q.GetUniverseLatestMatrixDate(ctx, sqlc.GetUniverseLatestMatrixDateParams{ScopeCode: groupCode, AsOfDate: date(snap.Range.Start), AsOfDate_2: date(snap.Range.End)})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return snap, universeQueryError(ctx, "GetUniverseLatestMatrixDate", err)
	}
	if err == nil {
		snap.MatrixDate = matrixDate.Time
		cells, err := q.ListUniverseMatrixCells(ctx, sqlc.ListUniverseMatrixCellsParams{
			ScopeCode: groupCode, AsOfDate: matrixDate,
		})
		if err != nil {
			return snap, universeQueryError(ctx, "ListUniverseMatrixCells", err)
		}
		for _, row := range cells {
			ratio, err := nullableFloat64(row.Ratio)
			if err != nil || ratio == nil || !row.RelatedEntityCode.Valid {
				snap.InvalidCharts["member_matrix_activity"] = "invalid_matrix_cell"
				continue
			}
			snap.Cells = append(snap.Cells, model.UniverseCell{
				Primary: row.PrimaryEntityCode, Related: row.RelatedEntityCode.String,
				Count: row.MemberCount, Ratio: *ratio,
			})
		}
		totals, err := q.ListUniverseMatrixTotals(ctx, sqlc.ListUniverseMatrixTotalsParams{ScopeCode: groupCode, AsOfDate: matrixDate})
		if err != nil {
			return snap, universeQueryError(ctx, "ListUniverseMatrixTotals", err)
		}
		for _, row := range totals {
			count := row.MemberCount
			snap.MatrixTotals[row.PrimaryEntityCode] = &count
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return snap, universeQueryError(ctx, "CommitUniverseRead", err)
	}
	return snap, nil
}

func universeInt(v int64) *int64 { return &v }

func universeQueryError(ctx context.Context, query string, err error) error {
	return queryError(ctx, "universe query failed", "UniverseRepository", query, err, "db_operation", "select")
}
