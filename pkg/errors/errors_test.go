package errors

import (
	"net/http"
	"testing"
)

func TestCommonErrorContracts(t *testing.T) {
	for _, tc := range []struct {
		name, code, message string
		status              int
		newError            func() *APIError
	}{
		{"invalid request", "10001", "invalid request", http.StatusBadRequest, InvalidRequest},
		{"unauthorized", "401", "unauthorized", http.StatusUnauthorized, Unauthorized},
		{"access denied", "40007", "access denied", http.StatusForbidden, AccessDenied},
		{"service unavailable", "503", "service unavailable", http.StatusServiceUnavailable, ServiceUnavailable},
		{"internal", "99999", "internal server error", http.StatusInternalServerError, Internal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.newError()
			if err.Code != tc.code || err.Message != tc.message || err.HTTPStatus != tc.status || err.Cause != nil {
				t.Fatalf("unexpected error contract: %+v", err)
			}
			if err == tc.newError() {
				t.Fatal("common error constructor reused a mutable value")
			}
		})
	}
}
