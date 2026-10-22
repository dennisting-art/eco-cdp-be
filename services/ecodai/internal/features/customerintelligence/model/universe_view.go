package model

import "time"

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
	MatrixEnabled, EntityCountEnabled                   bool
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
	Status             string
	StartDate, EndDate string
	DataStatuses       map[string]string
	Warnings           []UniverseWarning
	AsOfDate           time.Time
	AsOfDates          UniverseViewDates
	Scopes             []UniverseViewScope
	Group              UniverseViewGroup
	Opcos              []UniverseViewOpco
}

type UniverseWarning struct {
	Chart, ScopeCode, Code, Message string
}
