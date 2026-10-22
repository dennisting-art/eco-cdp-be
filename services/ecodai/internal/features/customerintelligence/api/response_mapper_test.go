package api

import (
	"encoding/json"
	"testing"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
)

func TestCardsResponseUsesFeatureDataWrappers(t *testing.T) {
	count := int64(42)
	view := model.CardsView{
		Status: "partial", ScopeCode: "AEON",
		Warnings: []model.CardsWarning{},
		Features: []model.CardsViewFeature{
			{Code: "ACTIVE_OPCO", Name: "Active vs Inactive", Type: "TREND", Enabled: true, Activity: []model.CardsActivityPoint{{Date: "2026-10-01", Active: &count}}},
			{Code: "PENETRATION_OPCO", Name: "Member Penetration", Type: "COMBO_TREND", Enabled: true, Trend: []model.CardsMetricPoint{{Label: "2026-10-01", Count: &count}}},
			{Code: "TIER", Name: "Member Tier Distribution", Type: "DISTRIBUTION", Enabled: true, Buckets: []model.CardsMetricPoint{{Label: "1-Star", Count: &count}}},
			{Code: "MVC", Name: "Member Value Centre", Type: "DISTRIBUTION", Enabled: true, Buckets: []model.CardsMetricPoint{{Label: "New", Count: &count}}, Summary: map[string]model.CardsSummaryMetric{"total_members": {Label: "Total Members", Count: 42}}},
			{Code: "SEX", Name: "Gender Distribution", Type: "DISTRIBUTION", Enabled: false},
			{Code: "AGE", Name: "Age Distribution", Type: "DISTRIBUTION", Enabled: true, DataStatus: "missing"},
		},
	}
	encoded, err := json.Marshal(mapCardsView(view))
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Features map[string]struct {
			IsEnabled    bool            `json:"is_enabled"`
			Data         json.RawMessage `json:"data"`
			FeatureCode  json.RawMessage `json:"feature_code"`
			DisplayName  json.RawMessage `json:"display_name"`
			Type         json.RawMessage `json:"type"`
			DisplayOrder json.RawMessage `json:"display_order"`
			Trend        json.RawMessage `json:"trend"`
		} `json:"features"`
	}
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	active := body.Features["active_opco"]
	if !active.IsEnabled || len(active.Trend) != 0 || len(active.FeatureCode) != 0 || len(active.DisplayName) != 0 || len(active.Type) != 0 || len(active.DisplayOrder) != 0 || string(active.Data) != `[{"date":"2026-10-01","active":42,"active_pre":null,"inactive":null}]` {
		t.Fatalf("active feature shape: %s", encoded)
	}
	penetration := body.Features["penetration_opco"]
	if !penetration.IsEnabled || string(penetration.Data) != `[{"label":"2026-10-01","count":42,"percent":null}]` {
		t.Fatalf("combo trend shape: %s", encoded)
	}
	tier := body.Features["tier"]
	if !tier.IsEnabled || string(tier.Data) != `[{"label":"1-Star","count":42,"percent":null}]` {
		t.Fatalf("distribution shape: %s", encoded)
	}
	mvc := body.Features["mvc"]
	if !mvc.IsEnabled || string(mvc.Data) != `{"buckets":[{"label":"New","count":42,"percent":null}],"summary":{"total_members":{"label":"Total Members","count":42,"percent":null}}}` {
		t.Fatalf("MVC shape: %s", encoded)
	}
	disabled := body.Features["sex"]
	if disabled.IsEnabled || string(disabled.Data) != "null" {
		t.Fatalf("disabled feature shape: %s", encoded)
	}
	if age, found := body.Features["age"]; !found || !age.IsEnabled || string(age.Data) != "null" {
		t.Fatalf("missing feature must remain visible: %s", encoded)
	}
}

