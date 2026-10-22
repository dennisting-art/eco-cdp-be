package api

import (
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
)

const universeDateLayout = "2006-01-02"

func mapUniverseMetric(v model.UniverseViewMetric) UniverseMetric {
	return UniverseMetric{Label: v.Label, Count: v.Count, Percent: v.Percent, Growth: v.Growth, Compare: v.Compare}
}

func mapOptionalUniverseMetric(v *model.UniverseViewMetric) *UniverseMetric {
	if v == nil {
		return nil
	}
	out := mapUniverseMetric(*v)
	return &out
}
func mapUniverseMetricSlice(v []model.UniverseViewMetric) []UniverseMetric {
	if v == nil {
		return nil
	}
	out := make([]UniverseMetric, 0, len(v))
	for _, metric := range v {
		out = append(out, mapUniverseMetric(metric))
	}
	return out
}

func universeChart[T any](data *T, enabled bool) UniverseChart[T] {
	return UniverseChart[T]{IsEnabled: enabled, Data: data}
}

func mapUniverseMetricChart(v *model.UniverseViewMetric) UniverseChart[UniverseMetric] {
	return universeChart(mapOptionalUniverseMetric(v), true)
}

func mapUniverseMetricListChart(v []model.UniverseViewMetric) UniverseChart[[]UniverseMetric] {
	if v == nil {
		return universeChart[[]UniverseMetric](nil, true)
	}
	items := mapUniverseMetricSlice(v)
	return universeChart(&items, true)
}

func mapUniverseView(v model.UniverseView) UniverseResponse {
	out := UniverseResponse{
		Status: v.Status, Warnings: []UniverseWarning{}, StartDate: v.StartDate, EndDate: v.EndDate,
		AsOfDates: UniverseAsOfDates{Group: map[string]*string{}, Opcos: map[string]*string{}},
		Scopes:    make([]UniverseScope, 0, len(v.Scopes)),
		Opcos:     make([]UniverseOpco, 0, len(v.Opcos)),
	}
	if !v.AsOfDate.IsZero() {
		out.AsOfDate = v.AsOfDate.Format(universeDateLayout)
	}
	for key, date := range v.AsOfDates.Group {
		if date == nil {
			out.AsOfDates.Group[key] = nil
			continue
		}
		formatted := date.Format(universeDateLayout)
		out.AsOfDates.Group[key] = &formatted
	}
	for key, date := range v.AsOfDates.Opcos {
		if date == nil {
			out.AsOfDates.Opcos[key] = nil
			continue
		}
		formatted := date.Format(universeDateLayout)
		out.AsOfDates.Opcos[key] = &formatted
	}
	for _, w := range v.Warnings {
		out.Warnings = append(out.Warnings, UniverseWarning{Chart: w.Chart, ScopeCode: w.ScopeCode, Code: w.Code, Message: w.Message})
	}
	for _, scope := range v.Scopes {
		out.Scopes = append(out.Scopes, UniverseScope{ScopeCode: scope.Code, DisplayName: scope.Name})
	}
	for _, card := range v.Opcos {
		item := UniverseOpco{ScopeCode: card.ScopeCode, IsEnabled: card.IsEnabled, DataStatus: v.DataStatuses["opco:"+card.ScopeCode]}
		if card.IsEnabled && card.Member != nil && card.NonMember != nil {
			item.Data = &UniverseOpcoData{
				TotalRetailCount: card.TotalRetailCount,
				Member:           mapOptionalUniverseMetric(card.Member), NonMember: mapOptionalUniverseMetric(card.NonMember),
			}
		}
		out.Opcos = append(out.Opcos, item)
	}
	group := v.Group
	out.Group.TotalMembers = mapUniverseMetricChart(group.TotalMembers)
	out.Group.TotalMembers.DataStatus = v.DataStatuses["total_members"]
	out.Group.LatestMemberGrowth = mapUniverseMetricChart(group.LatestMemberGrowth)
	out.Group.LatestMemberGrowth.DataStatus = v.DataStatuses["latest_member_growth"]
	if group.MemberActivity != nil {
		activity := UniverseActivity{
			Active:   mapUniverseMetric(group.MemberActivity.Active),
			Inactive: mapUniverseMetric(group.MemberActivity.Inactive),
		}
		out.Group.MemberActivity = universeChart(&activity, true)
	} else {
		out.Group.MemberActivity = universeChart[UniverseActivity](nil, true)
	}
	out.Group.MemberPenetrationRetail = mapUniverseMetricChart(group.MemberPenetrationRetail)
	out.Group.MemberActivity.DataStatus = v.DataStatuses["member_activity"]
	out.Group.MemberPenetrationRetail.DataStatus = v.DataStatuses["member_penetration_retail"]
	out.Group.NonMemberTransactionRetail = mapUniverseMetricChart(group.NonMemberTransactionRetail)
	out.Group.NonMemberTransactionRetail.DataStatus = v.DataStatuses["non_member_transaction_retail"]
	out.Group.AnnualMemberAcquisition = mapUniverseMetricListChart(group.AnnualMemberAcquisition)
	out.Group.AnnualMemberAcquisition.DataStatus = v.DataStatuses["annual_member_acquisition"]
	out.Group.EntityMemberCount = mapUniverseMetricListChart(group.EntityMemberCount)
	out.Group.EntityMemberCount.IsEnabled = group.EntityCountEnabled
	out.Group.EntityMemberCount.DataStatus = v.DataStatuses["entity_member_count"]
	if group.MemberMatrixActivity != nil {
		matrix := UniverseMatrix{
			Columns: append([]string{}, group.MemberMatrixActivity.Columns...),
			Rows:    make([]UniverseMatrixRow, 0, len(group.MemberMatrixActivity.Rows)),
		}
		for _, row := range group.MemberMatrixActivity.Rows {
			mapped := UniverseMatrixRow{
				PrimaryEntity: mapUniverseMetric(row.PrimaryEntity),
				Relationships: make([]UniverseMetric, 0, len(row.Relationships)),
			}
			for _, relationship := range row.Relationships {
				mapped.Relationships = append(mapped.Relationships, mapUniverseMetric(relationship))
			}
			matrix.Rows = append(matrix.Rows, mapped)
		}
		out.Group.MemberMatrixActivity = universeChart(&matrix, group.MatrixEnabled)
	} else {
		out.Group.MemberMatrixActivity = universeChart[UniverseMatrix](nil, group.MatrixEnabled)
	}
	out.Group.MemberMatrixActivity.DataStatus = v.DataStatuses["member_matrix_activity"]
	return out
}
