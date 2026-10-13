package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/domain"
)

type cardsFakeRepo struct {
	calls      int
	scope      string
	firstMonth time.Time
	lastMonth  time.Time
	snap       domain.CardsSnapshot
}

func TestCardsWildcardPermission(t *testing.T) {
	r := &cardsFakeRepo{snap: cardsFixture()}
	_, _ = NewCardsService(r).Query(context.Background(), domain.CardsQuery{Permissions: []string{"cdphub.*"}})
	if r.calls != 1 {
		t.Fatal("CDP Hub wildcard did not reach repository")
	}
	r.calls = 0
	_, err := NewCardsService(r).Query(context.Background(), domain.CardsQuery{Permissions: []string{"segment.*"}})
	if !errors.Is(err, domain.ErrCardsForbidden) || r.calls != 0 {
		t.Fatal("unrelated grant authorized Cards")
	}
}

func (r *cardsFakeRepo) GetSnapshot(_ context.Context, scope string, first, last time.Time) (domain.CardsSnapshot, error) {
	r.calls++
	r.scope, r.firstMonth, r.lastMonth = scope, first, last
	return r.snap, nil
}

func cardsDate(s string) time.Time {
	d, _ := time.Parse("2006-01-02", s)
	return d
}

func cardsI64(n int64) *int64     { return &n }
func cardsF64(n float64) *float64 { return &n }

func cardsFixture() domain.CardsSnapshot {
	date := cardsDate("2026-09-24")
	return domain.CardsSnapshot{
		ScopeFound: true, ScopeAvailable: true,
		Features: []domain.CardsFeature{
			{Code: "ACTIVE_OPCO", Name: "Activity", Type: "TREND", Enabled: true},
			{Code: "TIER", Name: "Tier", Type: "DISTRIBUTION", Enabled: true},
			{Code: "PENETRATION_OPCO", Name: "Penetration", Type: "COMBO_TREND", Enabled: true},
			{Code: "MVC", Name: "Value Centre", Type: "DISTRIBUTION", Enabled: false},
		},
		Activity: []domain.CardsActivityRow{
			{Month: cardsDate("2025-09-01"), Active: 70, Inactive: 10},
			{Month: cardsDate("2026-08-01"), Active: 0, Inactive: 0},
			{Month: cardsDate("2026-09-01"), Active: 100, Inactive: 20},
		},
		Penetration: []domain.CardsPenetrationRow{
			{Month: cardsDate("2026-08-01"), TotalSales: 0, Percent100: 0},
			{Month: cardsDate("2026-09-01"), TotalSales: 1000, Percent100: 34},
		},
		Distributions: []domain.CardsDistributionRow{
			{FeatureCode: "TIER", BucketCode: "4", Name: "4-Star", Date: &date, Count: cardsI64(0), Ratio: cardsF64(0)},
		},
	}
}

