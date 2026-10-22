package service

import (
	"strings"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
)

func requestedRange(start, end string) (*model.ReportingRange, error) {
	start, end = strings.TrimSpace(start), strings.TrimSpace(end)
	if start == "" && end == "" {
		return nil, nil
	}
	if start == "" || end == "" {
		return nil, model.ErrInvalidReportingRange
	}
	parse := func(value string) (time.Time, error) {
		if len(value) != len("2006-01-02") {
			return time.Time{}, model.ErrInvalidReportingRange
		}
		date, err := time.Parse("2006-01-02", value)
		if err != nil || date.Format("2006-01-02") != value {
			return time.Time{}, model.ErrInvalidReportingRange
		}
		return date, nil
	}
	first, err := parse(start)
	if err != nil {
		return nil, err
	}
	last, err := parse(end)
	if err != nil || first.After(last) {
		return nil, model.ErrInvalidReportingRange
	}
	return &model.ReportingRange{Start: first, End: last}, nil
}
