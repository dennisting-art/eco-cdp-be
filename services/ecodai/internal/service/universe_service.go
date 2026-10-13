package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/domain"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/platform/contextkeys"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/platform/logger"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/platform/permission"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/ports"
)

const UniversePermission = CustomerIntelligenceViewPermission

type UniverseService struct{ repo ports.UniverseRepository }

func NewUniverseService(repo ports.UniverseRepository) *UniverseService {
	return &UniverseService{repo: repo}
}

func (s *UniverseService) Query(ctx context.Context, req domain.UniverseQuery) (domain.UniverseView, error) {
	var out domain.UniverseView
	allowed := permission.Allows(req.Permissions, UniversePermission)
	if !allowed {
		return out, domain.ErrUniverseForbidden
	}
	if strings.TrimSpace(req.StartDate) != "" || strings.TrimSpace(req.EndDate) != "" {
		return out, domain.ErrUniverseDateUnsupported
	}
	snap, err := s.repo.GetLatestCharts(ctx)
	if err != nil {
		return out, fmt.Errorf("%w: %w", domain.ErrUniverseDatabase, err)
	}
	return buildUniverseContext(ctx, snap)
}

// buildUniverse is also used by snapshot-only tests.
func buildUniverse(s domain.UniverseSnapshot) (domain.UniverseView, error) {
	return buildUniverseContext(context.Background(), s)
}

type universeChartIssue struct {
	invalid bool
	reason  string
	details []any
}

func missingChart(reason string, details ...any) *universeChartIssue {
	return &universeChartIssue{reason: reason, details: details}
}
func invalidChart(reason string, details ...any) *universeChartIssue {
	return &universeChartIssue{invalid: true, reason: reason, details: details}
}

