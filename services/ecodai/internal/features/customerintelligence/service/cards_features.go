package service

import (
	"context"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/logger"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"strings"
	"time"
)

func buildCards(scopeCode string, firstMonth, lastMonth time.Time, snap model.CardsSnapshot) (model.CardsView, error) {
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

func buildCardsContext(ctx context.Context, scopeCode string, firstMonth, lastMonth time.Time, snap model.CardsSnapshot) (model.CardsView, error) {
	out := model.CardsView{
		Status: "complete", Warnings: []model.CardsWarning{}, ScopeCode: scopeCode,
		Features: []model.CardsViewFeature{},
	}
	if !firstMonth.IsZero() {
		out.StartDate = firstMonth.Format("2006-01-02")
	}
	if !lastMonth.IsZero() {
		out.EndDate = lastMonth.Format("2006-01-02")
	}
	invalidConfig := func(reason string) (model.CardsView, error) {
		logger.Error(ctx, "cards reporting data invalid", "feature", "feature_config", "scope_code", scopeCode,
			"reason", reason)
		return model.CardsView{}, model.ErrCardsInvalidData
	}
	if len(snap.Features) == 0 {
		return invalidConfig("missing_feature_config")
	}

	distributions := map[string][]model.CardsDistributionRow{}
	for _, row := range snap.Distributions {
		distributions[row.FeatureCode] = append(distributions[row.FeatureCode], row)
	}

	seen := map[string]bool{}
	for _, feature := range snap.Features {
		if feature.Code == "" {
			return invalidConfig("empty_feature_code")
		}
		key := strings.ToLower(feature.Code)
		if seen[key] {
			return invalidConfig("duplicate_feature_code")
		}
		if feature.Type != "TREND" && feature.Type != "COMBO_TREND" && feature.Type != "DISTRIBUTION" {
			return invalidConfig("unknown_feature_type")
		}
		seen[key] = true
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
				rows := distributions[feature.Code]
				var latest time.Time
				for _, row := range rows {
					if row.Date != nil && row.Date.After(latest) {
						latest = *row.Date
					}
				}
				if latest.IsZero() {
					view.DataStatus = "missing"
				} else if !sameDate(latest, lastMonth) {
					view.DataStatus = "stale"
				} else {
					view, issue = buildCardsDistribution(feature, rows)
				}
			}
		}
		if issue != nil {
			view.Unavailable = true
			if !issue.invalid {
				view.DataStatus = "missing"
				out.Status = "partial"
				out.Features = append(out.Features, view)
				continue
			}
			code := "FEATURE_DATA_INVALID"
			if issue.invalid {
				hasInvalid = true
			}
			out.Warnings = append(out.Warnings, model.CardsWarning{
				FeatureCode: feature.Code, Code: code, Message: "Feature data is temporarily unavailable.",
			})
			fields := []any{"feature_code", feature.Code, "scope_code", scopeCode, "code", code,
				"reason", issue.reason}
			fields = append(fields, issue.details...)
			logger.Error(ctx, "cards feature unavailable", fields...)
			out.Features = append(out.Features, view)
			continue
		}
		successful++
		if view.DataStatus != "" {
			out.Status = "partial"
		}
		out.Features = append(out.Features, view)
	}
	if enabled > 0 && successful == 0 && hasInvalid {
		return model.CardsView{}, model.ErrCardsInvalidData
	}
	if len(out.Warnings) > 0 {
		out.Status = "partial"
		logger.Warn(ctx, "cards partial response", "scope_code", scopeCode, "warning_count", len(out.Warnings),
			"successful_feature_count", successful)
	}
	return out, nil
}

func emptyCardsViewFeature(feature model.CardsFeature) model.CardsViewFeature {
	return model.CardsViewFeature{
		Code: feature.Code, Name: feature.Name, Type: feature.Type, Enabled: feature.Enabled,
		Activity: []model.CardsActivityPoint{}, Trend: []model.CardsMetricPoint{}, Buckets: []model.CardsMetricPoint{},
	}
}
