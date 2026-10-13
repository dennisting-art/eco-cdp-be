package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/domain"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/platform/contextkeys"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/platform/database"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/platform/logger"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/ports"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/repository/sqlc"
)

const universeDBTimeout = 3 * time.Second

type universeRepository struct{ db *database.PostgresDB }

func NewUniverseRepository(db *database.PostgresDB) ports.UniverseRepository {
	return &universeRepository{db: db}
}

func (r *universeRepository) GetLatestCharts(ctx context.Context) (snap domain.UniverseSnapshot, err error) {
	ctx, cancel := context.WithTimeout(ctx, universeDBTimeout)
	defer cancel()
	tx, err := r.db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return snap, universeQueryError(ctx, "BeginUniverseRead", err)
	}
	defer tx.Rollback(ctx)
	q := sqlc.New(tx)
	snap.Scopes = []domain.UniverseScope{}
	snap.InvalidCharts = map[string]string{}
	snap.Summaries = map[string]domain.UniverseSummary{}
	snap.MatrixTotals = map[string]*int64{}
	snap.Acquisition = []domain.UniverseAcquisition{}
	snap.Buckets = []domain.UniverseBucket{}
	snap.Cells = []domain.UniverseCell{}

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
		snap.Scopes = append(snap.Scopes, domain.UniverseScope{Code: row.ScopeCode, Name: row.DisplayName, Enabled: row.IsAvailable})
	}

	summaries, err := q.ListUniverseLatestSummaries(ctx, groupCode)
	if err != nil {
		return snap, universeQueryError(ctx, "ListUniverseLatestSummaries", err)
	}
	for _, row := range summaries {
		snap.Summaries[row.ScopeCode] = domain.UniverseSummary{
			Date:         row.AsOfDate.Time,
			TotalMembers: nullableUniverseInt(row.TotalMemberCount), ActiveMembers: nullableUniverseInt(row.ActiveMemberCount),
			InactiveMembers: nullableUniverseInt(row.InactiveMemberCount), TotalRetail: nullableUniverseInt(row.TotalRetailCount),
			RetailMembers: universeInt(row.RetailMemberCount), RetailNonMembers: universeInt(row.RetailNonMemberCount),
		}
	}

	acquisition, err := q.ListUniverseLatestAcquisition(ctx, groupCode)
	if err != nil {
		return snap, universeQueryError(ctx, "ListUniverseLatestAcquisition", err)
	}
	for _, row := range acquisition {
		snap.AcquisitionDate = row.AsOfDate.Time
		snap.Acquisition = append(snap.Acquisition, domain.UniverseAcquisition{
			Year: int(row.AcquisitionYear), Members: row.MemberCount, NewMembers: row.NewMemberCount,
			Compare: nullableUniverseInt(row.CompareCount),
		})
	}

	buckets, err := q.ListUniverseEntityBuckets(ctx, groupCode)
	if err != nil {
		return snap, universeQueryError(ctx, "ListUniverseEntityBuckets", err)
	}
	for _, row := range buckets {
		if row.AsOfDate.Valid {
			snap.DistributionDate = row.AsOfDate.Time
		}
		ratio, err := nullableUniverseNumeric(row.Ratio)
		if err != nil {
			snap.InvalidCharts["entity_member_count"] = "invalid_bucket_ratio"
		}
		snap.Buckets = append(snap.Buckets, domain.UniverseBucket{
			Code: fmt.Sprint(row.BucketCode), Name: row.DisplayName, Count: nullableUniverseInt(row.CustomerCount), Ratio: ratio,
		})
	}

	matrixDate, err := q.GetUniverseLatestMatrixDate(ctx, groupCode)
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
			ratio, err := nullableUniverseNumeric(row.Ratio)
			if err != nil || ratio == nil || !row.RelatedEntityCode.Valid {
				snap.InvalidCharts["member_matrix_activity"] = "invalid_matrix_cell"
				continue
			}
			snap.Cells = append(snap.Cells, domain.UniverseCell{
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

func nullableUniverseInt(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func universeInt(v int64) *int64 { return &v }

func nullableUniverseNumeric(v pgtype.Numeric) (*float64, error) {
	if !v.Valid {
		return nil, nil
	}
	f, err := v.Float64Value()
	if err != nil {
		return nil, err
	}
	return &f.Float64, nil
}

func universeQueryError(ctx context.Context, query string, err error) error {
	category := "database"
	if errors.Is(err, context.DeadlineExceeded) {
		category = "timeout"
	}
	if errors.Is(err, context.Canceled) {
		category = "canceled"
	}
	traceID, _ := ctx.Value(contextkeys.TraceIdKey).(string)
	logger.Error("universe query failed", "service", "ecodai", "repository", "UniverseRepository", "query_name", query, "db_operation", "select", "category", category, "trace_id", traceID)
	return fmt.Errorf("%s: %w", query, err)
}
