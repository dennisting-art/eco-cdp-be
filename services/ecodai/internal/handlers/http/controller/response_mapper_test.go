package http

import (
	"encoding/json"
	"testing"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/domain"
)

func TestCardsResponseUsesFeatureDataWrappers(t *testing.T) {
	count := int64(42)
	view := domain.CardsView{
		Status: "partial", ScopeCode: "AEON",
		Warnings: []domain.CardsWarning{{FeatureCode: "TIER", Code: "FEATURE_DATA_UNAVAILABLE", Message: "Feature data is temporarily unavailable."}},
		Features: []domain.CardsViewFeature{
			{Code: "ACTIVE_OPCO", Name: "Active vs Inactive", Type: "TREND", Enabled: true, Activity: []domain.CardsActivityPoint{{Date: "2026-10-01", Active: &count}}},
			{Code: "PENETRATION_OPCO", Name: "Member Penetration", Type: "COMBO_TREND", Enabled: true, Trend: []domain.CardsMetricPoint{{Label: "2026-10-01", Count: &count}}},
			{Code: "MVC", Name: "Member Value Centre", Type: "DISTRIBUTION", Enabled: true, Buckets: []domain.CardsMetricPoint{{Label: "New", Count: &count}}, Summary: map[string]domain.CardsSummaryMetric{"total_members": {Label: "Total Members", Count: 42}}},
			{Code: "SEX", Name: "Gender Distribution", Type: "DISTRIBUTION", Enabled: false},
		},
	}
	encoded, err := json.Marshal(mapCardsView(view))
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Features map[string]struct {
			DisplayOrder int             `json:"display_order"`
			IsEnabled    bool            `json:"is_enabled"`
			Data         json.RawMessage `json:"data"`
			Trend        json.RawMessage `json:"trend"`
		} `json:"features"`
	}
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	active := body.Features["active_opco"]
	if !active.IsEnabled || active.DisplayOrder != 1 || len(active.Trend) != 0 || string(active.Data) != `{"points":[{"date":"2026-10-01","active":42,"active_pre":null,"inactive":null}]}` {
		t.Fatalf("active feature shape: %s", encoded)
	}
	penetration := body.Features["penetration_opco"]
	if !penetration.IsEnabled || string(penetration.Data) != `{"points":[{"label":"2026-10-01","count":42,"percent":null}]}` {
		t.Fatalf("combo trend shape: %s", encoded)
	}
	mvc := body.Features["mvc"]
	if !mvc.IsEnabled || mvc.DisplayOrder != 3 || string(mvc.Data) != `{"buckets":[{"label":"New","count":42,"percent":null}],"summary":{"total_members":{"label":"Total Members","count":42,"percent":null}}}` {
		t.Fatalf("distribution shape: %s", encoded)
	}
	disabled := body.Features["sex"]
	if disabled.IsEnabled || disabled.DisplayOrder != 4 || string(disabled.Data) != "null" {
		t.Fatalf("disabled feature shape: %s", encoded)
	}
	if _, found := body.Features["tier"]; found {
		t.Fatalf("failed feature should be reported only in warnings: %s", encoded)
	}
}

func TestUniverseResponseUsesChartDataWrappers(t *testing.T) {
	count := int64(100)
	date := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	view := domain.UniverseView{
		Status: "partial", AsOfDate: date,
		AsOfDates: domain.UniverseViewDates{Group: map[string]*time.Time{}, Opcos: map[string]*time.Time{}},
		Group: domain.UniverseViewGroup{
			TotalMembers: &domain.UniverseViewMetric{Count: &count}, MatrixEnabled: false,
			EntityCountEnabled: true, EntityMemberCount: []domain.UniverseViewMetric{{Label: "1 Entity Only", Count: &count}},
		},
		Opcos: []domain.UniverseViewOpco{
			{ScopeCode: "ABG", IsEnabled: true, TotalRetailCount: &count, Member: &domain.UniverseViewMetric{Count: &count}, NonMember: &domain.UniverseViewMetric{Count: &count}},
			{ScopeCode: "ADB", IsEnabled: false, Member: &domain.UniverseViewMetric{}, NonMember: &domain.UniverseViewMetric{}},
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
