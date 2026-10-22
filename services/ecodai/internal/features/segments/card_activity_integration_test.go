package segments_test

import (
	"context"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/postgres"
	"os"
	"testing"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
)

// Run against an empty disposable database migrated through 000004.
func TestCardActivityLocalSeedAndFilters(t *testing.T) {
	dsn := os.Getenv("SEGMENT_CARD_TEST_DSN")
	if dsn == "" {
		t.Skip("set SEGMENT_CARD_TEST_DSN for the disposable card fixture database")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	fixture, err := os.ReadFile("testdata/card_activity_local.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(fixture)); err != nil {
		t.Fatal(err)
	}
	db := &database.PostgresDB{Pool: pool}
	if err := postgres.NewBuildPublisher(db).Publish(ctx, "CARD_PLAY_01"); err != nil {
		t.Fatal(err)
	}
	var raw, consented int64
	if err := pool.QueryRow(ctx, `SELECT raw_count,build_consented_count
		FROM segment_build_count_v0 WHERE build_id='CARD_PLAY_01' AND segment_id='seg_000001'`).
		Scan(&raw, &consented); err != nil {
		t.Fatal(err)
	}
	if raw != 90 || consented != 80 {
		t.Fatalf("predefined count raw=%d consented=%d, want 90/80", raw, consented)
	}
	evaluator := postgres.NewBuildEvaluator(db)
	for _, tc := range []struct {
		name    string
		filters []model.SegmentFilter
		want    int64
	}{
		{name: "base", want: 80},
		{name: "income 100k", filters: []model.SegmentFilter{{Key: "annual_income", Values: float64(100000)}}, want: 40},
		{name: "race malay", filters: []model.SegmentFilter{{Key: "race", Values: []any{"malay"}}}, want: 40},
		{name: "income and race", filters: []model.SegmentFilter{
			{Key: "annual_income", Values: float64(100000)},
			{Key: "race", Values: []any{"malay"}},
		}, want: 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluator.EstimateBuild(ctx, model.RequestInput{Base: "all_members",
				Segments: []model.SegmentSelection{{ID: "seg_000001", Filters: tc.filters}}})
			if err != nil {
				t.Fatal(err)
			}
			if result.Count != tc.want || result.BuildID != "CARD_PLAY_01" {
				t.Fatalf("estimate=%+v, want %d", result, tc.want)
			}
		})
	}
	var bitmapCardinality, dashboardCount int64
	if err := pool.QueryRow(ctx, `SELECT cardinality FROM member_bitmap_v0
		WHERE build_id='CARD_PLAY_01' AND predicate_kind='predefined' AND predicate_id='seg_000001'`).
		Scan(&bitmapCardinality); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT value FROM dashboard_result_v0
		WHERE build_id='CARD_PLAY_01' AND org_id='ALL' AND retail_scope='ALL'
		AND metric_id='member_count' AND dimension_key='ALL'`).Scan(&dashboardCount); err != nil {
		t.Fatal(err)
	}
	if bitmapCardinality != 90 || dashboardCount != 100 {
		t.Fatalf("bitmap=%d dashboard=%d, want 90/100", bitmapCardinality, dashboardCount)
	}
}
