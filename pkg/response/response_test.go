package response

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
	apierrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/errors"
)

func TestResolveErrorMatchesClientContract(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		err        error
		status     int
	}{
		{"typed", "503", apierrors.ServiceUnavailable(), http.StatusServiceUnavailable},
		{"wrapped typed", "10001", fmt.Errorf("request: %w", apierrors.InvalidRequest()), http.StatusBadRequest},
		{"not found", "404", echo.ErrNotFound, http.StatusNotFound},
		{"method not allowed", "405", echo.ErrMethodNotAllowed, http.StatusMethodNotAllowed},
		{"Echo 4xx", "418", echo.NewHTTPError(http.StatusTeapot, "private message"), http.StatusTeapot},
		{"Echo 5xx fallback", "99999", echo.NewHTTPError(http.StatusServiceUnavailable, "private message"), http.StatusInternalServerError},
		{"invalid typed status", "99999", apierrors.New("bad", "bad", http.StatusOK), http.StatusInternalServerError},
		{"unknown fallback", "99999", errors.New("private failure"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolved := ResolveError(tc.err)
			if resolved.Code != tc.code || resolved.HTTPStatus != tc.status {
				t.Fatalf("resolved %+v, want code %s status %d", resolved, tc.code, tc.status)
			}
			if resolved.Message == "private message" || resolved.Message == "private failure" {
				t.Fatal("private cause leaked into response")
			}
		})
	}
}