func TestUniverseResponseUsesChartDataWrappers(t *testing.T) {
	count := int64(100)
	date := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	view := model.UniverseView{
		Status: "partial", AsOfDate: date,
		AsOfDates: model.UniverseViewDates{Group: map[string]*time.Time{}, Opcos: map[string]*time.Time{}},
		Group: model.UniverseViewGroup{
			TotalMembers: &model.UniverseViewMetric{Count: &count}, MatrixEnabled: false,
			EntityCountEnabled: true, EntityMemberCount: []model.UniverseViewMetric{{Label: "1 Entity Only", Count: &count}},
		},
		Opcos: []model.UniverseViewOpco{
			{ScopeCode: "ABG", IsEnabled: true, TotalRetailCount: &count, Member: &model.UniverseViewMetric{Count: &count}, NonMember: &model.UniverseViewMetric{Count: &count}},
			{ScopeCode: "ADB", IsEnabled: false, Member: &model.UniverseViewMetric{}, NonMember: &model.UniverseViewMetric{}},
		},
	}
	encoded, err := json.Marshal(mapUniverseView(view))
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Group map[string]struct {
			IsEnabled bool            `json:"is_enabled"`
			Data      json.RawMessage `json:"data"`
		} `json:"group"`
		Opcos []struct {
			ScopeCode string          `json:"scope_code"`
			IsEnabled bool            `json:"is_enabled"`
			Data      json.RawMessage `json:"data"`
		} `json:"opcos"`
	}
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	if !body.Group["total_members"].IsEnabled || string(body.Group["total_members"].Data) != `{"count":100}` || !body.Group["member_activity"].IsEnabled || string(body.Group["member_activity"].Data) != "null" {
		t.Fatalf("group chart shape: %s", encoded)
	}
	if body.Group["member_matrix_activity"].IsEnabled || string(body.Group["member_matrix_activity"].Data) != "null" {
		t.Fatalf("disabled matrix shape: %s", encoded)
	}
	if !body.Group["entity_member_count"].IsEnabled || string(body.Group["entity_member_count"].Data) != `[{"label":"1 Entity Only","count":100}]` {
		t.Fatalf("entity distribution shape: %s", encoded)
	}
	if len(body.Opcos) != 2 || !body.Opcos[0].IsEnabled || string(body.Opcos[0].Data) == "null" || body.Opcos[1].IsEnabled || string(body.Opcos[1].Data) != "null" {
		t.Fatalf("opco chart shape: %s", encoded)
	}
}

func TestDataStatusIsLocalToAffectedCharts(t *testing.T) {
	cards := mapCardsView(model.CardsView{StartDate: "2026-09-01", EndDate: "2026-09-30", Features: []model.CardsViewFeature{
		{Code: "TIER", Type: "DISTRIBUTION", Enabled: true, DataStatus: "stale"},
		{Code: "ACTIVE_OPCO", Type: "TREND", Enabled: true, DataStatus: "missing"},
		{Code: "SEX", Type: "DISTRIBUTION", Enabled: false},
	}})
	encoded, err := json.Marshal(cards)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Features map[string]struct {
			Data       json.RawMessage `json:"data"`
			DataStatus string          `json:"data_status"`
		} `json:"features"`
	}
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	if string(body.Features["tier"].Data) != "null" || body.Features["tier"].DataStatus != "stale" ||
		string(body.Features["active_opco"].Data) != "[]" || body.Features["active_opco"].DataStatus != "missing" ||
		body.Features["sex"].DataStatus != "" {
		t.Fatalf("chart statuses: %s", encoded)
	}

	universe := mapUniverseView(model.UniverseView{DataStatuses: map[string]string{"total_members": "stale", "annual_member_acquisition": "missing", "opco:A": "missing"},
		Group: model.UniverseViewGroup{AnnualMemberAcquisition: []model.UniverseViewMetric{}}, Opcos: []model.UniverseViewOpco{{ScopeCode: "A", IsEnabled: true}, {ScopeCode: "B", IsEnabled: false}}})
	encoded, err = json.Marshal(universe)
	if err != nil {
		t.Fatal(err)
	}
	var charts struct {
		Group map[string]struct {
			Data       json.RawMessage `json:"data"`
			DataStatus string          `json:"data_status"`
		} `json:"group"`
		Opcos []struct {
			DataStatus string `json:"data_status"`
		} `json:"opcos"`
	}
	if err := json.Unmarshal(encoded, &charts); err != nil {
		t.Fatal(err)
	}
	if charts.Group["total_members"].DataStatus != "stale" || string(charts.Group["annual_member_acquisition"].Data) != "[]" || charts.Group["annual_member_acquisition"].DataStatus != "missing" || charts.Opcos[0].DataStatus != "missing" || charts.Opcos[1].DataStatus != "" {
		t.Fatalf("universe statuses: %s", encoded)
	}
}
