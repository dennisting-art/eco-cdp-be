package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/domain"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/platform/contextkeys"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/platform/logger"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/platform/permission"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/ports"
)

const defaultCardsScope = "AEON"

var cardsReportingZone = time.FixedZone("MYT", 8*60*60)

type CardsService struct {
	repo ports.CardsRepository
	now  func() time.Time
}

func NewCardsService(repo ports.CardsRepository) *CardsService {
	return &CardsService{repo: repo, now: time.Now}
}

func (s *CardsService) Query(ctx context.Context, req domain.CardsQuery) (domain.CardsView, error) {
	var out domain.CardsView
	allowed := permission.Allows(req.Permissions, CustomerIntelligenceViewPermission)
	if !allowed {
		return out, domain.ErrCardsForbidden
	}
	scopeCode := defaultCardsScope
	if req.ScopeCode != nil && *req.ScopeCode != "" {
		scopeCode = *req.ScopeCode
		if scopeCode != strings.TrimSpace(scopeCode) || len(scopeCode) > 30 {
			return out, domain.ErrCardsInvalidScope
		}
	}
	now := s.now().In(cardsReportingZone)
	lastMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstMonth := lastMonth.AddDate(0, -5, 0)
	snap, err := s.repo.GetSnapshot(ctx, scopeCode, firstMonth, lastMonth)
	if err != nil {
		return out, fmt.Errorf("%w: %w", domain.ErrCardsDatabase, err)
	}
	if !snap.ScopeFound || !snap.ScopeAvailable {
		return out, domain.ErrCardsScopeMissing
	}
	return buildCardsContext(ctx, scopeCode, firstMonth, lastMonth, snap)
}

func buildCards(scopeCode string, firstMonth, lastMonth time.Time, snap domain.CardsSnapshot) (domain.CardsView, error) {
	return buildCardsContext(context.Background(), scopeCode, firstMonth, lastMonth, snap)
}

type cardsFeatureIssue struct {
	invalid bool
	reason  string
	details []any
}

func missingCardsFeature(reason string, details ...any) *cardsFeatureIssue {
	return &cardsFeatureIssue{reason: reason, details: details}
}

func invalidCardsFeature(reason string, details ...any) *cardsFeatureIssue {
	return &cardsFeatureIssue{invalid: true, reason: reason, details: details}
}

func buildCardsContext(ctx context.Context, scopeCode string, firstMonth, lastMonth time.Time, snap domain.CardsSnapshot) (domain.CardsView, error) {
	out := domain.CardsView{
		Status: "complete", Warnings: []domain.CardsWarning{}, ScopeCode: scopeCode,
		Features: []domain.CardsViewFeature{},
	}
	invalidConfig := func(reason string) (domain.CardsView, error) {
		logger.Error("cards reporting data invalid", "feature", "feature_config", "scope_code", scopeCode,
			"reason", reason, "trace_id", ctx.Value(contextkeys.TraceIdKey))
		return domain.CardsView{}, domain.ErrCardsInvalidData
	}
	if len(snap.Features) == 0 {
		return invalidConfig("missing_feature_config")
	}

	distributions := map[string][]domain.CardsDistributionRow{}
	for _, row := range snap.Distributions {
		distributions[row.FeatureCode] = append(distributions[row.FeatureCode], row)
	}

	seen := map[string]bool{}
	for _, feature := range snap.Features {
		if feature.Code == "" {
			return invalidConfig("empty_feature_code")
		}
		if seen[feature.Code] {
			return invalidConfig("duplicate_feature_code")
		}
		if feature.Type != "TREND" && feature.Type != "COMBO_TREND" && feature.Type != "DISTRIBUTION" {
			return invalidConfig("unknown_feature_type")
		}
		seen[feature.Code] = true
	}

	enabled, successful, hasInvalid := 0, 0, false
	for _, feature := range snap.Features {
		view := emptyCardsViewFeature(feature)
		if !feature.Enabled {
			out.Features = append(out.Features, view)
			continue
		}
		enabled++
		var issue *cardsFeatureIssue
		if (feature.Code == "ACTIVE_OPCO" && feature.Type != "TREND") ||
			(feature.Code == "PENETRATION_OPCO" && feature.Type != "COMBO_TREND") ||
			((feature.Code == "TIER" || feature.Code == "MBR_TIER") && feature.Type != "DISTRIBUTION") {
			issue = invalidCardsFeature("incompatible_feature_type")
		} else {
			switch feature.Type {
			case "TREND":
				if feature.Code != "ACTIVE_OPCO" {
					issue = invalidCardsFeature("unsupported_trend_feature")
				} else {
					view, issue = buildCardsActivity(feature, firstMonth, lastMonth, snap.Activity)
				}
			case "COMBO_TREND":
				if feature.Code != "PENETRATION_OPCO" {
					issue = invalidCardsFeature("unsupported_combo_trend_feature")
				} else {
					view, issue = buildCardsPenetration(feature, firstMonth, lastMonth, snap.Penetration)
				}
			case "DISTRIBUTION":
				view, issue = buildCardsDistribution(feature, distributions[feature.Code])
			}
		}
		if issue != nil {
			code := "FEATURE_DATA_UNAVAILABLE"
			if issue.invalid {
				code = "FEATURE_DATA_INVALID"
				hasInvalid = true
			}
			out.Warnings = append(out.Warnings, domain.CardsWarning{
				FeatureCode: feature.Code, Code: code, Message: "Feature data is temporarily unavailable.",
			})
			fields := []any{"feature_code", feature.Code, "scope_code", scopeCode, "code", code,
				"reason", issue.reason, "trace_id", ctx.Value(contextkeys.TraceIdKey)}
			fields = append(fields, issue.details...)
			logger.Error("cards feature unavailable", fields...)
			continue
		}
		successful++
		out.Features = append(out.Features, view)
	}
	if enabled > 0 && successful == 0 {
		if hasInvalid {
			return domain.CardsView{}, domain.ErrCardsInvalidData
		}
		return domain.CardsView{}, domain.ErrCardsUnavailable
	}
	if len(out.Warnings) > 0 {
		out.Status = "partial"
		logger.Warn("cards partial response", "scope_code", scopeCode, "warning_count", len(out.Warnings),
			"successful_feature_count", successful, "trace_id", ctx.Value(contextkeys.TraceIdKey))
	}
	return out, nil
}

