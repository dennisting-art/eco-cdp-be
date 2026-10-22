package service

import (
	"context"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/logger"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"time"
)

// buildUniverse is also used by snapshot-only tests.
func buildUniverse(s model.UniverseSnapshot) (model.UniverseView, error) {
	return buildUniverseContext(context.Background(), s)
}

type universeChartIssue struct {
	invalid bool
	reason  string
	details []any
}

func missingChart(reason string, details ...any) *universeChartIssue {
	return &universeChartIssue{reason: reason, details: details}
}
func invalidChart(reason string, details ...any) *universeChartIssue {
	return &universeChartIssue{invalid: true, reason: reason, details: details}
}

func buildUniverseContext(ctx context.Context, s model.UniverseSnapshot) (model.UniverseView, error) {
	out := model.UniverseView{
		Status: "complete", Warnings: []model.UniverseWarning{},
		DataStatuses: map[string]string{},
		AsOfDates:    model.UniverseViewDates{Group: map[string]*time.Time{}, Opcos: map[string]*time.Time{}},
		Scopes:       []model.UniverseViewScope{}, Opcos: []model.UniverseViewOpco{},
	}
	if !s.Range.Start.IsZero() {
		out.StartDate = s.Range.Start.Format("2006-01-02")
	}
	if !s.Range.End.IsZero() {
		out.EndDate = s.Range.End.Format("2006-01-02")
	}
	if !s.GroupConfigured || s.GroupCode == "" {
		return model.UniverseView{}, model.ErrUniverseInvalidData
	}
	seen := map[string]bool{}
	for _, scope := range s.Scopes {
		if scope.Code == "" || scope.Code == s.GroupCode || seen[scope.Code] {
			return model.UniverseView{}, model.ErrUniverseInvalidData
		}
		seen[scope.Code] = true
		out.Scopes = append(out.Scopes, model.UniverseViewScope{Code: scope.Code, Name: scope.Name})
	}
	successful, hasInvalid := 0, false
	record := func(chart, scope string, date time.Time, issue *universeChartIssue) {
		if scope == "" {
			out.AsOfDates.Group[chart] = nil
		} else {
			out.AsOfDates.Opcos[scope] = nil
		}
		if issue != nil {
			if !issue.invalid {
				status := "missing"
				if issue.reason == "stale" {
					status = "stale"
				}
				key := chart
				if scope != "" {
					key = "opco:" + scope
				}
				out.DataStatuses[key] = status
				out.Status = "partial"
				return
			}
			code := "CHART_DATA_INVALID"
			hasInvalid = true
			message := "Chart data is temporarily unavailable."
			if chart == "member_matrix_activity" {
				message = "Member activity matrix is temporarily unavailable."
			}
			out.Warnings = append(out.Warnings, model.UniverseWarning{Chart: chart, ScopeCode: scope, Code: code, Message: message})
			fields := []any{"chart", chart, "group", s.GroupCode, "scope", scope, "selected_date", date, "reason", issue.reason, "code", code}
			fields = append(fields, issue.details...)
			logger.Error(ctx, "universe chart unavailable", fields...)
			return
		}
		successful++
		if scope == "" {
			out.AsOfDates.Group[chart] = &date
		} else {
			out.AsOfDates.Opcos[scope] = &date
		}
		if date.After(out.AsOfDate) {
			out.AsOfDate = date
		}
	}
	applyUniverseGroup(&out, s, record)
	applyUniverseAcquisition(&out, s, record)
	applyUniverseDistribution(&out, s, record)
	applyUniverseOpcos(&out, s, record)
	applyUniverseMatrix(&out, s, record)
	if successful == 0 && hasInvalid {
		return model.UniverseView{}, model.ErrUniverseInvalidData
	}
	if len(out.Warnings) > 0 {
		out.Status = "partial"
		logger.Warn(ctx, "universe partial response", "warning_count", len(out.Warnings), "group", s.GroupCode)
	}
	return out, nil
}
