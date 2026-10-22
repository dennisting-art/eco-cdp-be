package service

import (
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"time"
)

func applyUniverseGroup(out *model.UniverseView, s model.UniverseSnapshot, record func(string, string, time.Time, *universeChartIssue)) {
	group := s.Summaries[s.GroupCode]
	out.Group.MatrixEnabled = s.MatrixEnabled
	out.Group.EntityCountEnabled = s.EntityCountEnabled
	totalIssue := universeSnapshotIssue(group.Date, s.Range.End)
	if totalIssue == nil {
		totalIssue = universeCounts(group.Date, group.TotalMembers)
	}
	if totalIssue == nil {
		out.Group.TotalMembers = &model.UniverseViewMetric{Count: group.TotalMembers}
	}
	record("total_members", "", group.Date, totalIssue)
	activityIssue := universeSnapshotIssue(group.Date, s.Range.End)
	if activityIssue == nil {
		activityIssue = universeCounts(group.Date, group.TotalMembers, group.ActiveMembers, group.InactiveMembers)
	}
	if activityIssue == nil && (*group.ActiveMembers > *group.TotalMembers || *group.InactiveMembers > *group.TotalMembers) {
		activityIssue = invalidChart("member_count_exceeds_total")
	}
	if activityIssue == nil {
		out.Group.MemberActivity = &model.UniverseViewActivity{
			Active:   model.UniverseViewMetric{Count: group.ActiveMembers, Percent: universeRatio(group.ActiveMembers, group.TotalMembers)},
			Inactive: model.UniverseViewMetric{Count: group.InactiveMembers, Percent: universeRatio(group.InactiveMembers, group.TotalMembers)},
		}
	}
	record("member_activity", "", group.Date, activityIssue)
	for _, metric := range []struct {
		name   string
		count  *int64
		target **model.UniverseViewMetric
	}{
		{"member_penetration_retail", group.RetailMembers, &out.Group.MemberPenetrationRetail},
		{"non_member_transaction_retail", group.RetailNonMembers, &out.Group.NonMemberTransactionRetail},
	} {
		issue := universeSnapshotIssue(group.Date, s.Range.End)
		if issue == nil {
			issue = universeRetail(group.Date, group.TotalRetail, metric.count)
		}
		if issue == nil {
			*metric.target = &model.UniverseViewMetric{Count: metric.count, Percent: universeRatio(metric.count, group.TotalRetail)}
		}
		record(metric.name, "", group.Date, issue)
	}
}
