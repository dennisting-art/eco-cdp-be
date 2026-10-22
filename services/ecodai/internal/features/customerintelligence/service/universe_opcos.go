package service

import (
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"time"
)

func applyUniverseOpcos(out *model.UniverseView, s model.UniverseSnapshot, record func(string, string, time.Time, *universeChartIssue)) {
	for _, scope := range s.Scopes {
		card := model.UniverseViewOpco{ScopeCode: scope.Code, IsEnabled: scope.Enabled}
		if !scope.Enabled {
			// Preserve the pre-existing disabled-card placeholders.
			card.Member, card.NonMember = &model.UniverseViewMetric{}, &model.UniverseViewMetric{}
			out.AsOfDates.Opcos[scope.Code] = nil
		} else {
			summary := s.Summaries[scope.Code]
			issue := universeSnapshotIssue(summary.Date, s.Range.End)
			if issue == nil {
				issue = universeRetail(summary.Date, summary.TotalRetail, summary.RetailMembers, summary.RetailNonMembers)
			}
			if issue == nil {
				card.TotalRetailCount = summary.TotalRetail
				card.Member = &model.UniverseViewMetric{Count: summary.RetailMembers, Percent: universeRatio(summary.RetailMembers, summary.TotalRetail)}
				card.NonMember = &model.UniverseViewMetric{Count: summary.RetailNonMembers, Percent: universeRatio(summary.RetailNonMembers, summary.TotalRetail)}
			}
			record("opco_retail", scope.Code, summary.Date, issue)
		}
		out.Opcos = append(out.Opcos, card)
	}
}