func TestCardsScopePermissionAndMonthlyMapping(t *testing.T) {
	repo := &cardsFakeRepo{snap: cardsFixture()}
	svc := NewCardsService(repo)
	svc.now = func() time.Time { return cardsDate("2026-09-29") }
	_, err := svc.Query(context.Background(), domain.CardsQuery{})
	if !errors.Is(err, domain.ErrCardsForbidden) || repo.calls != 0 {
		t.Fatalf("permission must deny before read: %v, calls=%d", err, repo.calls)
	}
	out, err := svc.Query(context.Background(), domain.CardsQuery{Permissions: []string{CustomerIntelligenceViewPermission}})
	if err != nil {
		t.Fatal(err)
	}
	if repo.scope != "AEON" || repo.firstMonth.Format("2006-01-02") != "2026-04-01" || repo.lastMonth.Format("2006-01-02") != "2026-09-01" {
		t.Fatalf("scope/window: %s %s %s", repo.scope, repo.firstMonth, repo.lastMonth)
	}
	if len(out.Features) != 4 || len(out.Features[0].Activity) != 6 || len(out.Features[2].Trend) != 6 {
		t.Fatalf("feature or month count: %+v", out)
	}
	if out.Status != "complete" || len(out.Warnings) != 0 {
		t.Fatalf("complete response metadata: %+v", out)
	}
	activity := out.Features[0].Activity
	if activity[3].Active != nil || activity[3].Inactive != nil || activity[3].ActivePre != nil {
		t.Fatalf("missing July should be null: %+v", activity[3])
	}
	if activity[4].Active == nil || *activity[4].Active != 0 || activity[5].ActivePre == nil || *activity[5].ActivePre != 70 {
		t.Fatalf("zero or prior-year value lost: %+v", activity)
	}
	if out.Features[1].Buckets[0].Count == nil || *out.Features[1].Buckets[0].Count != 0 || out.Features[1].Buckets[0].Percent == nil || *out.Features[1].Buckets[0].Percent != 0 {
		t.Fatalf("stored zero bucket lost: %+v", out.Features[1])
	}
	trend := out.Features[2].Trend
	if trend[3].Count != nil || trend[4].Count == nil || *trend[4].Count != 0 || trend[5].Percent == nil || *trend[5].Percent != 0.34 {
		t.Fatalf("penetration mapping: %+v", trend)
	}
	if len(out.Features[3].Buckets) != 0 || out.Features[3].Enabled {
		t.Fatalf("disabled feature: %+v", out.Features[3])
	}
	custom := "ABG"
	if _, err := svc.Query(context.Background(), domain.CardsQuery{ScopeCode: &custom, Permissions: []string{CustomerIntelligenceViewPermission}}); err != nil || repo.scope != "ABG" {
		t.Fatalf("explicit scope: %s, %v", repo.scope, err)
	}
	empty := ""
	if _, err := svc.Query(context.Background(), domain.CardsQuery{ScopeCode: &empty, Permissions: []string{CustomerIntelligenceViewPermission}}); err != nil || repo.scope != "AEON" {
		t.Fatalf("empty scope should select AEON: %s, %v", repo.scope, err)
	}
}

func TestCardsPartialFeatureFailures(t *testing.T) {
	base := cardsFixture()
	base.Distributions[0].Ratio = nil
	out, err := buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), base)
	if err != nil || out.Status != "partial" || len(out.Warnings) != 1 ||
		out.Warnings[0].FeatureCode != "TIER" || out.Warnings[0].Code != "FEATURE_DATA_UNAVAILABLE" ||
		cardsHasFeature(out, "TIER") {
		t.Fatalf("expected omitted unavailable feature: %+v, %v", out, err)
	}

	base = cardsFixture()
	base.Features[0].Type = "DISTRIBUTION"
	out, err = buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), base)
	if err != nil || len(out.Warnings) != 1 || out.Warnings[0].FeatureCode != "ACTIVE_OPCO" ||
		out.Warnings[0].Code != "FEATURE_DATA_INVALID" || cardsHasFeature(out, "ACTIVE_OPCO") {
		t.Fatalf("expected omitted invalid feature mapping: %+v, %v", out, err)
	}

	base = cardsFixture()
	base.Distributions[0].Ratio = nil
	base.Penetration[0].Percent100 = 120
	out, err = buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), base)
	if err != nil || len(out.Warnings) != 2 ||
		out.Warnings[0].FeatureCode != "TIER" || out.Warnings[0].Code != "FEATURE_DATA_UNAVAILABLE" ||
		out.Warnings[1].FeatureCode != "PENETRATION_OPCO" || out.Warnings[1].Code != "FEATURE_DATA_INVALID" {
		t.Fatalf("expected ordered unavailable and invalid warnings: %+v, %v", out, err)
	}
}

func cardsHasFeature(view domain.CardsView, code string) bool {
	for _, feature := range view.Features {
		if feature.Code == code {
			return true
		}
	}
	return false
}

