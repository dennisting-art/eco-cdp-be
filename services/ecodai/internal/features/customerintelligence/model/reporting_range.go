package model

import (
	"errors"
	"time"
)

var ErrInvalidReportingRange = errors.New("invalid reporting date range")

type ReportingRange struct {
	Start time.Time
	End   time.Time
}
