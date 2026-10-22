package system

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/api"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/service"
)

type authStub struct {
	identity auth.Identity
	err      error
}

func TestErrorEnvelopeAndTrace(t *testing.T) {
	for _, tc := range []struct {
		name    string
		authErr error
		status  int
		code    string
	}{
		{"invalid token", auth.ErrInvalidToken, http.StatusUnauthorized, "401"},
		{"provider unavailable", errors.New("secret upstream failure"), http.StatusServiceUnavailable, "503"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := New(Dependencies{Auth: authStub{err: tc.authErr}, Cards: api.NewCardsHandler(nil), Universe: api.NewUniverseHandler(nil), DB: dbStub{}})
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v0/internal/access", nil)
			req.Header.Set("Authorization", "Bearer token")
			req.Header.Set("X-Trace-ID", "test-trace")
			r.ServeHTTP(w, req)
			var body struct {
				Success bool   `json:"success"`
				TraceID string `json:"trace_id"`
				Errors  struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"errors"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || body.Success || body.Errors.Code != tc.code || body.TraceID != "test-trace" || w.Header().Get("X-Trace-ID") != "test-trace" {
				t.Fatalf("status=%d body=%s header=%q", w.Code, w.Body.String(), w.Header().Get("X-Trace-ID"))
			}
			if strings.Contains(w.Body.String(), "secret upstream failure") {
				t.Fatal("internal cause leaked")
			}
		})
	}
}

func (s authStub) Authenticate(context.Context, string) (auth.Identity, error) {
	return s.identity, s.err
}

type dbStub struct{}

func (dbStub) Ping(context.Context) error { return nil }

type cardsRepoStub struct{ scope string }

func (s *cardsRepoStub) GetSnapshot(_ context.Context, scope string, _ *model.ReportingRange) (model.CardsSnapshot, error) {
	s.scope = scope
	return model.CardsSnapshot{}, nil
}

func TestEchoRequestParity(t *testing.T) {
	repo := &cardsRepoStub{}
	r, err := New(Dependencies{Auth: authStub{identity: auth.Identity{Permissions: []string{service.CustomerIntelligenceViewPermission}}}, Cards: api.NewCardsHandler(service.NewCardsService(repo)), Universe: api.NewUniverseHandler(nil), DB: dbStub{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, body string
		contentType      bool
		want             int
		scope            string
	}{
		{"malformed JSON", "/api/v0/customer-intelligence/cards", `{"scope_code":`, true, http.StatusBadRequest, ""},
		{"unknown field", "/api/v0/customer-intelligence/cards", `{"unknown":1}`, true, http.StatusBadRequest, ""},
		{"oversized body", "/api/v0/customer-intelligence/cards", strings.Repeat(" ", 4097) + `{}`, true, http.StatusBadRequest, ""},
		{"missing content type", "/api/v0/customer-intelligence/cards", `{}`, false, http.StatusNotFound, "AEON"},
		{"duplicate key", "/api/v0/customer-intelligence/cards", `{"scope_code":"AEON","scope_code":"ABG"}`, true, http.StatusNotFound, "ABG"},
		{"trailing slash", "/api/v0/customer-intelligence/cards/", `{}`, true, http.StatusNotFound, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo.scope = ""
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer token")
			if tc.contentType {
				req.Header.Set("Content-Type", "application/json")
			}
			r.ServeHTTP(w, req)
			if w.Code != tc.want || repo.scope != tc.scope {
				t.Fatalf("status=%d want=%d scope=%q wantScope=%q body=%s", w.Code, tc.want, repo.scope, tc.scope, w.Body.String())
			}
		})
	}
}

func TestProtectedRoutesDenyMissingAndWrongPermissions(t *testing.T) {
	paths := []string{"/api/v0/customer-intelligence/cards", "/api/v0/customer-intelligence/universe"}
	for _, path := range paths {
		for _, tc := range []struct {
			name, header string
			grants       []string
			want         int
		}{
			{"missing token", "", nil, http.StatusUnauthorized},
			{"wrong permission", "Bearer token", []string{"cdphub.other.view"}, http.StatusForbidden},
			{"view permission", "Bearer token", []string{service.CustomerIntelligenceViewPermission}, http.StatusBadRequest},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				r, err := New(Dependencies{Auth: authStub{identity: auth.Identity{Permissions: tc.grants}}, Cards: api.NewCardsHandler(nil), Universe: api.NewUniverseHandler(nil), DB: dbStub{}})
				if err != nil {
					t.Fatal(err)
				}
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("invalid"))
				req.Header.Set("Authorization", tc.header)
				r.ServeHTTP(w, req)
				if w.Code != tc.want {
					t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
				}
			})
		}
	}
}

func TestAccessRouteAndNoLegacyRoutes(t *testing.T) {
	r, err := New(Dependencies{Auth: authStub{}, Cards: api.NewCardsHandler(nil), Universe: api.NewUniverseHandler(nil), DB: dbStub{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, path, auth string
		status             int
	}{
		{http.MethodGet, "/api/v0/internal/access", "Bearer token", http.StatusOK},
		{http.MethodGet, "/api/v0/internal/access", "", http.StatusUnauthorized},
		{http.MethodGet, "/api/v1/audiences", "Bearer token", http.StatusNotFound},
		{http.MethodPost, "/api/v0/segments", "Bearer token", http.StatusNotFound},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("Authorization", tc.auth)
		r.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("%s %s: status=%d want=%d", tc.method, tc.path, w.Code, tc.status)
		}
	}
}

func TestDocumentationRoutesArePublic(t *testing.T) {
	r, err := New(Dependencies{Auth: authStub{}, Cards: api.NewCardsHandler(nil), Universe: api.NewUniverseHandler(nil), DB: dbStub{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, contentType, contains string
	}{
		{"/openapi.yaml", "application/yaml", "url: ./"},
		{"/docs/openapi.yaml", "application/yaml", "url: ./"},
		{"/docs/swagger-ui/", "text/html; charset=utf-8", `url: "../../openapi.yaml"`},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != tc.contentType || !strings.Contains(w.Body.String(), tc.contains) {
			t.Fatalf("%s: status=%d content-type=%q body-match=%t", tc.path, w.Code, w.Header().Get("Content-Type"), strings.Contains(w.Body.String(), tc.contains))
		}
	}
}
