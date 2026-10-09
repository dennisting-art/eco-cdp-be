package dto

type CardsRequest struct {
	ScopeCode *string `json:"scope_code"`
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
	FeatureCode string                        `json:"feature_code"`
	DisplayName string                        `json:"display_name"`
	Type        string                        `json:"type"`
	IsEnabled   bool                          `json:"is_enabled"`
	Trend       any                           `json:"trend,omitempty"`
	Buckets     *[]CardsMetricPoint           `json:"buckets,omitempty"`
	Summary     map[string]CardsSummaryMetric `json:"summary,omitempty"`
}

type CardsWarning struct {
	FeatureCode string `json:"feature_code"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

type CardsResponse struct {
	Status    string         `json:"status"`
	Warnings  []CardsWarning `json:"warnings"`
	ScopeCode string         `json:"scope_code"`
	Features  []CardsFeature `json:"features"`
}