func TestCardsAllEnabledFeaturesFail(t *testing.T) {
	missing := mvcCardsFixture()
	missing.Distributions[0].Ratio = nil
	if _, err := buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), missing); !errors.Is(err, domain.ErrCardsUnavailable) {
		t.Fatalf("all unavailable features: %v", err)
	}

	invalid := mvcCardsFixture()
	invalid.Distributions = invalid.Distributions[:6]
	if _, err := buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), invalid); !errors.Is(err, domain.ErrCardsInvalidData) {
		t.Fatalf("all invalid features: %v", err)
	}

	mixed := cardsFixture()
	mixed.Features = mixed.Features[:2]
	mixed.Features[0].Type = "DISTRIBUTION"
	mixed.Distributions[0].Ratio = nil
	if _, err := buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), mixed); !errors.Is(err, domain.ErrCardsInvalidData) {
		t.Fatalf("invalid failure must take precedence when all features fail: %v", err)
	}
}

func TestCardsRejectsEndpointFeatureConfiguration(t *testing.T) {
	for name, mutate := range map[string]func(*domain.CardsSnapshot){
		"empty code": func(s *domain.CardsSnapshot) { s.Features[0].Code = "" },
		"duplicate code": func(s *domain.CardsSnapshot) {
			s.Features[1].Code = s.Features[0].Code
		},
		"case-insensitive duplicate code": func(s *domain.CardsSnapshot) {
			s.Features[1].Code = strings.ToLower(s.Features[0].Code)
		},
		"unknown type": func(s *domain.CardsSnapshot) { s.Features[0].Type = "UNKNOWN" },
	} {
		t.Run(name, func(t *testing.T) {
			fixture := cardsFixture()
			mutate(&fixture)
			if _, err := buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), fixture); !errors.Is(err, domain.ErrCardsInvalidData) {
				t.Fatalf("endpoint configuration must fail: %v", err)
			}
		})
	}
}

func TestCardsScopeValidationAndMissingScope(t *testing.T) {
	repo := &cardsFakeRepo{snap: domain.CardsSnapshot{}}
	svc := NewCardsService(repo)
	bad := " "
	_, err := svc.Query(context.Background(), domain.CardsQuery{ScopeCode: &bad, Permissions: []string{CustomerIntelligenceViewPermission}})
	if !errors.Is(err, domain.ErrCardsInvalidScope) || repo.calls != 0 {
		t.Fatalf("invalid scope should not query repository: %v, calls=%d", err, repo.calls)
	}
	_, err = svc.Query(context.Background(), domain.CardsQuery{Permissions: []string{CustomerIntelligenceViewPermission}})
	if !errors.Is(err, domain.ErrCardsScopeMissing) || repo.calls != 1 {
		t.Fatalf("missing AEON scope: %v, calls=%d", err, repo.calls)
	}
}

func TestCardsWindowUsesMalaysiaMonthBoundary(t *testing.T) {
	repo := &cardsFakeRepo{snap: domain.CardsSnapshot{
		ScopeFound: true, ScopeAvailable: true,
		Features: []domain.CardsFeature{{Code: "MVC", Name: "Value Centre", Type: "DISTRIBUTION", Enabled: false}},
	}}
	svc := NewCardsService(repo)
	svc.now = func() time.Time { return time.Date(2026, 9, 30, 16, 30, 0, 0, time.UTC) }
	out, err := svc.Query(context.Background(), domain.CardsQuery{Permissions: []string{CustomerIntelligenceViewPermission}})
	if err != nil || repo.lastMonth.Format("2006-01-02") != "2026-10-01" || out.Status != "complete" ||
		len(out.Warnings) != 0 || len(out.Features) != 1 || out.Features[0].Enabled {
		t.Fatalf("Malaysia month boundary or disabled-only response: %s, %+v, %v", repo.lastMonth, out, err)
	}
}

