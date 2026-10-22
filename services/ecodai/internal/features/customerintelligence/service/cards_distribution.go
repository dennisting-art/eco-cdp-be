package service

import (
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"math"
	"strings"
	"time"
)

func buildCardsDistribution(feature model.CardsFeature, rows []model.CardsDistributionRow) (model.CardsViewFeature, *cardsFeatureIssue) {
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
		view.Buckets = append(view.Buckets, model.CardsMetricPoint{Label: row.Name, Count: row.Count, Percent: row.Ratio})
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

func buildMVCSummary(rows []model.CardsDistributionRow) (map[string]model.CardsSummaryMetric, bool) {
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
	return map[string]model.CardsSummaryMetric{
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
