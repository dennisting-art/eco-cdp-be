package segments_test

import (
	"context"
	"crypto/md5"
	"fmt"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/service"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/worker"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/postgres"
)

// Run after local.sql and attribute_local.sql; scale_1m.sql is optional.
func TestLocalAttributeCatalogueAndExport(t *testing.T) {
	dsn := os.Getenv("SEGMENT_TEST_DSN")
	if dsn == "" {
		t.Skip("set SEGMENT_TEST_DSN after loading the local attribute fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	db := &database.PostgresDB{Pool: pool}
	repo := postgres.NewRepository(db)
	segments, total, err := repo.List(ctx, model.ListFilter{Tags: []string{"attribute"}, Page: 1, PageSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	if total != 27 || len(segments) != 27 {
		t.Fatalf("attribute catalogue has %d of %d entries, want 27", len(segments), total)
	}
	if _, err := repo.Get(ctx, "seg_100000"); err != pgx.ErrNoRows {
		t.Fatalf("internal base is visible: %v", err)
	}
	boundaryID := func(n int) string {
		return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("ecodai-attribute-boundary-%d", n))))
	}
	boundaryChecks := map[string]struct {
		present []int
		absent  []int
	}{
		"seg_100003": {present: []int{1, 14}, absent: []int{13}},
		"seg_100004": {present: []int{13}, absent: []int{1, 14}},
		"seg_100007": {present: []int{5}},
		"seg_100012": {present: []int{6}},
		"seg_100023": {present: []int{2}, absent: []int{3}},
		"seg_100024": {present: []int{3}, absent: []int{2}},
		"seg_100014": {present: []int{3}},
		"seg_100001": {present: []int{2}, absent: []int{1}},
		"seg_100002": {present: []int{1}, absent: []int{2}},
		"seg_100019": {present: []int{1}, absent: []int{2, 3, 4}},
		"seg_100020": {present: []int{2}, absent: []int{1, 3, 4}},
		"seg_100021": {present: []int{3}, absent: []int{1, 2, 4}},
		"seg_100022": {present: []int{4}, absent: []int{1, 2, 3}},
	}
	for _, segment := range segments {
		if segment.Category == nil || *segment.Category == "" || !slices.Contains(segment.Tags, "attribute") {
			t.Fatalf("missing attribute classification: %+v", segment)
		}
		members, err := repo.MatchMembersAt(ctx, model.SegmentSelection{ID: segment.ID}, false, nil)
		if err != nil {
			t.Fatalf("evaluate %s: %v", segment.ID, err)
		}
		if segment.EstimatedMemberCount == nil || segment.CountSource == nil || *segment.CountSource != "patched" || int64(len(members)) != *segment.EstimatedMemberCount {
			t.Fatalf("%s has %d members, patch count %v", segment.ID, len(members), segment.EstimatedMemberCount)
		}
		for _, n := range boundaryChecks[segment.ID].present {
			if !slices.Contains(members, boundaryID(n)) {
				t.Fatalf("%s omitted boundary member %d", segment.ID, n)
			}
		}
		for _, n := range boundaryChecks[segment.ID].absent {
			if slices.Contains(members, boundaryID(n)) {
				t.Fatalf("%s included boundary member %d", segment.ID, n)
			}
		}
	}

	svc := service.NewService(repo, repo, true)
	requester := uuid.NewString()
	_, request, err := svc.Request(ctx, model.RequestInput{Mode: "confirm", Segments: []model.SegmentSelection{{ID: "seg_100012"}}}, requester, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM segment_usage_v0 WHERE recipe_id IN (SELECT id FROM recipes_v0 WHERE segment_request_id=$1)`, request.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM segment_requests_v0 WHERE id=$1`, request.ID)
	})
	worker := worker.NewWorker(db, worker.LocalExporter{Root: t.TempDir()})
	processed, err := worker.ProcessNext(ctx)
	if err != nil || !processed {
		t.Fatalf("attribute export processed=%v: %v", processed, err)
	}
	ready, err := svc.GetRequest(ctx, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Status != "READY" || ready.TotalCount == nil || *ready.TotalCount == 0 || ready.ConsentCount == nil || *ready.ConsentCount > *ready.TotalCount || ready.Bucket == nil || !strings.HasPrefix(*ready.Bucket, "file:///") {
		t.Fatalf("attribute export did not complete correctly: %+v", ready)
	}
}
