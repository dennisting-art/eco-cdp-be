package api

type CardsRequest struct {
	ScopeCode *string `json:"scope_code"`
	StartDate string  `json:"start_date"`
	EndDate   string  `json:"end_date"`
}

type CardsActivityPoint struct {
	Date      string `json:"date"`
	Active    *int64 `json:"active"`
	ActivePre *int64 `json:"active_pre"`
	Inactive  *int64 `json:"inactive"`
}

type CardsMetricPoint struct {
	Label   string   `json:"label"`
	Count   *int64   `json:"count"`
	Percent *float64 `json:"percent"`
}

type CardsSummaryMetric struct {
	Label   string   `json:"label"`
	Count   int64    `json:"count"`
	Percent *float64 `json:"percent"`
	Growth  *float64 `json:"growth,omitempty"`
	Compare *int64   `json:"compare,omitempty"`
}

type CardsFeature struct {
	IsEnabled  bool   `json:"is_enabled"`
	Data       any    `json:"data"`
	DataStatus string `json:"data_status,omitempty"`
}

type CardsMVCData struct {
	Buckets []CardsMetricPoint            `json:"buckets"`
	Summary map[string]CardsSummaryMetric `json:"summary"`
}

type CardsWarning struct {
	FeatureCode string `json:"feature_code"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

type CardsResponse struct {
	Status    string                  `json:"status"`
	StartDate string                  `json:"start_date"`
	EndDate   string                  `json:"end_date"`
	Warnings  []CardsWarning          `json:"warnings"`
	ScopeCode string                  `json:"scope_code"`
	Features  map[string]CardsFeature `json:"features"`
}
