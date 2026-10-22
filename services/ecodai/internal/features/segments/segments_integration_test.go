package segments_test

import (
	"context"
	"errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/postgres"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/service"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/worker"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
)

type failingExporter struct{ calls int }

const expectedRetryAttempts = 3

func (e *failingExporter) Upload(context.Context, string, string) (worker.ExportedObject, error) {
	e.calls++
	return worker.ExportedObject{}, errors.New("simulated object finalization failure")
}

func TestLocalExportFailureRetriesAndStops(t *testing.T) {
	dsn := os.Getenv("SEGMENT_TEST_DSN")
	if dsn == "" {
		t.Skip("set SEGMENT_TEST_DSN for the local PostgreSQL integration test")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	db := &database.PostgresDB{Pool: pool}
	svc := service.NewService(postgres.NewRepository(db), postgres.NewEvaluator(db), true)
	_, req, err := svc.Request(ctx, model.RequestInput{Mode: "confirm", Segments: []model.SegmentSelection{{ID: "seg_000001"}}}, uuid.NewString(), uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM segment_requests_v0 WHERE id=$1`, req.ID)
	})
	exporter := &failingExporter{}
	jobWorker := worker.NewWorker(db, exporter)
	for attempt := 1; attempt <= expectedRetryAttempts; attempt++ {
		processed, err := jobWorker.ProcessNext(ctx)
		if !processed || err == nil {
			t.Fatalf("attempt %d: processed=%v err=%v", attempt, processed, err)
		}
		status, err := svc.GetRequest(ctx, req.RequestID)
		if err != nil {
			t.Fatal(err)
		}
		var jobStatus string
		var attempts int
		if err := pool.QueryRow(ctx, `SELECT status,attempt_count FROM segment_request_jobs_v0 WHERE segment_request_id=$1`, req.ID).Scan(&jobStatus, &attempts); err != nil {
			t.Fatal(err)
		}
		if attempts != attempt || status.Bucket != nil || status.Progress.FilesCompleted != 0 || status.Progress.BytesUploaded != 0 {
			t.Fatalf("attempt %d leaked completed export: request=%+v job=%s/%d", attempt, status, jobStatus, attempts)
		}
		if attempt < expectedRetryAttempts {
			if status.Status != "INIT" || status.Progress.Stage != "QUEUED" || jobStatus != "PENDING" {
				t.Fatalf("attempt %d did not queue retry: request=%+v job=%s", attempt, status, jobStatus)
			}
			if _, err := pool.Exec(ctx, `UPDATE segment_request_jobs_v0 SET next_run_at=now() WHERE segment_request_id=$1`, req.ID); err != nil {
				t.Fatal(err)
			}
		} else if status.Status != "FAILED" || status.Progress.Stage != "FAILED" || status.ErrorCode == nil || *status.ErrorCode != "CONFIRM_FAILED" || jobStatus != "FAILED" {
			t.Fatalf("terminal failure not persisted: request=%+v job=%s", status, jobStatus)
		}
	}
	if exporter.calls != expectedRetryAttempts {
		t.Fatalf("upload attempts = %d, want %d", exporter.calls, expectedRetryAttempts)
	}
}

// Run after applying migrations and testdata/local.sql with SEGMENT_TEST_DSN set.
func TestLocalEstimateConfirmAndExport(t *testing.T) {
	dsn := os.Getenv("SEGMENT_TEST_DSN")
	if dsn == "" {
		t.Skip("set SEGMENT_TEST_DSN for the local PostgreSQL integration test")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	db := &database.PostgresDB{Pool: pool}
	repo := postgres.NewRepository(db)
	svc := service.NewService(repo, postgres.NewEvaluator(db), true)
	in := model.RequestInput{Mode: "estimate", Segments: []model.SegmentSelection{{ID: "seg_000001", Operation: "AND"}, {ID: "seg_000002"}}}
	estimate, _, err := svc.Request(ctx, in, "local-test", "")
	if err != nil {
		t.Fatal(err)
	}
	if estimate.Count != 1 {
		t.Fatalf("AND estimate = %d, want 1", estimate.Count)
	}
	in.Segments[0].Operation = "OR"
	estimate, _, err = svc.Request(ctx, in, "local-test", "")
	if err != nil {
		t.Fatal(err)
	}
	if estimate.Count != 3 {
		t.Fatalf("OR estimate = %d, want 3", estimate.Count)
	}
	in.Segments = []model.SegmentSelection{{ID: "seg_000002", Filters: []model.SegmentFilter{{Key: "annual_income", Values: float64(100000)}}}}
	estimate, _, err = svc.Request(ctx, in, "local-test", "")
	if err != nil {
		t.Fatal(err)
	}
	if estimate.Count != 1 {
		t.Fatalf("numeric filter estimate = %d, want 1", estimate.Count)
	}
	in.Mode = "confirm"
	in.Segments = []model.SegmentSelection{{ID: "seg_000001"}}
	requester := uuid.NewString()
	key := uuid.NewString()
	_, req, err := svc.Request(ctx, in, requester, key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM segment_usage_v0 WHERE recipe_id IN (SELECT id FROM recipes_v0 WHERE segment_request_id=$1)`, req.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM segment_requests_v0 WHERE id=$1`, req.ID)
	})
	_, repeat, err := svc.Request(ctx, in, requester, key)
	if err != nil {
		t.Fatal(err)
	}
	if repeat.RequestID != req.RequestID {
		t.Fatalf("idempotent request changed: %s != %s", repeat.RequestID, req.RequestID)
	}
	exporter := worker.LocalExporter{Root: t.TempDir()}
	jobWorker := worker.NewWorker(db, exporter)
	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	done, err := jobWorker.ProcessNext(runCtx)
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("no job claimed")
	}
	ready, err := svc.GetRequest(ctx, req.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Status != "READY" || ready.Progress.Stage != "COMPLETE" || ready.Progress.FilesCompleted != 1 {
		t.Fatalf("unexpected final request: %+v", ready)
	}
	if *ready.TotalCount != 3 || *ready.ConsentCount != 2 {
		t.Fatalf("counts: total=%d consent=%d", *ready.TotalCount, *ready.ConsentCount)
	}
	path := filepath.Join(exporter.Root, "segments-v0", req.RequestID, "members.csv")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "member_id\n11111111111141118111111111111111\n22222222222242228222222222222222\n"
	if string(content) != want {
		t.Fatalf("CSV = %q", content)
	}
	if ready.Bucket == nil || !strings.HasPrefix(*ready.Bucket, "file://") || ready.Progress.BytesUploaded != int64(len(content)) {
		t.Fatalf("bad local object reference: %+v", ready)
	}
	custom, err := svc.CreateSegment(ctx, model.CreateSegmentInput{Name: "local empty " + uuid.NewString(), Filters: []model.SegmentFilter{{Key: "cc_active_flag", Operation: "=", Values: []any{true, false}}}}, requester)
	if err != nil {
		t.Fatal(err)
	}
	if len(custom.Tags) != 1 || custom.Tags[0] != "custom" {
		t.Fatalf("custom segment category tags: %v", custom.Tags)
	}
	if custom.Kind != "custom" || custom.DefinitionStatus != "draft" || custom.EstimatedMemberCount != nil || custom.CountSource != nil {
		t.Fatalf("custom draft has an evaluated count or wrong classification: %+v", custom)
	}
	for _, reserved := range []string{"predefine", "custom", "attribute"} {
		_, err := svc.CreateSegment(ctx, model.CreateSegmentInput{Name: "invalid category", Tags: []string{reserved}, Filters: []model.SegmentFilter{{Key: "cc_active_flag", Operation: "=", Values: []any{true}}}}, requester)
		if err == nil {
			t.Fatalf("accepted user-supplied reserved tag %q", reserved)
		}
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM segments_v0 WHERE id=$1`, custom.ID) })
	_, empty, err := svc.Request(ctx, model.RequestInput{Mode: "confirm", Segments: []model.SegmentSelection{{ID: custom.ID}}}, requester, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM segment_usage_v0 WHERE recipe_id IN (SELECT id FROM recipes_v0 WHERE segment_request_id=$1)`, empty.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM segment_requests_v0 WHERE id=$1`, empty.ID)
	})
	done, err = jobWorker.ProcessNext(runCtx)
	if err != nil || !done {
		t.Fatalf("empty custom export: done=%v err=%v", done, err)
	}
	emptyReady, err := svc.GetRequest(ctx, empty.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if emptyReady.Status != "READY" || *emptyReady.TotalCount != 0 || *emptyReady.ConsentCount != 0 {
		t.Fatalf("empty custom result: %+v", emptyReady)
	}
	_, other, err := svc.Request(ctx, model.RequestInput{Mode: "confirm", Segments: []model.SegmentSelection{{ID: "seg_000001"}}}, uuid.NewString(), key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM segment_usage_v0 WHERE recipe_id IN (SELECT id FROM recipes_v0 WHERE segment_request_id=$1)`, other.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM segment_requests_v0 WHERE id=$1`, other.ID)
	})
	if other.RequestID == req.RequestID {
		t.Fatal("idempotency key leaked across requesters")
	}
	_, err = pool.Exec(ctx, `UPDATE segment_request_jobs_v0 SET status='PROCESSING',attempt_count=1,locked_by='crashed-worker',lease_until=now()-interval '1 minute' WHERE segment_request_id=$1`, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `UPDATE segment_requests_v0 SET status='PROCESSING',progress_stage='UPLOADING' WHERE id=$1`, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	done, err = jobWorker.ProcessNext(runCtx)
	if err != nil || !done {
		t.Fatalf("stale job recovery: done=%v err=%v", done, err)
	}
	recovered, err := svc.GetRequest(ctx, other.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != "READY" {
		t.Fatalf("recovered request: %+v", recovered)
	}
}
