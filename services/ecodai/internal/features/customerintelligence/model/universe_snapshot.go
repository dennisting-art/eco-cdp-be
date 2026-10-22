package model

import "time"

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
	Range              ReportingRange
	InvalidCharts      map[string]string
	GroupConfigured    bool
	GroupCode          string
	MatrixEnabled      bool
	EntityCountEnabled bool
	Scopes             []UniverseScope
	Summaries          map[string]UniverseSummary
	AcquisitionDate    time.Time
	Acquisition        []UniverseAcquisition
	DistributionDate   time.Time
	Buckets            []UniverseBucket
	MatrixDate         time.Time
	Cells              []UniverseCell
	MatrixTotals       map[string]*int64
}
