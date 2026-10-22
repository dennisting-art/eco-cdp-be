package api

import (
	"strings"
	"testing"
)

func TestDecodeJSONObjectFraming(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"empty object", `{}`, true},
		{"duplicate key remains accepted", `{"scope_code":"AEON","scope_code":"ABG"}`, true},
		{"empty body", ``, false},
		{"null object", `null`, false},
		{"array", `[]`, false},
		{"malformed", `{"scope_code":`, false},
		{"trailing object", `{} {}`, false},
		{"trailing scalar", `{} 1`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := decodeJSONObject(strings.NewReader(tc.body))
			if ok != tc.valid {
				t.Fatalf("valid=%t, want %t", ok, tc.valid)
			}
		})
	}
}
