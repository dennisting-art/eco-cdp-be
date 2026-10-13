package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/domain"
)

type universeFakeRepo struct {
	calls int
	snap  domain.UniverseSnapshot
	err   error
}

func TestUniverseWildcardPermission(t *testing.T) {
	r := &universeFakeRepo{snap: fixtureUniverse()}
	_, _ = NewUniverseService(r).Query(context.Background(), domain.UniverseQuery{Permissions: []string{"cdphub.*"}})
	if r.calls != 1 {
		t.Fatal("CDP Hub wildcard did not reach repository")
	}
	r.calls = 0
	_, err := NewUniverseService(r).Query(context.Background(), domain.UniverseQuery{Permissions: []string{"segment.*"}})
	if !errors.Is(err, domain.ErrUniverseForbidden) || r.calls != 0 {
		t.Fatal("unrelated grant authorized Universe")
	}
}

func (r *universeFakeRepo) GetLatestCharts(context.Context) (domain.UniverseSnapshot, error) {
	r.calls++
	return r.snap, r.err
}
func u64(n int64) *int64    { return &n }
func uf(n float64) *float64 { return &n }
func ud(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }

func fixtureUniverse() domain.UniverseSnapshot {
	summary := func(date string, members, retail int64) domain.UniverseSummary {
		return domain.UniverseSummary{Date: ud(date), TotalMembers: u64(members), ActiveMembers: u64(members / 2), InactiveMembers: u64(members - members/2), TotalRetail: u64(retail), RetailMembers: u64(retail / 2), RetailNonMembers: u64(retail - retail/2)}
	}
	return domain.UniverseSnapshot{
		GroupConfigured: true,
		GroupCode:       "AEON_GROUP",
		MatrixEnabled:   true,
		Scopes:          []domain.UniverseScope{{Code: "B", Name: "Beta", Enabled: true}, {Code: "A", Name: "Alpha", Enabled: true}, {Code: "X", Name: "Disabled", Enabled: false}},
		Summaries:       map[string]domain.UniverseSummary{"AEON_GROUP": summary("2026-03-31", 1000, 100), "A": summary("2026-03-25", 100, 10), "B": summary("2026-04-01", 200, 20)},
		AcquisitionDate: ud("2026-03-30"), Acquisition: []domain.UniverseAcquisition{{Year: 2026, NewMembers: 50, Compare: u64(-50)}, {Year: 2025, NewMembers: 100}},
		DistributionDate: ud("2026-03-29"), Buckets: []domain.UniverseBucket{{Code: "1", Name: "1 Entity Only", Count: u64(2), Ratio: uf(0.2)}, {Code: "2", Name: "2 Entities", Count: u64(0), Ratio: uf(0)}, {Code: "3", Name: "3+ Entities", Count: u64(8), Ratio: uf(0.8)}},
		MatrixDate: ud("2026-03-28"), MatrixTotals: map[string]*int64{"A": u64(90), "B": u64(180)},
		Cells: []domain.UniverseCell{{Primary: "A", Related: "A", Count: 90, Ratio: 1}, {Primary: "A", Related: "B", Count: 10, Ratio: 0.1}, {Primary: "B", Related: "A", Count: 20, Ratio: 0.2}, {Primary: "B", Related: "B", Count: 180, Ratio: 1}},
	}
}

