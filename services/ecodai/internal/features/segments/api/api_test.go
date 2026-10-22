package api

import (
	"context"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/response"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/service"
)

type routeRepo struct{}

func (routeRepo) List(context.Context, model.ListFilter) ([]model.Segment, int64, error) {
	return []model.Segment{}, 0, nil
}
func (routeRepo) Get(context.Context, string) (*model.Segment, error) { return nil, pgx.ErrNoRows }
func (routeRepo) CreateSegment(context.Context, model.CreateSegmentInput, string) (*model.Segment, error) {
	return nil, nil
}
func (routeRepo) CreateConfirm(context.Context, model.RequestInput, string, string) (*model.Request, error) {
	return nil, nil
}
func (routeRepo) GetRequest(_ context.Context, id string) (*model.Request, error) {
	if id != "known" {
		return nil, pgx.ErrNoRows
	}
	return &model.Request{RequestID: id, Status: "INIT", Progress: model.Progress{Stage: "QUEUED", FilesTotal: 1}}, nil
}

type routeAuth struct{}

func (routeAuth) Authenticate(_ context.Context, token string) (auth.Identity, error) {
	if token == "valid" {
		return auth.Identity{Subject: "user", Permissions: []string{"cdphub.cust_intel.view"}}, nil
	}
	if token == "limited" {
		return auth.Identity{Subject: "user"}, nil
	}
	return auth.Identity{}, auth.ErrInvalidToken
}
func TestStatusRequiresIdentity(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = response.ErrorHandler
	if err := RegisterRoutes(e, routeAuth{}, nil, &Handler{Service: service.NewService(routeRepo{}, nil, false)}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		authorization string
		status        int
	}{
		{"", 401}, {"Bearer invalid", 401}, {"Bearer limited", 403}, {"Bearer valid", 200},
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v0/segments/requests/known", nil)
		if tc.authorization != "" {
			req.Header.Set("Authorization", tc.authorization)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("polling %q: %d %s", tc.authorization, rec.Code, rec.Body.String())
		}
		if tc.status == 200 && !strings.Contains(rec.Body.String(), `"stage":"QUEUED"`) {
			t.Fatalf("missing progress: %s", rec.Body.String())
		}
	}
	unknown := httptest.NewRecorder()
	unknownRequest := httptest.NewRequest(http.MethodGet, "/api/v0/segments/requests/absent", nil)
	unknownRequest.Header.Set("Authorization", "Bearer valid")
	e.ServeHTTP(unknown, unknownRequest)
	if unknown.Code != 404 {
		t.Fatalf("unknown request status %d", unknown.Code)
	}
	for _, route := range []struct{ method, path string }{{http.MethodGet, "/api/v0/segments/list"}, {http.MethodGet, "/api/v0/segments/seg_000001"}, {http.MethodPost, "/api/v0/segments"}, {http.MethodPost, "/api/v0/segments/requests"}} {
		protected := httptest.NewRecorder()
		e.ServeHTTP(protected, httptest.NewRequest(route.method, route.path, nil))
		if protected.Code != 401 {
			t.Fatalf("unauthed %s %s status %d", route.method, route.path, protected.Code)
		}
		limited := httptest.NewRequest(route.method, route.path, nil)
		limited.Header.Set("Authorization", "Bearer limited")
		denied := httptest.NewRecorder()
		e.ServeHTTP(denied, limited)
		if denied.Code != 403 {
			t.Fatalf("ungranted %s %s status %d", route.method, route.path, denied.Code)
		}
	}
	allowed := httptest.NewRequest(http.MethodGet, "/api/v0/segments/list", nil)
	allowed.Header.Set("Authorization", "Bearer valid")
	got := httptest.NewRecorder()
	e.ServeHTTP(got, allowed)
	if got.Code != 200 {
		t.Fatalf("allowed list status %d: %s", got.Code, got.Body.String())
	}
}

func TestSegmentRouteValidationAndEnvelope(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = response.ErrorHandler
	if err := RegisterRoutes(e, routeAuth{}, nil, &Handler{Service: service.NewService(routeRepo{}, nil, false)}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/v0/segments/list?page=0", ""},
		{http.MethodGet, "/api/v0/segments/list?pageSize=101", ""},
		{http.MethodGet, "/api/v0/segments/list?page=abc", ""},
		{http.MethodPost, "/api/v0/segments", `{"name":"x","filters":[],"unexpected":true}`},
		{http.MethodPost, "/api/v0/segments", `{"name":"x","filters":[]} {}`},
		{http.MethodPost, "/api/v0/segments/requests", `{"mode":"estimate","segments":[{"id":"seg_000001","filters":[{"key":"","values":[1]}]}]}`},
		{http.MethodPost, "/api/v0/segments/requests", `{"mode":"confirm","segments":[],"unexpected":true}`},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer valid")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s %s body=%s: status %d, response %s", tc.method, tc.path, tc.body, rec.Code, rec.Body.String())
		}
	}
	list := httptest.NewRequest(http.MethodGet, "/api/v0/segments/list", nil)
	list.Header.Set("Authorization", "Bearer valid")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, list)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"page":1`) || !strings.Contains(rec.Body.String(), `"pageSize":20`) || !strings.Contains(rec.Body.String(), `"segments":[]`) {
		t.Fatalf("default list envelope: %d %s", rec.Code, rec.Body.String())
	}
}