func buildUniverseContext(ctx context.Context, s domain.UniverseSnapshot) (domain.UniverseView, error) {
	out := domain.UniverseView{
		Status: "complete", Warnings: []domain.UniverseWarning{},
		AsOfDates: domain.UniverseViewDates{Group: map[string]*time.Time{}, Opcos: map[string]*time.Time{}},
		Scopes:    []domain.UniverseViewScope{}, Opcos: []domain.UniverseViewOpco{},
	}
	if !s.GroupConfigured || s.GroupCode == "" {
		return domain.UniverseView{}, domain.ErrUniverseInvalidData
	}
	seen := map[string]bool{}
	for _, scope := range s.Scopes {
		if scope.Code == "" || scope.Code == s.GroupCode || seen[scope.Code] {
			return domain.UniverseView{}, domain.ErrUniverseInvalidData
		}
		seen[scope.Code] = true
		out.Scopes = append(out.Scopes, domain.UniverseViewScope{Code: scope.Code, Name: scope.Name})
	}
	successful, hasInvalid := 0, false
	record := func(chart, scope string, date time.Time, issue *universeChartIssue) {
		if scope == "" {
			out.AsOfDates.Group[chart] = nil
		} else {
			out.AsOfDates.Opcos[scope] = nil
		}
		if issue != nil {
			code := "CHART_DATA_UNAVAILABLE"
			if issue.invalid {
				code = "CHART_DATA_INVALID"
				hasInvalid = true
			}
			message := "Chart data is temporarily unavailable."
			if chart == "member_matrix_activity" {
				message = "Member activity matrix is temporarily unavailable."
			}
			out.Warnings = append(out.Warnings, domain.UniverseWarning{Chart: chart, ScopeCode: scope, Code: code, Message: message})
			fields := []any{"chart", chart, "group", s.GroupCode, "scope", scope, "selected_date", date, "reason", issue.reason, "code", code, "trace_id", ctx.Value(contextkeys.TraceIdKey)}
			fields = append(fields, issue.details...)
			logger.Error("universe chart unavailable", fields...)
			return
		}
		successful++
		if scope == "" {
			out.AsOfDates.Group[chart] = &date
		} else {
			out.AsOfDates.Opcos[scope] = &date
		}
		if date.After(out.AsOfDate) {
			out.AsOfDate = date
		}
	}
	group := s.Summaries[s.GroupCode]
	totalIssue := universeCounts(group.Date, group.TotalMembers)
	if totalIssue == nil {
		out.Group.TotalMembers = &domain.UniverseViewMetric{Count: group.TotalMembers}
	}
	record("total_members", "", group.Date, totalIssue)
	activityIssue := universeCounts(group.Date, group.TotalMembers, group.ActiveMembers, group.InactiveMembers)
	if activityIssue == nil && (*group.ActiveMembers > *group.TotalMembers || *group.InactiveMembers > *group.TotalMembers) {
		activityIssue = invalidChart("member_count_exceeds_total")
	}
	if activityIssue == nil {
		out.Group.MemberActivity = &domain.UniverseViewActivity{
			Active:   domain.UniverseViewMetric{Count: group.ActiveMembers, Percent: universeRatio(group.ActiveMembers, group.TotalMembers)},
			Inactive: domain.UniverseViewMetric{Count: group.InactiveMembers, Percent: universeRatio(group.InactiveMembers, group.TotalMembers)},
		}
	}
	record("member_activity", "", group.Date, activityIssue)
	for _, metric := range []struct {
		name   string
		count  *int64
		target **domain.UniverseViewMetric
	}{
		{"member_penetration_retail", group.RetailMembers, &out.Group.MemberPenetrationRetail},
		{"non_member_transaction_retail", group.RetailNonMembers, &out.Group.NonMemberTransactionRetail},
	} {
		issue := universeRetail(group.Date, group.TotalRetail, metric.count)
		if issue == nil {
			*metric.target = &domain.UniverseViewMetric{Count: metric.count, Percent: universeRatio(metric.count, group.TotalRetail)}
		}
		record(metric.name, "", group.Date, issue)
	}
	acquisition, growth, acquisitionIssue := buildUniverseAcquisition(s)
	if acquisitionIssue == nil {
		out.Group.AnnualMemberAcquisition, out.Group.LatestMemberGrowth = acquisition, growth
	}
	record("annual_member_acquisition", "", s.AcquisitionDate, acquisitionIssue)
	record("latest_member_growth", "", s.AcquisitionDate, acquisitionIssue)
	buckets, bucketIssue := buildUniverseBuckets(s)
	if bucketIssue == nil {
		out.Group.EntityMemberCount = buckets
	}
	record("entity_member_count", "", s.DistributionDate, bucketIssue)
	for _, scope := range s.Scopes {
		card := domain.UniverseViewOpco{ScopeCode: scope.Code, IsEnabled: scope.Enabled}
		if !scope.Enabled {
			// Preserve the pre-existing disabled-card placeholders.
			card.Member, card.NonMember = &domain.UniverseViewMetric{}, &domain.UniverseViewMetric{}
			out.AsOfDates.Opcos[scope.Code] = nil
		} else {
			summary := s.Summaries[scope.Code]
			issue := universeRetail(summary.Date, summary.TotalRetail, summary.RetailMembers, summary.RetailNonMembers)
			if issue == nil {
				card.TotalRetailCount = summary.TotalRetail
				card.Member = &domain.UniverseViewMetric{Count: summary.RetailMembers, Percent: universeRatio(summary.RetailMembers, summary.TotalRetail)}
				card.NonMember = &domain.UniverseViewMetric{Count: summary.RetailNonMembers, Percent: universeRatio(summary.RetailNonMembers, summary.TotalRetail)}
			}
			record("opco_retail", scope.Code, summary.Date, issue)
		}
		out.Opcos = append(out.Opcos, card)
	}
	if s.MatrixEnabled {
		matrix, matrixIssue := buildUniverseMatrix(s)
		if matrixIssue == nil {
			out.Group.MemberMatrixActivity = matrix
		}
		record("member_matrix_activity", "", s.MatrixDate, matrixIssue)
	} else {
		out.AsOfDates.Group["member_matrix_activity"] = nil
	}
	if successful == 0 {
		if hasInvalid {
			return domain.UniverseView{}, domain.ErrUniverseInvalidData
		}
		return domain.UniverseView{}, domain.ErrUniverseUnavailable
	}
	if len(out.Warnings) > 0 {
		out.Status = "partial"
		logger.Warn("universe partial response", "warning_count", len(out.Warnings), "group", s.GroupCode, "trace_id", ctx.Value(contextkeys.TraceIdKey))
	}
	return out, nil
}

func universeCounts(date time.Time, counts ...*int64) *universeChartIssue {
	// Known invalid data takes precedence over missing data.
	for _, count := range counts {
		if count != nil && *count < 0 {
			return invalidChart("negative_count")
		}
	}
	if date.IsZero() {
		return missingChart("missing_snapshot_date")
	}
	for _, count := range counts {
		if count == nil {
			return missingChart("missing_count")
		}
	}
	return nil
}

func universeRetail(date time.Time, total *int64, counts ...*int64) *universeChartIssue {
	if issue := universeCounts(date, append([]*int64{total}, counts...)...); issue != nil {
		return issue
	}
	for _, count := range counts {
		if *count > *total {
			return invalidChart("retail_count_exceeds_total")
		}
	}
	return nil
}