func TestUniverseIndependentDatesAndOrdering(t *testing.T) {
	s := fixtureUniverse()
	out, err := buildUniverse(s)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "complete" || len(out.Warnings) != 0 {
		t.Fatalf("complete status: %q, warnings=%+v", out.Status, out.Warnings)
	}
	if out.AsOfDate.Format("2006-01-02") != "2026-04-01" || out.AsOfDates.Group["member_matrix_activity"] == nil || out.AsOfDates.Group["member_matrix_activity"].Format("2006-01-02") != "2026-03-28" || out.AsOfDates.Opcos["A"].Format("2006-01-02") != "2026-03-25" {
		t.Fatalf("dates: %+v", out.AsOfDates)
	}
	if len(out.Scopes) != 3 || out.Scopes[0].Code != "B" || out.Group.MemberMatrixActivity.Columns[0] != "B" {
		t.Fatalf("scope order: %+v", out.Scopes)
	}
	if out.Group.MemberMatrixActivity.Rows[0].PrimaryEntity.Count == nil || *out.Group.MemberMatrixActivity.Rows[0].PrimaryEntity.Count != 180 {
		t.Fatal("matrix total must come from matrix date")
	}
	if out.Group.AnnualMemberAcquisition[0].Label != "2025" || out.Group.LatestMemberGrowth.Growth == nil || *out.Group.LatestMemberGrowth.Growth != -0.5 {
		t.Fatalf("acquisition: %+v", out.Group)
	}
	if out.Group.EntityMemberCount[1].Percent == nil || *out.Group.EntityMemberCount[1].Percent != 0 || out.AsOfDates.Opcos["X"] != nil {
		t.Fatal("zero or disabled metadata lost")
	}
}

func TestUniverseIncompleteLatestMatrixReturnsPartial(t *testing.T) {
	s := fixtureUniverse()
	s.Cells = s.Cells[:3]
	out, err := buildUniverse(s)
	if err != nil || out.Status != "partial" || out.Group.MemberMatrixActivity != nil || out.AsOfDates.Group["member_matrix_activity"] != nil {
		t.Fatalf("expected partial response with unavailable matrix: %+v, %v", out, err)
	}
	if len(out.Warnings) != 1 || out.Warnings[0].Chart != "member_matrix_activity" || out.Warnings[0].Code != "CHART_DATA_UNAVAILABLE" {
		t.Fatalf("matrix warning: %+v", out.Warnings)
	}
}

func TestUniverseMatrixIncludesUnavailableOpco(t *testing.T) {
	s := fixtureUniverse()
	s.MatrixTotals["X"] = u64(10)
	s.Cells = append(s.Cells,
		domain.UniverseCell{Primary: "B", Related: "X", Count: 2, Ratio: 0.01},
		domain.UniverseCell{Primary: "A", Related: "X", Count: 1, Ratio: 0.01},
		domain.UniverseCell{Primary: "X", Related: "B", Count: 2, Ratio: 0.2},
		domain.UniverseCell{Primary: "X", Related: "A", Count: 1, Ratio: 0.1},
		domain.UniverseCell{Primary: "X", Related: "X", Count: 10, Ratio: 1},
	)
	out, err := buildUniverse(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Group.MemberMatrixActivity.Columns) != 3 || out.Group.MemberMatrixActivity.Columns[2] != "X" || len(out.Group.MemberMatrixActivity.Rows) != 3 || out.Opcos[2].IsEnabled {
		t.Fatalf("unavailable scope missing from matrix or enabled for detail: %+v", out)
	}
}

func TestUniverseRetailOnlyOpcoSummaryAndMatrixTotal(t *testing.T) {
	s := fixtureUniverse()
	s.Summaries[s.GroupCode] = domain.UniverseSummary{Date: ud("2026-08-12"), TotalMembers: u64(4380000), ActiveMembers: u64(963600), InactiveMembers: u64(3416400), TotalRetail: u64(6701700), RetailMembers: u64(901700), RetailNonMembers: u64(5800000)}
	s.Summaries["A"] = domain.UniverseSummary{Date: ud("2026-08-12"), TotalRetail: u64(948200), RetailMembers: u64(95200), RetailNonMembers: u64(853000)}
	s.MatrixTotals["A"] = u64(215372)
	out, err := buildUniverse(s)
	if err != nil {
		t.Fatal(err)
	}
	var card *domain.UniverseViewOpco
	for i := range out.Opcos {
		if out.Opcos[i].ScopeCode == "A" {
			card = &out.Opcos[i]
		}
	}
	if card == nil || card.TotalRetailCount == nil || *card.TotalRetailCount != 948200 || card.Member == nil || card.Member.Count == nil || *card.Member.Count != 95200 || card.NonMember == nil || *card.NonMember.Count != 853000 {
		t.Fatalf("retail-only card: %+v", card)
	}
	if out.Group.TotalMembers == nil || *out.Group.TotalMembers.Count != 4380000 || out.Group.MemberActivity == nil || *out.Group.MemberActivity.Active.Count != 963600 || *out.Group.MemberActivity.Inactive.Count != 3416400 || card.Member.Percent == nil || *card.Member.Percent != 0.100401 {
		t.Fatalf("summary mapping or retail ratio: group=%+v card=%+v", out.Group, card)
	}
	if out.Group.MemberMatrixActivity == nil || *out.Group.MemberMatrixActivity.Rows[1].PrimaryEntity.Count != 215372 {
		t.Fatalf("matrix total did not come from matrix source: %+v", out.Group.MemberMatrixActivity)
	}
}

