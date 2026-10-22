package service

import (
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"math"
	"time"
)

func buildCardsActivity(feature model.CardsFeature, firstMonth, lastMonth time.Time, rows []model.CardsActivityRow) (model.CardsViewFeature, *cardsFeatureIssue) {
	view := emptyCardsViewFeature(feature)
	activity := map[string]model.CardsActivityRow{}
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
	missing := false
	hasSource := false
	for month := monthStart(firstMonth); !month.After(lastMonth); month = month.AddDate(0, 1, 0) {
		point := model.CardsActivityPoint{Date: month.Format("2006-01-02")}
		periodEnd := month.AddDate(0, 1, 0).AddDate(0, 0, -1)
		if lastMonth.Before(periodEnd) {
			periodEnd = lastMonth
		}
		row, ok := activity[point.Date]
		if ok && !row.Date.Before(firstMonth) && !row.Date.After(lastMonth) {
			hasSource = true
		}
		if ok && !row.Date.Before(firstMonth) && sameDate(row.Date, periodEnd) {
			active, inactive := row.Active, row.Inactive
			point.Active, point.Inactive = &active, &inactive
		} else {
			missing = true
			continue
		}
		if previous, ok := activity[month.AddDate(-1, 0, 0).Format("2006-01-02")]; ok {
			value := previous.Active
			point.ActivePre = &value
		}
		view.Activity = append(view.Activity, point)
	}
	if len(view.Activity) == 0 {
		if hasSource {
			view.DataStatus = "partial"
		} else {
			view.DataStatus = "missing"
		}
	} else if missing {
		view.DataStatus = "partial"
	}
	return view, nil
}

func buildCardsPenetration(feature model.CardsFeature, firstMonth, lastMonth time.Time, rows []model.CardsPenetrationRow) (model.CardsViewFeature, *cardsFeatureIssue) {
	view := emptyCardsViewFeature(feature)
	penetration := map[string]model.CardsPenetrationRow{}
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
	missing := false
	hasSource := len(rows) > 0
	for month := monthStart(firstMonth); !month.After(lastMonth); month = month.AddDate(0, 1, 0) {
		point := model.CardsMetricPoint{Label: month.Format("2006-01-02")}
		if row, ok := penetration[point.Label]; ok && row.Complete {
			count, ratio := row.TotalSales, row.Percent100/100
			point.Count, point.Percent = &count, &ratio
		} else {
			missing = true
			continue
		}
		view.Trend = append(view.Trend, point)
	}
	if len(view.Trend) == 0 {
		if hasSource {
			view.DataStatus = "partial"
		} else {
			view.DataStatus = "missing"
		}
	} else if missing {
		view.DataStatus = "partial"
	}
	return view, nil
}

func monthStart(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
}
func sameDate(a, b time.Time) bool { return a.Format("2006-01-02") == b.Format("2006-01-02") }
