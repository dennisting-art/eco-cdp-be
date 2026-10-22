package service

import (
	"math"
	"time"
)

func universeSnapshotIssue(date, expected time.Time) *universeChartIssue {
	if date.IsZero() {
		return missingChart("missing_snapshot_date")
	}
	if !expected.IsZero() && !sameDate(date, expected) {
		return missingChart("stale")
	}
	return nil
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
