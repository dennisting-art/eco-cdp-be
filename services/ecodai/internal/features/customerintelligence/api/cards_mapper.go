package api

import (
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"strings"
)

func mapCardsView(view model.CardsView) CardsResponse {
	out := CardsResponse{
		Status: view.Status, ScopeCode: view.ScopeCode, StartDate: view.StartDate, EndDate: view.EndDate,
		Warnings: make([]CardsWarning, 0, len(view.Warnings)),
		Features: make(map[string]CardsFeature, len(view.Features)),
	}
	for _, warning := range view.Warnings {
		out.Warnings = append(out.Warnings, CardsWarning{
			FeatureCode: warning.FeatureCode,
			Code:        warning.Code,
			Message:     warning.Message,
		})
	}
	for _, feature := range view.Features {
		item := CardsFeature{IsEnabled: feature.Enabled, DataStatus: feature.DataStatus}
		if !feature.Enabled || feature.Unavailable || feature.DataStatus == "stale" || feature.Type == "DISTRIBUTION" && feature.DataStatus == "missing" {
			out.Features[strings.ToLower(feature.Code)] = item
			continue
		}
		switch feature.Type {
		case "TREND":
			trend := make([]CardsActivityPoint, 0, len(feature.Activity))
			for _, point := range feature.Activity {
				trend = append(trend, CardsActivityPoint{Date: point.Date, Active: point.Active, ActivePre: point.ActivePre, Inactive: point.Inactive})
			}
			item.Data = trend
		case "COMBO_TREND":
			trend := make([]CardsMetricPoint, 0, len(feature.Trend))
			for _, point := range feature.Trend {
				trend = append(trend, CardsMetricPoint{Label: point.Label, Count: point.Count, Percent: point.Percent})
			}
			item.Data = trend
		case "DISTRIBUTION":
			buckets := make([]CardsMetricPoint, 0, len(feature.Buckets))
			for _, point := range feature.Buckets {
				buckets = append(buckets, CardsMetricPoint{Label: point.Label, Count: point.Count, Percent: point.Percent})
			}
			if feature.Code == "MVC" {
				data := CardsMVCData{Buckets: buckets, Summary: make(map[string]CardsSummaryMetric, len(feature.Summary))}
				for code, metric := range feature.Summary {
					data.Summary[code] = CardsSummaryMetric{
						Label: metric.Label, Count: metric.Count, Percent: metric.Percent,
						Growth: metric.Growth, Compare: metric.Compare,
					}
				}
				item.Data = data
			} else {
				item.Data = buckets
			}
		}
		out.Features[strings.ToLower(feature.Code)] = item
	}
	return out
}