func emptyCardsViewFeature(feature domain.CardsFeature) domain.CardsViewFeature {
	return domain.CardsViewFeature{
		Code: feature.Code, Name: feature.Name, Type: feature.Type, Enabled: feature.Enabled,
		Activity: []domain.CardsActivityPoint{}, Trend: []domain.CardsMetricPoint{}, Buckets: []domain.CardsMetricPoint{},
	}
}

func buildCardsActivity(feature domain.CardsFeature, firstMonth, lastMonth time.Time, rows []domain.CardsActivityRow) (domain.CardsViewFeature, *cardsFeatureIssue) {
	view := emptyCardsViewFeature(feature)
	activity := map[string]domain.CardsActivityRow{}
	for _, row := range rows {
		key := row.Month.Format("2006-01-02")
		if row.Active < 0 || row.Inactive < 0 || row.Month.Day() != 1 {
			return view, invalidCardsFeature("invalid_activity_row", "report_month", key)
		}
		if _, exists := activity[key]; exists {
			return view, invalidCardsFeature("duplicate_activity_month", "report_month", key)
		}
		activity[key] = row
	}
	for month := firstMonth; !month.After(lastMonth); month = month.AddDate(0, 1, 0) {
		point := domain.CardsActivityPoint{Date: month.Format("2006-01-02")}
		if row, ok := activity[point.Date]; ok {
			active, inactive := row.Active, row.Inactive
			point.Active, point.Inactive = &active, &inactive
		}
		if previous, ok := activity[month.AddDate(-1, 0, 0).Format("2006-01-02")]; ok {
			value := previous.Active
			point.ActivePre = &value
		}
		view.Activity = append(view.Activity, point)
	}
	return view, nil
}

func buildCardsPenetration(feature domain.CardsFeature, firstMonth, lastMonth time.Time, rows []domain.CardsPenetrationRow) (domain.CardsViewFeature, *cardsFeatureIssue) {
	view := emptyCardsViewFeature(feature)
	penetration := map[string]domain.CardsPenetrationRow{}
	for _, row := range rows {
		key := row.Month.Format("2006-01-02")
		if row.Month.Day() != 1 || row.TotalSales < 0 || math.IsNaN(row.Percent100) || math.IsInf(row.Percent100, 0) || row.Percent100 < 0 || row.Percent100 > 100 {
			return view, invalidCardsFeature("invalid_penetration_row", "report_month", key)
		}
		if _, exists := penetration[key]; exists {
			return view, invalidCardsFeature("duplicate_penetration_month", "report_month", key)
		}
		penetration[key] = row
	}
	for month := firstMonth; !month.After(lastMonth); month = month.AddDate(0, 1, 0) {
		point := domain.CardsMetricPoint{Label: month.Format("2006-01-02")}
		if row, ok := penetration[point.Label]; ok {
			count, ratio := row.TotalSales, row.Percent100/100
			point.Count, point.Percent = &count, &ratio
		}
		view.Trend = append(view.Trend, point)
	}
	return view, nil
}