func TestUniverseUnavailableOpcoDoesNotHideOthers(t *testing.T) {
	s := fixtureUniverse()
	delete(s.Summaries, "A")
	out, err := buildUniverse(s)
	if err != nil || out.Status != "partial" {
		t.Fatalf("expected partial response: %+v, %v", out, err)
	}
	if len(out.Warnings) != 1 || out.Warnings[0].Chart != "opco_retail" || out.Warnings[0].ScopeCode != "A" {
		t.Fatalf("scoped warning: %+v", out.Warnings)
	}
	if out.Opcos[1].Member != nil || out.Opcos[0].Member == nil || out.Group.MemberMatrixActivity == nil {
		t.Fatalf("card isolation failed: %+v", out.Opcos)
	}
}

func TestUniverseAllChartsUnavailableFails(t *testing.T) {
	s := fixtureUniverse()
	s.Summaries = map[string]domain.UniverseSummary{}
	s.AcquisitionDate, s.DistributionDate, s.MatrixDate = time.Time{}, time.Time{}, time.Time{}
	s.Acquisition, s.Buckets, s.Cells, s.MatrixTotals = nil, nil, nil, nil
	_, err := buildUniverse(s)
	if !errors.Is(err, domain.ErrUniverseUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}

func TestUniverseAllChartsInvalidFails(t *testing.T) {
	s := fixtureUniverse()
	bad := int64(-1)
	s.Summaries[s.GroupCode] = domain.UniverseSummary{Date: ud("2026-03-31"), TotalMembers: &bad, ActiveMembers: &bad, InactiveMembers: &bad, TotalRetail: &bad, RetailMembers: &bad, RetailNonMembers: &bad}
	for _, code := range []string{"A", "B"} {
		s.Summaries[code] = domain.UniverseSummary{Date: ud("2026-03-31"), TotalRetail: &bad, RetailMembers: &bad, RetailNonMembers: &bad}
	}
	s.Acquisition[0].NewMembers = -1
	s.Buckets[0].Count = &bad
	s.Cells[0].Ratio = 2
	_, err := buildUniverse(s)
	if !errors.Is(err, domain.ErrUniverseInvalidData) {
		t.Fatalf("expected invalid-data precedence, got %v", err)
	}
}

func TestUniverseDatabaseFailureRemainsFatal(t *testing.T) {
	repo := &universeFakeRepo{err: errors.New("database down")}
	_, err := NewUniverseService(repo).Query(context.Background(), domain.UniverseQuery{Permissions: []string{UniversePermission}})
	if !errors.Is(err, domain.ErrUniverseDatabase) {
		t.Fatalf("expected database error, got %v", err)
	}
}

func TestUniverseMatrixDimensionsIgnoreDetailAvailability(t *testing.T) {
	s := fixtureUniverse()
	s.Scopes[0].Enabled = false
	out, err := buildUniverse(s)
	if err != nil {
		t.Fatal(err)
	}
	matrix := out.Group.MemberMatrixActivity
	if len(matrix.Columns) != 2 || matrix.Columns[0] != "B" || len(matrix.Rows) != 2 || matrix.Rows[0].PrimaryEntity.Label != "B" {
		t.Fatalf("matrix changed with detail availability: %+v", matrix)
	}
	if out.AsOfDates.Opcos["B"] != nil || out.Opcos[0].IsEnabled {
		t.Fatalf("disabled card: %+v", out.Opcos[0])
	}
}

func TestUniverseMatrixWorksWithAllDetailsDisabled(t *testing.T) {
	s := fixtureUniverse()
	for i := range s.Scopes {
		s.Scopes[i].Enabled = false
	}
	out, err := buildUniverse(s)
	if err != nil || out.Group.MemberMatrixActivity == nil || len(out.Group.MemberMatrixActivity.Columns) != 2 {
		t.Fatalf("group matrix depends on detail availability: %+v, %v", out, err)
	}
	for _, card := range out.Opcos {
		if card.IsEnabled || card.TotalRetailCount != nil {
			t.Fatalf("disabled detail card was enabled: %+v", card)
		}
	}
}

func TestUniverseDisabledMatrixFeatureHasNoWarning(t *testing.T) {
	s := fixtureUniverse()
	s.MatrixEnabled = false
	out, err := buildUniverse(s)
	if err != nil || out.Group.MemberMatrixActivity != nil || out.AsOfDates.Group["member_matrix_activity"] != nil || out.Status != "complete" || len(out.Warnings) != 0 {
		t.Fatalf("disabled group matrix feature: %+v, %v", out, err)
	}
}

func TestUniverseMatrixCellWithoutTotalReturnsUnavailable(t *testing.T) {
	s := fixtureUniverse()
	s.Cells = append(s.Cells, domain.UniverseCell{Primary: "X", Related: "A", Count: 1, Ratio: 0.1})
	out, err := buildUniverse(s)
	if err != nil || out.Status != "partial" || out.Group.MemberMatrixActivity != nil || len(out.Warnings) != 1 || out.Warnings[0].Code != "CHART_DATA_UNAVAILABLE" {
		t.Fatalf("incomplete matrix should be unavailable: %+v, %v", out, err)
	}
}

func TestUniverseMissingGroupConfigurationFails(t *testing.T) {
	s := fixtureUniverse()
	s.GroupConfigured = false
	_, err := buildUniverse(s)
	if !errors.Is(err, domain.ErrUniverseInvalidData) {
		t.Fatalf("expected 500, got %v", err)
	}
}

func TestUniverseUsesConfiguredGroupCode(t *testing.T) {
	s := fixtureUniverse()
	s.GroupCode = "GROUP_FROM_DB"
	s.Summaries[s.GroupCode] = s.Summaries["AEON_GROUP"]
	delete(s.Summaries, "AEON_GROUP")
	if _, err := buildUniverse(s); err != nil {
		t.Fatal(err)
	}
}

func TestUniversePermissionBeforeRead(t *testing.T) {
	repo := &universeFakeRepo{snap: fixtureUniverse()}
	svc := NewUniverseService(repo)
	_, err := svc.Query(context.Background(), domain.UniverseQuery{})
	if !errors.Is(err, domain.ErrUniverseForbidden) || repo.calls != 0 {
		t.Fatalf("expected 403 before read, got %v", err)
	}
	_, err = svc.Query(context.Background(), domain.UniverseQuery{Permissions: []string{"cdphub.customer_intelligence.universe.view"}})
	if !errors.Is(err, domain.ErrUniverseForbidden) || repo.calls != 0 {
		t.Fatalf("expected old permission to be denied before read, got %v", err)
	}
	q := domain.UniverseQuery{Permissions: []string{"cdphub.cust_intel.view"}}
	q.StartDate = "2026-01-01"
	_, err = svc.Query(context.Background(), q)
	if !errors.Is(err, domain.ErrUniverseDateUnsupported) || repo.calls != 0 {
		t.Fatalf("expected 400 before read, got %v", err)
	}
	q.StartDate = ""
	_, err = svc.Query(context.Background(), q)
	if err != nil || repo.calls != 1 {
		t.Fatalf("expected successful read, got %v", err)
	}
}
