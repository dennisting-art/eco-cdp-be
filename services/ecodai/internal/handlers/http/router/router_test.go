package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
	controller "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/handlers/http/controller"
)

type authStub struct {
	identity auth.Identity
	err      error
}

func (s authStub) Authenticate(context.Context, string) (auth.Identity, error) {
	return s.identity, s.err
}

type dbStub struct{}

func (dbStub) Ping(context.Context) error { return nil }

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
			{"view permission", "Bearer token", []string{CustomerIntelligenceViewPermission}, http.StatusBadRequest},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				r, err := New(Dependencies{Auth: authStub{identity: auth.Identity{Permissions: tc.grants}}, Cards: controller.NewCardsHandler(nil), Universe: controller.NewUniverseHandler(nil), DB: dbStub{}})
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
	r, err := New(Dependencies{Auth: authStub{}, Cards: controller.NewCardsHandler(nil), Universe: controller.NewUniverseHandler(nil), DB: dbStub{}})
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
	r, err := New(Dependencies{Auth: authStub{}, Cards: controller.NewCardsHandler(nil), Universe: controller.NewUniverseHandler(nil), DB: dbStub{}})
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
