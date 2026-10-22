package segments_test

import (
	"context"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/postgres"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/service"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/worker"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
)

// Use a fresh disposable database migrated through 000004.
func TestPublishVersionedBuild(t *testing.T) {
	dsn := os.Getenv("SEGMENT_BUILD_TEST_DSN")
	if dsn == "" {
		t.Skip("set SEGMENT_BUILD_TEST_DSN for the disposable build database")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	fixture, err := os.ReadFile("testdata/build_b21.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(fixture)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO segment_count_patch_v0
		(segment_id,member_count,population_basis,as_of_date,source_ref,reviewed_by)
		VALUES ('seg_100016',99,'patched_reference','2026-10-20','fixture','test')`); err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewBuildRepository(&database.PostgresDB{Pool: pool})
	before, err := repo.Get(ctx, "seg_100016")
	if err != nil || before.EstimatedMemberCount == nil || *before.EstimatedMemberCount != 99 ||
		before.CountSource == nil || *before.CountSource != "patched" {
		t.Fatalf("patched listing before publication: %+v, %v", before, err)
	}
	publisher := postgres.NewBuildPublisher(&database.PostgresDB{Pool: pool})
	if err := publisher.Publish(ctx, "B21"); err != nil {
		t.Fatal(err)
	}
	after, err := repo.Get(ctx, "seg_100016")
	if err != nil || after.EstimatedMemberCount == nil || *after.EstimatedMemberCount != 1 ||
		after.CountSource == nil || *after.CountSource != "computed" ||
		after.CountBuildID == nil || *after.CountBuildID != "B21" {
		t.Fatalf("computed listing after publication: %+v, %v", after, err)
	}
	var current, status string
	if err := pool.QueryRow(ctx, `SELECT c.build_id,b.status FROM segment_current_build_v0 c
		JOIN segment_build_v0 b ON b.build_id=c.build_id`).Scan(&current, &status); err != nil {
		t.Fatal(err)
	}
	if current != "B21" || status != "READY" {
		t.Fatalf("current build=%s status=%s", current, status)
	}
	if _, err := pool.Exec(ctx, `UPDATE member_facts_v0 SET build_consent=FALSE
		WHERE build_id='B21' AND member_id='M001'`); err == nil {
		t.Fatal("READY source rows must be immutable")
	}
	if _, err := pool.Exec(ctx, `UPDATE segment_build_v0 SET required_outputs=ARRAY['source']
		WHERE build_id='B21'`); err == nil {
		t.Fatal("READY manifest must be immutable")
	}
	var raw, consented int64
	if err := pool.QueryRow(ctx, `SELECT raw_count,build_consented_count FROM segment_build_count_v0
		WHERE build_id='B21' AND segment_id='seg_100016' AND org_id='ALL'`).Scan(&raw, &consented); err != nil {
		t.Fatal(err)
	}
	if raw != 2 || consented != 1 {
		t.Fatalf("Female count raw=%d consented=%d", raw, consented)
	}
	var ageID string
	if err := pool.QueryRow(ctx, `SELECT age_option_id FROM member_facts_v0
		WHERE build_id='B21' AND member_id='M001'`).Scan(&ageID); err != nil {
		t.Fatal(err)
	}
	if ageID != "seg_100004" {
		t.Fatalf("M001 age option %s", ageID)
	}
	estimate, err := postgres.NewBuildEvaluator(&database.PostgresDB{Pool: pool}).EstimateBuild(ctx,
		model.RequestInput{Base: "all_members", Segments: []model.SegmentSelection{
			{ID: "seg_100016", Operation: "AND"}, {ID: "seg_100004"},
		}})
	if err != nil {
		t.Fatal(err)
	}
	if estimate.Count != 1 || estimate.BuildID != "B21" || estimate.SourceDate != "2026-10-21" {
		t.Fatalf("wrong pinned estimate: %+v", estimate)
	}
	db := &database.PostgresDB{Pool: pool}
	service := service.NewService(postgres.NewBuildRepository(db), postgres.NewBuildEvaluator(db), true)
	_, request, err := service.Request(ctx, model.RequestInput{Mode: "confirm", Base: "all_members",
		Segments: []model.SegmentSelection{{ID: "seg_100016", Operation: "AND"}, {ID: "seg_100004"}}},
		"local-build-test", "build-request-1")
	if err != nil {
		t.Fatal(err)
	}
	if request.BuildID == nil || *request.BuildID != "B21" {
		t.Fatalf("confirmation was not pinned: %+v", request)
	}
	exportRoot := t.TempDir()
	processed, err := worker.NewWorker(db, worker.LocalExporter{Root: exportRoot}).ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("export processed=%v: %v", processed, err)
	}
	ready, err := service.GetRequest(ctx, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Status != "READY" || ready.TotalCount == nil || *ready.TotalCount != 1 ||
		ready.BuildConsentCount == nil || *ready.BuildConsentCount != 1 ||
		ready.ConsentCount == nil || *ready.ConsentCount != 1 || ready.ConsentCheckedAt == nil {
		t.Fatalf("wrong versioned export counts: %+v", ready)
	}
	file, err := os.ReadFile(filepath.Join(exportRoot, "segments-v0", request.RequestID, "members.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if string(file) != "member_id\nM001\n" {
		t.Fatalf("versioned export = %q", file)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO segment_build_v0
		(build_id,source_date,status,required_outputs,source_member_count)
		VALUES ('B22','2026-10-21','LOADING',ARRAY['source','sql_counts'],3)`); err != nil {
		t.Fatal(err)
	}
	if err := publisher.Publish(ctx, "B22"); err == nil {
		t.Fatal("incomplete B22 published")
	}
	if err := pool.QueryRow(ctx, `SELECT build_id FROM segment_current_build_v0`).Scan(&current); err != nil {
		t.Fatal(err)
	}
	if current != "B21" {
		t.Fatalf("failed B22 displaced B21: %s", current)
	}
}
