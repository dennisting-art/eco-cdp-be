package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
)

// Use a fresh disposable database migrated through 000004.
func TestPublishBitmapBuild(t *testing.T) {
	dsn := os.Getenv("SEGMENT_BITMAP_TEST_DSN")
	if dsn == "" {
		t.Skip("set SEGMENT_BITMAP_TEST_DSN for the disposable bitmap database")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	fixture, err := os.ReadFile("../testdata/build_b21.sql")
	if err != nil {
		t.Fatal(err)
	}
	bitmapFixture := strings.Replace(string(fixture), "ARRAY['source','sql_counts']",
		"ARRAY['source','sql_counts','bitmaps','dashboards']", 1)
	if _, err := pool.Exec(ctx, bitmapFixture); err != nil {
		t.Fatal(err)
	}
	db := &database.PostgresDB{Pool: pool}
	if err := NewBuildPublisher(db).Publish(ctx, "B21"); err != nil {
		t.Fatal(err)
	}
	evaluator := NewBuildEvaluator(db)
	for _, tc := range []struct {
		kind, id string
		want     int64
	}{
		{"option", "seg_100016", 2}, {"option", "seg_100004", 2},
		{"consent", "yes", 2}, {"retail", "big", 2}, {"activity", "active_30d", 2},
	} {
		bitmap, err := evaluator.loadBitmap(ctx, "B21", tc.kind, tc.id, 1)
		if err != nil {
			t.Fatal(err)
		}
		if int64(bitmap.GetCardinality()) != tc.want {
			t.Fatalf("bitmap %s/%s = %d, want %d", tc.kind, tc.id, bitmap.GetCardinality(), tc.want)
		}
	}
	in := model.RequestInput{Base: "all_members", Segments: []model.SegmentSelection{
		{ID: "seg_100016", Operation: "AND"}, {ID: "seg_100004"},
	}}
	bitmapCount, supported, err := evaluator.bitmapCount(ctx, "B21", in)
	if err != nil || !supported || bitmapCount != 1 {
		t.Fatalf("bitmap count=%d supported=%v err=%v", bitmapCount, supported, err)
	}
	query, args, err := evaluator.CandidateSQL(ctx, "B21", sourceDate(t), in)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	if err := pool.QueryRow(ctx, `SELECT candidates.member_id FROM (`+query+`) candidates
		JOIN member_facts_v0 m ON m.build_id=$1 AND m.member_id=candidates.member_id
		WHERE m.build_consent`, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if id != "M001" {
		t.Fatalf("SQL exact member = %s", id)
	}
	for _, tc := range []struct {
		scope string
		want  int64
	}{{"ALL", 2}, {"big", 2}, {"co", 1}, {"bank", 0}, {"big+co", 2}} {
		var count int64
		if err := pool.QueryRow(ctx, `SELECT value FROM dashboard_result_v0
			WHERE build_id='B21' AND org_id='ALL' AND retail_scope=$1
			AND metric_id='member_count' AND dimension_key='ALL'`, tc.scope).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != tc.want {
			t.Fatalf("dashboard scope %s = %d, want %d", tc.scope, count, tc.want)
		}
	}
	var sourceBuild string
	if err := pool.QueryRow(ctx, `SELECT source_build_id FROM customer_monthly_summary_v0
		WHERE month='2026-10-01' AND is_current`).Scan(&sourceBuild); err != nil {
		t.Fatal(err)
	}
	if sourceBuild != "B21" {
		t.Fatalf("monthly revision points to %s", sourceBuild)
	}
}

func sourceDate(t *testing.T) (date time.Time) {
	t.Helper()
	date, err := time.Parse("2006-01-02", "2026-10-21")
	if err != nil {
		t.Fatal(err)
	}
	return date
}
