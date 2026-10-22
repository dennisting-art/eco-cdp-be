package selfpilot

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/server"
)

func TestTokenRouteContract(t *testing.T) {
	service, err := New("local", Config{Enabled: true, Issuer: "test-issuer", TokenTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	e := server.New(server.Options{BodyLimitBytes: 4096})
	RegisterRoutes(e, service)
	call := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v0/internal/self-pilot/token", strings.NewReader(body)))
		return w
	}
	invalid := call(`{"unexpected":true}`)
	if invalid.Code != http.StatusBadRequest || invalid.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("invalid request: status=%d header=%q", invalid.Code, invalid.Header().Get("Cache-Control"))
	}
	var errorBody struct {
		Success bool `json:"success"`
		Errors  struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(invalid.Body.Bytes(), &errorBody); err != nil || errorBody.Success || errorBody.Errors.Code != "10001" {
		t.Fatalf("invalid response: %s", invalid.Body.String())
	}
	for i := 0; i < 29; i++ {
		if w := call(`{}`); w.Code != http.StatusOK {
			t.Fatalf("issuance %d: status=%d body=%s", i, w.Code, w.Body.String())
		}
	}
	limited := call(`{}`)
	if limited.Code != http.StatusTooManyRequests || limited.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("rate limit: status=%d body=%s", limited.Code, limited.Body.String())
	}
	if err := json.Unmarshal(limited.Body.Bytes(), &errorBody); err != nil || errorBody.Errors.Code != "429" {
		t.Fatalf("rate limit response: %s", limited.Body.String())
	}
}
