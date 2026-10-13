package http

import (
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/domain"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/dto"
)

const universeDateLayout = "2006-01-02"

func mapUniverseMetric(v domain.UniverseViewMetric) dto.UniverseMetric {
	return dto.UniverseMetric{Label: v.Label, Count: v.Count, Percent: v.Percent, Growth: v.Growth, Compare: v.Compare}
}

func mapOptionalUniverseMetric(v *domain.UniverseViewMetric) *dto.UniverseMetric {
	if v == nil {
		return nil
	}
	out := mapUniverseMetric(*v)
	return &out
}
func mapUniverseMetricSlice(v []domain.UniverseViewMetric) []dto.UniverseMetric {
	if v == nil {
		return nil
	}
	out := make([]dto.UniverseMetric, 0, len(v))
	for _, metric := range v {
		out = append(out, mapUniverseMetric(metric))
	}
	return out
}

func universeChart[T any](data *T, enabled bool) dto.UniverseChart[T] {
	return dto.UniverseChart[T]{IsEnabled: enabled, Data: data}
}

func mapUniverseMetricChart(v *domain.UniverseViewMetric) dto.UniverseChart[dto.UniverseMetric] {
	return universeChart(mapOptionalUniverseMetric(v), true)
}

func mapUniverseMetricListChart(v []domain.UniverseViewMetric) dto.UniverseChart[[]dto.UniverseMetric] {
	if v == nil {
		return universeChart[[]dto.UniverseMetric](nil, true)
	}
	items := mapUniverseMetricSlice(v)
	return universeChart(&items, true)
}

func mapUniverseView(v domain.UniverseView) dto.UniverseResponse {
	out := dto.UniverseResponse{
		Status: v.Status, Warnings: []dto.UniverseWarning{},
		AsOfDate:  v.AsOfDate.Format(universeDateLayout),
		AsOfDates: dto.UniverseAsOfDates{Group: map[string]*string{}, Opcos: map[string]*string{}},
		Scopes:    make([]dto.UniverseScope, 0, len(v.Scopes)),
		Opcos:     make([]dto.UniverseOpco, 0, len(v.Opcos)),
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
		out.Warnings = append(out.Warnings, dto.UniverseWarning{Chart: w.Chart, ScopeCode: w.ScopeCode, Code: w.Code, Message: w.Message})
	}
	for _, scope := range v.Scopes {
		out.Scopes = append(out.Scopes, dto.UniverseScope{ScopeCode: scope.Code, DisplayName: scope.Name})
	}
	for _, card := range v.Opcos {
		item := dto.UniverseOpco{ScopeCode: card.ScopeCode, IsEnabled: card.IsEnabled}
		if card.IsEnabled && card.Member != nil && card.NonMember != nil {
			item.Data = &dto.UniverseOpcoData{
				TotalRetailCount: card.TotalRetailCount,
				Member:           mapOptionalUniverseMetric(card.Member), NonMember: mapOptionalUniverseMetric(card.NonMember),
			}
		}
		out.Opcos = append(out.Opcos, item)
	}
	group := v.Group
	out.Group.TotalMembers = mapUniverseMetricChart(group.TotalMembers)
	out.Group.LatestMemberGrowth = mapUniverseMetricChart(group.LatestMemberGrowth)
	if group.MemberActivity != nil {
		activity := dto.UniverseActivity{
			Active:   mapUniverseMetric(group.MemberActivity.Active),
			Inactive: mapUniverseMetric(group.MemberActivity.Inactive),
		}
		out.Group.MemberActivity = universeChart(&activity, true)
	} else {
		out.Group.MemberActivity = universeChart[dto.UniverseActivity](nil, true)
	}
	out.Group.MemberPenetrationRetail = mapUniverseMetricChart(group.MemberPenetrationRetail)
	out.Group.NonMemberTransactionRetail = mapUniverseMetricChart(group.NonMemberTransactionRetail)
	out.Group.AnnualMemberAcquisition = mapUniverseMetricListChart(group.AnnualMemberAcquisition)
	out.Group.EntityMemberCount = mapUniverseMetricListChart(group.EntityMemberCount)
	out.Group.EntityMemberCount.IsEnabled = group.EntityCountEnabled
	if group.MemberMatrixActivity != nil {
		matrix := dto.UniverseMatrix{
			Columns: append([]string{}, group.MemberMatrixActivity.Columns...),
			Rows:    make([]dto.UniverseMatrixRow, 0, len(group.MemberMatrixActivity.Rows)),
		}
		for _, row := range group.MemberMatrixActivity.Rows {
			mapped := dto.UniverseMatrixRow{
				PrimaryEntity: mapUniverseMetric(row.PrimaryEntity),
				Relationships: make([]dto.UniverseMetric, 0, len(row.Relationships)),
			}
			for _, relationship := range row.Relationships {
				mapped.Relationships = append(mapped.Relationships, mapUniverseMetric(relationship))
			}
			matrix.Rows = append(matrix.Rows, mapped)
		}
		out.Group.MemberMatrixActivity = universeChart(&matrix, group.MatrixEnabled)
	} else {
		out.Group.MemberMatrixActivity = universeChart[dto.UniverseMatrix](nil, group.MatrixEnabled)
	}
	return out
}