func buildUniverseAcquisition(s domain.UniverseSnapshot) ([]domain.UniverseViewMetric, *domain.UniverseViewMetric, *universeChartIssue) {
	if s.AcquisitionDate.IsZero() || len(s.Acquisition) == 0 {
		return nil, nil, missingChart("missing_acquisition")
	}
	rows := append([]domain.UniverseAcquisition(nil), s.Acquisition...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Year < rows[j].Year })
	out := make([]domain.UniverseViewMetric, 0, len(rows))
	for i, a := range rows {
		if a.NewMembers < 0 || a.Members < 0 || a.Year < 1900 || a.Year > 9999 || (i > 0 && rows[i-1].Year == a.Year) {
			return nil, nil, invalidChart("invalid_acquisition")
		}
		count := a.NewMembers
		out = append(out, domain.UniverseViewMetric{Label: strconv.Itoa(a.Year), Count: &count, Compare: a.Compare})
	}
	current := rows[len(rows)-1]
	growth := &domain.UniverseViewMetric{Count: &current.NewMembers}
	if len(rows) > 1 {
		previous := rows[len(rows)-2]
		if previous.Year == current.Year-1 && previous.NewMembers > 0 && current.Compare != nil {
			ratio := float64(*current.Compare) / float64(previous.NewMembers)
			growth.Growth = &ratio
		}
	}
	return out, growth, nil
}

func buildUniverseBuckets(s domain.UniverseSnapshot) ([]domain.UniverseViewMetric, *universeChartIssue) {
	if reason := s.InvalidCharts["entity_member_count"]; reason != "" {
		return nil, invalidChart(reason)
	}
	if s.DistributionDate.IsZero() || len(s.Buckets) == 0 {
		return nil, missingChart("missing_distribution")
	}
	out := make([]domain.UniverseViewMetric, 0, len(s.Buckets))
	seen := map[string]bool{}
	var issue *universeChartIssue
	for _, b := range s.Buckets {
		if seen[b.Code] || b.Code == "" {
			return nil, invalidChart("invalid_bucket_configuration")
		}
		seen[b.Code] = true
		if b.Count != nil && *b.Count < 0 || b.Ratio != nil && !validUniverseRatio(*b.Ratio) {
			return nil, invalidChart("invalid_bucket")
		}
		if b.Count == nil || b.Ratio == nil {
			issue = missingChart("missing_bucket")
		}
		out = append(out, domain.UniverseViewMetric{Label: b.Name, Count: b.Count, Percent: b.Ratio})
	}
	if issue != nil {
		return nil, issue
	}
	return out, nil
}

func buildUniverseMatrix(s domain.UniverseSnapshot) (*domain.UniverseViewMatrix, *universeChartIssue) {
	if reason := s.InvalidCharts["member_matrix_activity"]; reason != "" {
		return nil, invalidChart(reason)
	}
	if s.MatrixDate.IsZero() {
		return nil, missingChart("missing_matrix_date")
	}
	configured := map[string]bool{}
	for _, scope := range s.Scopes {
		configured[scope.Code] = true
	}
	participants := map[string]bool{}
	for code := range s.MatrixTotals {
		participants[code] = true
	}
	cells := map[string]domain.UniverseCell{}
	for _, cell := range s.Cells {
		participants[cell.Primary], participants[cell.Related] = true, true
		if cell.Count < 0 || !validUniverseRatio(cell.Ratio) {
			return nil, invalidChart("invalid_matrix_cell")
		}
		key := cell.Primary + "\x00" + cell.Related
		if _, exists := cells[key]; exists {
			return nil, invalidChart("duplicate_matrix_cell")
		}
		cells[key] = cell
	}
	if len(participants) == 0 {
		return nil, missingChart("missing_matrix_entities")
	}
	for code := range participants {
		if !configured[code] {
			return nil, invalidChart("unknown_matrix_entity", "entity", code)
		}
	}
	ordered := make([]string, 0, len(participants))
	for _, scope := range s.Scopes {
		if participants[scope.Code] {
			ordered = append(ordered, scope.Code)
		}
	}
	for _, primary := range ordered {
		if total := s.MatrixTotals[primary]; total != nil && *total < 0 {
			return nil, invalidChart("negative_primary_total", "primary_entity", primary)
		}
	}
	if len(cells) != len(ordered)*len(ordered) {
		return nil, missingChart("missing_relationship_cells", "expected_cells", len(ordered)*len(ordered), "actual_cells", len(cells))
	}
	out := &domain.UniverseViewMatrix{Columns: append([]string{}, ordered...), Rows: []domain.UniverseViewMatrixRow{}}
	for _, primary := range ordered {
		total := s.MatrixTotals[primary]
		if total == nil {
			return nil, missingChart("missing_primary_total", "primary_entity", primary)
		}
		row := domain.UniverseViewMatrixRow{PrimaryEntity: domain.UniverseViewMetric{Label: primary, Count: total}, Relationships: []domain.UniverseViewMetric{}}
		for _, related := range ordered {
			cell := cells[primary+"\x00"+related]
			count, ratio := cell.Count, cell.Ratio
			row.Relationships = append(row.Relationships, domain.UniverseViewMetric{Label: related, Count: &count, Percent: &ratio})
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

func universeRatio(numerator, denominator *int64) *float64 {
	if numerator == nil || denominator == nil || *denominator == 0 {
		return nil
	}
	v := math.Round(float64(*numerator)/float64(*denominator)*1e6) / 1e6
	return &v
}
func validUniverseRatio(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1
}
