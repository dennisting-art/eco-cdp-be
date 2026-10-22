package api

type UniverseRequest struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type UniverseMetric struct {
	Label   string   `json:"label,omitempty"`
	Count   *int64   `json:"count"`
	Percent *float64 `json:"percent,omitempty"`
	Growth  *float64 `json:"growth,omitempty"`
	Compare *int64   `json:"compare,omitempty"`
}

type UniverseScope struct {
	ScopeCode   string `json:"scope_code"`
	DisplayName string `json:"display_name"`
}

type UniverseActivity struct {
	Active   UniverseMetric `json:"active"`
	Inactive UniverseMetric `json:"inactive"`
}

type UniverseMatrixRow struct {
	PrimaryEntity UniverseMetric   `json:"primary_entity"`
	Relationships []UniverseMetric `json:"relationships"`
}

type UniverseMatrix struct {
	Columns []string            `json:"columns"`
	Rows    []UniverseMatrixRow `json:"rows"`
}

type UniverseChart[T any] struct {
	IsEnabled  bool   `json:"is_enabled"`
	Data       *T     `json:"data"`
	DataStatus string `json:"data_status,omitempty"`
}

type UniverseGroup struct {
	TotalMembers               UniverseChart[UniverseMetric]   `json:"total_members"`
	LatestMemberGrowth         UniverseChart[UniverseMetric]   `json:"latest_member_growth"`
	MemberActivity             UniverseChart[UniverseActivity] `json:"member_activity"`
	MemberPenetrationRetail    UniverseChart[UniverseMetric]   `json:"member_penetration_retail"`
	NonMemberTransactionRetail UniverseChart[UniverseMetric]   `json:"non_member_transaction_retail"`
	AnnualMemberAcquisition    UniverseChart[[]UniverseMetric] `json:"annual_member_acquisition"`
	EntityMemberCount          UniverseChart[[]UniverseMetric] `json:"entity_member_count"`
	MemberMatrixActivity       UniverseChart[UniverseMatrix]   `json:"member_matrix_activity"`
}

type UniverseOpco struct {
	ScopeCode  string            `json:"scope_code"`
	IsEnabled  bool              `json:"is_enabled"`
	Data       *UniverseOpcoData `json:"data"`
	DataStatus string            `json:"data_status,omitempty"`
}

type UniverseOpcoData struct {
	TotalRetailCount *int64          `json:"total_retail_count"`
	Member           *UniverseMetric `json:"member"`
	NonMember        *UniverseMetric `json:"non_member"`
}

type UniverseAsOfDates struct {
	Group map[string]*string `json:"group"`
	Opcos map[string]*string `json:"opcos"`
}

type UniverseResponse struct {
	Status    string            `json:"status"`
	StartDate string            `json:"start_date"`
	EndDate   string            `json:"end_date"`
	Warnings  []UniverseWarning `json:"warnings"`
	AsOfDate  string            `json:"as_of_date"`
	AsOfDates UniverseAsOfDates `json:"as_of_dates"`
	Scopes    []UniverseScope   `json:"scopes"`
	Group     UniverseGroup     `json:"group"`
	Opcos     []UniverseOpco    `json:"opcos"`
}

type UniverseWarning struct {
	Chart     string `json:"chart"`
	ScopeCode string `json:"scope_code,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}
