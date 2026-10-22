package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestRequestLogStatusMatchesErrorResponse(t *testing.T) {
	for _, tc := range []struct {
		name      string
		committed bool
		status    int
	}{
		{"uncommitted Echo error", false, http.StatusInternalServerError},
		{"committed response", true, http.StatusAccepted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			old := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
			defer slog.SetDefault(old)

			e := New(Options{})
			e.GET("/fail", func(c *echo.Context) error {
				if tc.committed {
					if err := c.String(http.StatusAccepted, "accepted"); err != nil {
						return err
					}
				}
				return echo.NewHTTPError(http.StatusServiceUnavailable, "private upstream message")
			})
			w := httptest.NewRecorder()
			e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/fail", nil))
			if w.Code != tc.status {
				t.Fatalf("client status=%d, want %d", w.Code, tc.status)
			}
			var logLine map[string]any
			if err := json.Unmarshal(output.Bytes(), &logLine); err != nil {
				t.Fatal(err)
			}
			if logLine["status"] != float64(w.Code) {
				t.Fatalf("logged status=%v differs from response=%d", logLine["status"], w.Code)
			}
			if !tc.committed {
				var body struct {
					Errors struct {
						Code string `json:"code"`
					} `json:"errors"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Errors.Code != "99999" {
					t.Fatalf("unexpected client error: %s", w.Body.String())
				}
			}
		})
	}
}
