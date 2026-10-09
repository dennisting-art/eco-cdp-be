package domain

import (
	"errors"
	"time"
)

var (
	ErrUniverseForbidden       = errors.New("universe access forbidden")
	ErrUniverseDateUnsupported = errors.New("universe date filtering unsupported")
	ErrUniverseUnavailable     = errors.New("universe data unavailable")
	ErrUniverseInvalidData     = errors.New("invalid universe reporting data")
	ErrUniverseDatabase        = errors.New("universe database failure")
)

type UniverseQuery struct {
	StartDate, EndDate string
	Permissions        []string
}

type UniverseScope struct {
	Code, Name string
	Enabled    bool
}

type UniverseSummary struct {
	Date                                         time.Time
	TotalMembers, ActiveMembers, InactiveMembers *int64
	TotalRetail, RetailMembers, RetailNonMembers *int64
}

type UniverseAcquisition struct {
	Year                int
	Members, NewMembers int64
	Compare             *int64
}

type UniverseBucket struct {
	Code, Name string
	Count      *int64
	Ratio      *float64
}

type UniverseCell struct {
	Primary, Related string
	Count            int64
	Ratio            float64
}

type UniverseSnapshot struct {
	InvalidCharts    map[string]string
	GroupConfigured  bool
	GroupCode        string
	MatrixEnabled    bool
	Scopes           []UniverseScope
	Summaries        map[string]UniverseSummary
	AcquisitionDate  time.Time
	Acquisition      []UniverseAcquisition
	DistributionDate time.Time
	Buckets          []UniverseBucket
	MatrixDate       time.Time
	Cells            []UniverseCell
	MatrixTotals     map[string]*int64
}

type UniverseViewMetric struct {
	Label           string
	Count           *int64
	Percent, Growth *float64
	Compare         *int64
}

type UniverseViewScope struct{ Code, Name string }
type UniverseViewActivity struct{ Active, Inactive UniverseViewMetric }
type UniverseViewMatrixRow struct {
	PrimaryEntity UniverseViewMetric
	Relationships []UniverseViewMetric
}
type UniverseViewMatrix struct {
	Columns []string
	Rows    []UniverseViewMatrixRow
}
type UniverseViewGroup struct {
	TotalMembers, LatestMemberGrowth                    *UniverseViewMetric
	MemberActivity                                      *UniverseViewActivity
	MemberPenetrationRetail, NonMemberTransactionRetail *UniverseViewMetric
	AnnualMemberAcquisition, EntityMemberCount          []UniverseViewMetric
	MemberMatrixActivity                                *UniverseViewMatrix
}
type UniverseViewOpco struct {
	ScopeCode         string
	IsEnabled         bool
	TotalRetailCount  *int64
	Member, NonMember *UniverseViewMetric
}
type UniverseViewDates struct {
	Group map[string]*time.Time
	Opcos map[string]*time.Time
}
type UniverseView struct {
	Status    string
	Warnings  []UniverseWarning
	AsOfDate  time.Time
	AsOfDates UniverseViewDates
	Scopes    []UniverseViewScope
	Group     UniverseViewGroup
	Opcos     []UniverseViewOpco
}

type UniverseWarning struct {
	Chart, ScopeCode, Code, Message string
}
