package config

import (
	"strings"
	"testing"
)

func TestBuildBackedSegmentsStayLocalUntilOrgGuard(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example.invalid/db")
	t.Setenv("AUTH_ISSUER", "https://issuer.example.invalid")
	t.Setenv("AUTH_JWKS_URL", "https://issuer.example.invalid/jwks")
	t.Setenv("AUTH_AUDIENCE", "eco-cdp-be")
	t.Setenv("AUTH_INTROSPECTION_URL", "https://issuer.example.invalid/introspect")
	t.Setenv("SEGMENT_V0_BUILD_ENABLED", "true")
	t.Setenv("APP_ENV", "dev")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "APP_ENV=local") {
		t.Fatalf("non-local build flag must fail closed: %v", err)
	}
	t.Setenv("APP_ENV", "local")
	config, err := Load()
	if err != nil || !config.SegmentBuildEnabled {
		t.Fatalf("local synthetic build should be enabled: %+v, %v", config, err)
	}
}
