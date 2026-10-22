package model

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
	DataStatus       string
	Unavailable      bool
	Activity         []CardsActivityPoint
	Trend            []CardsMetricPoint
	Buckets          []CardsMetricPoint
	Summary          map[string]CardsSummaryMetric
}

type CardsWarning struct {
	FeatureCode, Code, Message string
}

type CardsView struct {
	Status             string
	StartDate, EndDate string
	Warnings           []CardsWarning
	ScopeCode          string
	Features           []CardsViewFeature
}
