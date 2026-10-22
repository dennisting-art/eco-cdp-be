package service

import (
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"sort"
	"strconv"
	"time"
)

func buildUniverseAcquisition(s model.UniverseSnapshot) ([]model.UniverseViewMetric, *model.UniverseViewMetric, *universeChartIssue) {
	if s.AcquisitionDate.IsZero() || len(s.Acquisition) == 0 {
		return nil, nil, missingChart("missing_acquisition")
	}
	rows := append([]model.UniverseAcquisition(nil), s.Acquisition...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Year < rows[j].Year })
	out := make([]model.UniverseViewMetric, 0, len(rows))
	latestSelected := -1
	for i, a := range rows {
		if a.NewMembers < 0 || a.Members < 0 || a.Year < 1900 || a.Year > 9999 || (i > 0 && rows[i-1].Year == a.Year) {
			return nil, nil, invalidChart("invalid_acquisition")
		}
		if a.Year >= s.Range.Start.Year() && a.Year <= s.Range.End.Year() {
			count := a.NewMembers
			out = append(out, model.UniverseViewMetric{Label: strconv.Itoa(a.Year), Count: &count, Compare: a.Compare})
			latestSelected = i
		}
	}
	if latestSelected < 0 {
		return nil, nil, missingChart("missing_acquisition_years")
	}
	current := rows[latestSelected]
	growth := &model.UniverseViewMetric{Count: &current.NewMembers}
	if latestSelected > 0 {
		previous := rows[latestSelected-1]
		if previous.Year == current.Year-1 && previous.NewMembers > 0 && current.Compare != nil {
			ratio := float64(*current.Compare) / float64(previous.NewMembers)
			growth.Growth = &ratio
		}
	}
	return out, growth, nil
}

func buildUniverseBuckets(s model.UniverseSnapshot) ([]model.UniverseViewMetric, *universeChartIssue) {
	if reason := s.InvalidCharts["entity_member_count"]; reason != "" {
		return nil, invalidChart(reason)
	}
	if s.DistributionDate.IsZero() || len(s.Buckets) == 0 {
		return nil, missingChart("missing_distribution")
	}
	out := make([]model.UniverseViewMetric, 0, len(s.Buckets))
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
		out = append(out, model.UniverseViewMetric{Label: b.Name, Count: b.Count, Percent: b.Ratio})
	}
	if issue != nil {
		return nil, issue
	}
	return out, nil
}

func applyUniverseAcquisition(out *model.UniverseView, s model.UniverseSnapshot, record func(string, string, time.Time, *universeChartIssue)) {
	acquisition, growth, acquisitionIssue := buildUniverseAcquisition(s)
	if acquisitionIssue == nil {
		out.Group.AnnualMemberAcquisition = acquisition
		if sameDate(s.AcquisitionDate, s.Range.End) && len(acquisition) > 0 && acquisition[len(acquisition)-1].Label == strconv.Itoa(s.Range.End.Year()) {
			out.Group.LatestMemberGrowth = growth
		}
	}
	record("annual_member_acquisition", "", s.AcquisitionDate, acquisitionIssue)
	if acquisitionIssue != nil && !acquisitionIssue.invalid {
		out.Group.AnnualMemberAcquisition = []model.UniverseViewMetric{}
	}
	if acquisitionIssue == nil {
		missingYear := false
		for year := s.Range.Start.Year(); year <= s.Range.End.Year(); year++ {
			if year < 2018 {
				continue
			}
			found := false
			for _, point := range acquisition {
				if point.Label == strconv.Itoa(year) {
					found = true
					break
				}
			}
			if !found {
				missingYear = true
				break
			}
		}
		if missingYear || !sameDate(s.AcquisitionDate, s.Range.End) {
			out.DataStatuses["annual_member_acquisition"] = "partial"
			out.Status = "partial"
		}
	}
	acquisitionSnapshotIssue := universeSnapshotIssue(s.AcquisitionDate, s.Range.End)
	if acquisitionIssue != nil {
		acquisitionSnapshotIssue = acquisitionIssue
	} else if len(acquisition) == 0 || acquisition[len(acquisition)-1].Label != strconv.Itoa(s.Range.End.Year()) {
		acquisitionSnapshotIssue = missingChart("missing_current_acquisition_year")
	}
	record("latest_member_growth", "", s.AcquisitionDate, acquisitionSnapshotIssue)
}

func applyUniverseDistribution(out *model.UniverseView, s model.UniverseSnapshot, record func(string, string, time.Time, *universeChartIssue)) {
	if s.EntityCountEnabled {
		bucketIssue := universeSnapshotIssue(s.DistributionDate, s.Range.End)
		var buckets []model.UniverseViewMetric
		if bucketIssue == nil {
			buckets, bucketIssue = buildUniverseBuckets(s)
		}
		if bucketIssue == nil {
			out.Group.EntityMemberCount = buckets
		}
		record("entity_member_count", "", s.DistributionDate, bucketIssue)
	} else {
		out.AsOfDates.Group["entity_member_count"] = nil
	}
}
