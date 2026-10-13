package domain

import (
	"errors"
	"time"
)

var (
	ErrCardsForbidden    = errors.New("cards access forbidden")
	ErrCardsInvalidScope = errors.New("invalid cards scope")
	ErrCardsScopeMissing = errors.New("cards scope not found")
	ErrCardsUnavailable  = errors.New("cards data unavailable")
	ErrCardsInvalidData  = errors.New("invalid cards reporting data")
	ErrCardsDatabase     = errors.New("cards database failure")
)

type CardsQuery struct {
	ScopeCode   *string
	Permissions []string
}

type CardsFeature struct {
	Code, Name, Type string
	Enabled          bool
}

type CardsActivityRow struct {
	Month            time.Time
	Active, Inactive int64
}

type CardsPenetrationRow struct {
	Month      time.Time
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
	Features                   []CardsFeature
	Activity                   []CardsActivityRow
	Penetration                []CardsPenetrationRow
	Distributions              []CardsDistributionRow
}

type CardsActivityPoint struct {
	Date                        string
	Active, ActivePre, Inactive *int64
}

type CardsMetricPoint struct {
	Label   string
	Count   *int64
	Percent *float64
}

type CardsSummaryMetric struct {
	Label   string
	Count   int64
	Percent *float64
	Growth  *float64
	Compare *int64
}

type CardsViewFeature struct {
	Code, Name, Type string
	Enabled          bool
	Activity         []CardsActivityPoint
	Trend            []CardsMetricPoint
	Buckets          []CardsMetricPoint
	Summary          map[string]CardsSummaryMetric
}

type CardsWarning struct {
	FeatureCode, Code, Message string
}

type CardsView struct {
	Status    string
	Warnings  []CardsWarning
	ScopeCode string
	Features  []CardsViewFeature
}
