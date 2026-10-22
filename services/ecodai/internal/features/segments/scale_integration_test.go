package segments_test

import (
	"bufio"
	"context"
	"encoding/json"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/postgres"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/service"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/worker"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/server"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
	segmentsapi "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/api"
)

type scaleAuth struct{}

func (scaleAuth) Authenticate(_ context.Context, token string) (auth.Identity, error) {
	if token != "valid" {
		return auth.Identity{}, auth.ErrInvalidToken
	}
	return auth.Identity{Subject: "scale-test", Permissions: []string{"cdphub.cust_intel.view"}}, nil
}

// Run only after scale_1m.sql with SEGMENT_SCALE_DSN set. The request and
// exported file are removed; the million-member fixture remains available.
func TestMillionMemberLocalExport(t *testing.T) {
	dsn := os.Getenv("SEGMENT_SCALE_DSN")
	if dsn == "" {
		t.Skip("set SEGMENT_SCALE_DSN after loading scale_1m.sql")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	db := &database.PostgresDB{Pool: pool}
	svc := service.NewService(postgres.NewRepository(db), postgres.NewEvaluator(db), true)
	e := server.New(server.Options{RequestTimeout: 10 * time.Second, BodyLimitBytes: 4096})
	if err = segmentsapi.RegisterRoutes(e, scaleAuth{}, nil, &segmentsapi.Handler{Service: svc}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v0/segments/list?tags=scale", "/api/v0/segments/seg_900001"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer valid")
		recorder := httptest.NewRecorder()
		e.ServeHTTP(recorder, request)
		if recorder.Code != 200 {
			t.Fatalf("GET %s: %d %s", path, recorder.Code, recorder.Body.String())
		}
	}
	create := httptest.NewRequest(http.MethodPost, "/api/v0/segments", strings.NewReader(`{"name":"Scale test custom","filters":[{"key":"cc_active_flag","operation":"=","values":[true,false]}]}`))
	create.Header.Set("Authorization", "Bearer valid")
	create.Header.Set("Content-Type", "application/json")
	createdResponse := httptest.NewRecorder()
	e.ServeHTTP(createdResponse, create)
	if createdResponse.Code != 201 {
		t.Fatalf("create HTTP %d: %s", createdResponse.Code, createdResponse.Body.String())
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err = json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Data.ID == "" {
		t.Fatal("create response missing ID")
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM segments_v0 WHERE id=$1`, created.Data.ID) })
	call := func(body string) (int, []byte, time.Duration) {
		req := httptest.NewRequest(http.MethodPost, "/api/v0/segments/requests", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer valid")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", uuid.NewString())
		rec := httptest.NewRecorder()
		start := time.Now()
		e.ServeHTTP(rec, req)
		return rec.Code, rec.Body.Bytes(), time.Since(start)
	}
	status, body, duration := call(`{"mode":"estimate","segments":[{"id":"seg_900001"}]}`)
	t.Logf("million-member estimate: %s", duration)
	if status != 200 {
		t.Fatalf("estimate HTTP %d: %s", status, body)
	}
	var estimate struct {
		Data struct {
			Count int64 `json:"count"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &estimate); err != nil {
		t.Fatal(err)
	}
	if estimate.Data.Count != 800000 {
		t.Fatalf("estimate count %d, want 800000", estimate.Data.Count)
	}
	status, body, duration = call(`{"mode":"confirm","segments":[{"id":"seg_900001"}]}`)
	t.Logf("million-member confirm: %s", duration)
	if status != 200 {
		t.Fatalf("confirm HTTP %d: %s", status, body)
	}
	var confirmed struct {
		Data struct {
			RequestID string `json:"request_id"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &confirmed); err != nil {
		t.Fatal(err)
	}
	if confirmed.Data.RequestID == "" {
		t.Fatalf("missing request ID: %s", body)
	}
	req, err := svc.GetRequest(ctx, confirmed.Data.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM segment_usage_v0 WHERE recipe_id IN (SELECT id FROM recipes_v0 WHERE segment_request_id=$1)`, req.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM segment_requests_v0 WHERE id=$1`, req.ID)
	})
	exporter := worker.LocalExporter{Root: t.TempDir()}
	jobWorker := worker.NewWorker(db, exporter)
	runCtx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	started := time.Now()
	done, err := jobWorker.ProcessNext(runCtx)
	t.Logf("million-member export worker: %s", time.Since(started))
	if err != nil || !done {
		t.Fatalf("worker done=%v err=%v", done, err)
	}
	ready, err := svc.GetRequest(ctx, confirmed.Data.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Status != "READY" || ready.TotalCount == nil || *ready.TotalCount != 1000000 || ready.ConsentCount == nil || *ready.ConsentCount != 800000 {
		t.Fatalf("unexpected final counts/status: %+v", ready)
	}
	publicStatus := httptest.NewRecorder()
	e.ServeHTTP(publicStatus, httptest.NewRequest(http.MethodGet, "/api/v0/segments/requests/"+confirmed.Data.RequestID, nil))
	if publicStatus.Code != 200 || !strings.Contains(publicStatus.Body.String(), `"status":"READY"`) {
		t.Fatalf("public status HTTP %d: %s", publicStatus.Code, publicStatus.Body.String())
	}
	path := filepath.Join(exporter.Root, "segments-v0", confirmed.Data.RequestID, "members.csv")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if lines != 800001 {
		t.Fatalf("CSV lines %d, want 800001", lines)
	}
}