func mvcCardsFixture() domain.CardsSnapshot {
	date := cardsDate("2026-09-24")
	counts := []int64{20, 20, 30, 20, 5, 5, 10}
	codes := []string{"1", "2", "3", "4", "5", "6", "99"}
	names := []string{"New", "Activated", "Engaged", "Loyal", "Declining", "Churned", "Unacquired"}
	groups := []string{"", "MVC_ACTIVE", "MVC_ACTIVE", "MVC_ACTIVE", "MVC_RISK", "MVC_RISK", ""}
	rows := make([]domain.CardsDistributionRow, 0, len(counts))
	for i, count := range counts {
		rows = append(rows, domain.CardsDistributionRow{
			FeatureCode: "MVC", BucketCode: codes[i], Name: names[i],
			IsRollup: groups[i] != "" || codes[i] == "99", RollupGroup: groups[i],
			Date: &date, Count: cardsI64(count), Ratio: cardsF64(0),
		})
	}
	return domain.CardsSnapshot{
		ScopeFound: true, ScopeAvailable: true,
		Features:      []domain.CardsFeature{{Code: "MVC", Name: "Member Value Centre", Type: "DISTRIBUTION", Enabled: true}},
		Distributions: rows,
	}
}

func TestCardsMVCSummary(t *testing.T) {
	fixture := mvcCardsFixture()
	view, err := buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), fixture)
	if err != nil {
		t.Fatal(err)
	}
	feature := view.Features[0]
	if len(feature.Buckets) != 7 || feature.Summary["total_members"].Count != 100 ||
		feature.Summary["active_members"].Count != 70 || feature.Summary["at_risk_customers"].Count != 10 {
		t.Fatalf("wrong MVC summary: %+v", feature)
	}
	percent := feature.Summary["active_members"].Percent
	if percent == nil || *percent != 0.7 {
		t.Fatalf("wrong active ratio: %v", percent)
	}

	for i := range fixture.Distributions {
		*fixture.Distributions[i].Count = 0
	}
	view, err = buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), fixture)
	if err != nil || view.Features[0].Summary["active_members"].Percent != nil {
		t.Fatalf("zero total must have null active ratio: %+v, %v", view, err)
	}
}

func TestCardsMVCRejectsIncompleteOrInvalidGroups(t *testing.T) {
	fixture := mvcCardsFixture()
	fixture.Distributions = append(fixture.Distributions[:5], fixture.Distributions[6:]...)
	if _, err := buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), fixture); !errors.Is(err, domain.ErrCardsInvalidData) {
		t.Fatalf("missing Churned bucket: %v", err)
	}
	fixture = mvcCardsFixture()
	fixture.Distributions[1].RollupGroup = "MVC_RISK"
	if _, err := buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), fixture); !errors.Is(err, domain.ErrCardsInvalidData) {
		t.Fatalf("misgrouped Activated bucket: %v", err)
	}
	fixture = mvcCardsFixture()
	fixture.Distributions[4].Count = nil
	if _, err := buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), fixture); !errors.Is(err, domain.ErrCardsUnavailable) {
		t.Fatalf("missing Declining count: %v", err)
	}
	fixture = mvcCardsFixture()
	fixture.Distributions[6].IsRollup = false
	if _, err := buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), fixture); !errors.Is(err, domain.ErrCardsInvalidData) {
		t.Fatalf("Unacquired bucket must retain catalog rollup flag: %v", err)
	}
	fixture = mvcCardsFixture()
	fixture.Features[0].Enabled = false
	view, err := buildCards("AEON", cardsDate("2026-04-01"), cardsDate("2026-09-01"), fixture)
	if err != nil || len(view.Features[0].Summary) != 0 || len(view.Features[0].Buckets) != 0 {
		t.Fatalf("disabled MVC has summary: %+v, %v", view, err)
	}
}
