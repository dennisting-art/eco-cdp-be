package model

import "time"

type CardsFeature struct {
	Code, Name, Type string
	Enabled          bool
}

type CardsActivityRow struct {
	Month            time.Time
	Date             time.Time
	Active, Inactive int64
}

type CardsPenetrationRow struct {
	Month      time.Time
	Date       time.Time
	Complete   bool
	TotalSales int64
	Percent100 float64
}

type CardsDistributionRow struct {
	FeatureCode, BucketCode, Name string
	IsRollup                      bool
	RollupGroup                   string
	Date                          *time.Time
	Count                         *int64
	Ratio                         *float64
}

type CardsSnapshot struct {
	ScopeFound, ScopeAvailable bool
	Range                      ReportingRange
	Features                   []CardsFeature
	Activity                   []CardsActivityRow
	Penetration                []CardsPenetrationRow
	Distributions              []CardsDistributionRow
}