func buildCardsDistribution(feature domain.CardsFeature, rows []domain.CardsDistributionRow) (domain.CardsViewFeature, *cardsFeatureIssue) {
	view := emptyCardsViewFeature(feature)
	if len(rows) == 0 {
		return view, missingCardsFeature("missing_distribution")
	}
	var selectedDate time.Time
	bucketCodes := map[string]bool{}
	missing := false
	missingBucket := ""
	for _, row := range rows {
		if row.BucketCode == "" || bucketCodes[row.BucketCode] {
			return view, invalidCardsFeature("invalid_bucket_configuration", "bucket_code", row.BucketCode)
		}
		bucketCodes[row.BucketCode] = true
		if row.Date != nil {
			if selectedDate.IsZero() {
				selectedDate = *row.Date
			} else if !selectedDate.Equal(*row.Date) {
				return view, invalidCardsFeature("inconsistent_snapshot_dates", "bucket_code", row.BucketCode)
			}
		}
		if row.Count != nil && *row.Count < 0 {
			return view, invalidCardsFeature("negative_bucket_count", "bucket_code", row.BucketCode)
		}
		if row.Ratio != nil && (math.IsNaN(*row.Ratio) || math.IsInf(*row.Ratio, 0) || *row.Ratio < 0 || *row.Ratio > 1) {
			return view, invalidCardsFeature("invalid_bucket_ratio", "bucket_code", row.BucketCode)
		}
		if row.Date == nil || row.Count == nil || row.Ratio == nil {
			missing = true
			if missingBucket == "" {
				missingBucket = row.BucketCode
			}
			continue
		}
		view.Buckets = append(view.Buckets, domain.CardsMetricPoint{Label: row.Name, Count: row.Count, Percent: row.Ratio})
	}
	if missing {
		return view, missingCardsFeature("missing_bucket_data", "bucket_code", missingBucket, "selected_date", selectedDate)
	}
	if feature.Code == "MVC" {
		summary, ok := buildMVCSummary(rows)
		if !ok {
			return view, invalidCardsFeature("invalid_mvc_summary", "selected_date", selectedDate)
		}
		view.Summary = summary
	}
	return view, nil
}

func buildMVCSummary(rows []domain.CardsDistributionRow) (map[string]domain.CardsSummaryMetric, bool) {
	// The reference catalog uses bucket 99 for unacquired members. It is
	// displayed but excluded from the acquired-member summary denominator.
	expectedGroups := map[string]string{
		"1": "", "2": "MVC_ACTIVE", "3": "MVC_ACTIVE", "4": "MVC_ACTIVE",
		"5": "MVC_RISK", "6": "MVC_RISK", "99": "",
	}
	seen := make(map[string]bool, len(rows))
	var total, active, risk int64
	for _, row := range rows {
		if row.Count == nil || *row.Count < 0 || seen[row.BucketCode] {
			return nil, false
		}
		seen[row.BucketCode] = true
		group := strings.TrimSpace(row.RollupGroup)
		if row.IsRollup != (group != "" || row.BucketCode == "99") {
			return nil, false
		}
		if expected, known := expectedGroups[row.BucketCode]; known && group != expected {
			return nil, false
		}
		if row.BucketCode != "99" {
			if !addCardsCount(&total, *row.Count) {
				return nil, false
			}
		}
		switch group {
		case "":
		case "MVC_ACTIVE":
			if !addCardsCount(&active, *row.Count) {
				return nil, false
			}
		case "MVC_RISK":
			if !addCardsCount(&risk, *row.Count) {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	for code := range expectedGroups {
		if !seen[code] {
			return nil, false
		}
	}
	if active > total || risk > total || active > total-risk {
		return nil, false
	}
	var activePercent *float64
	if total > 0 {
		value := float64(active) / float64(total)
		activePercent = &value
	}
	return map[string]domain.CardsSummaryMetric{
		"total_members":     {Label: "Total Members", Count: total},
		"active_members":    {Label: "Active Members", Count: active, Percent: activePercent},
		"at_risk_customers": {Label: "At Risk Customers", Count: risk},
	}, true
}

func addCardsCount(total *int64, value int64) bool {
	const maxInt64 = int64(1<<63 - 1)
	if value < 0 || *total > maxInt64-value {
		return false
	}
	*total += value
	return true
}
