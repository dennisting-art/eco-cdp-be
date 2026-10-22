package config

import (
	"errors"
	"os"
	"strconv"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
)

type Config struct {
	DatabaseURL           string
	Auth                  auth.Config
	IntrospectionURL      string
	AuthCacheTTL          time.Duration
	SelfPilotEnabled      bool
	SelfPilotIssuer       string
	SelfPilotTokenTTL     time.Duration
	Environment           string
	HTTPAddress           string
	SegmentAPIEnabled     bool
	SegmentBuildEnabled   bool
	SegmentWorkerEnabled  bool
	SegmentGCSBucket      string
	SegmentLocalExportDir string
}

func Load() (Config, error) {
	for _, key := range []string{"DATABASE_URL", "AUTH_ISSUER", "AUTH_JWKS_URL", "AUTH_AUDIENCE", "AUTH_INTROSPECTION_URL"} {
		if os.Getenv(key) == "" {
			return Config{}, errors.New("missing " + key)
		}
	}
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), Auth: auth.Config{Issuer: os.Getenv("AUTH_ISSUER"), JWKSURL: os.Getenv("AUTH_JWKS_URL"), Audience: os.Getenv("AUTH_AUDIENCE")}, IntrospectionURL: os.Getenv("AUTH_INTROSPECTION_URL"), AuthCacheTTL: 30 * time.Second, SelfPilotIssuer: os.Getenv("SELF_PILOT_ISSUER"), SelfPilotTokenTTL: time.Hour, Environment: os.Getenv("APP_ENV"), HTTPAddress: os.Getenv("HTTP_ADDR")}
	if err := auth.ValidateConfig(c.Auth); err != nil {
		return Config{}, err
	}
	if err := auth.ValidateIntrospectionURL(c.IntrospectionURL); err != nil {
		return Config{}, err
	}
	if value := os.Getenv("AUTH_CACHE_TTL"); value != "" {
		var err error
		c.AuthCacheTTL, err = time.ParseDuration(value)
		if err != nil {
			return Config{}, err
		}
	}
	if value := os.Getenv("SELF_PILOT_ENABLED"); value != "" {
		var err error
		c.SelfPilotEnabled, err = strconv.ParseBool(value)
		if err != nil {
			return Config{}, err
		}
	}
	if c.SelfPilotEnabled {
		if value := os.Getenv("SELF_PILOT_TOKEN_TTL"); value != "" {
			var err error
			c.SelfPilotTokenTTL, err = time.ParseDuration(value)
			if err != nil {
				return Config{}, err
			}
		}
	}
	if c.HTTPAddress == "" {
		c.HTTPAddress = ":8080"
	}
	for _, setting := range []struct {
		name  string
		value *bool
	}{{"SEGMENT_V0_API_ENABLED", &c.SegmentAPIEnabled}, {"SEGMENT_V0_BUILD_ENABLED", &c.SegmentBuildEnabled}, {"SEGMENT_V0_WORKER_ENABLED", &c.SegmentWorkerEnabled}} {
		if raw := os.Getenv(setting.name); raw != "" {
			parsed, err := strconv.ParseBool(raw)
			if err != nil {
				return Config{}, err
			}
			*setting.value = parsed
		}
	}
	if c.SegmentWorkerEnabled && !c.SegmentBuildEnabled {
		return Config{}, errors.New("SEGMENT_V0_WORKER_ENABLED requires SEGMENT_V0_BUILD_ENABLED")
	}
	// The build-backed routes can return member IDs through exports. Until the
	// trusted organisation guard is implemented, keep this prototype local.
	if c.SegmentBuildEnabled && c.Environment != "local" {
		return Config{}, errors.New("SEGMENT_V0_BUILD_ENABLED requires APP_ENV=local until the organisation guard is enabled")
	}
	c.SegmentGCSBucket = os.Getenv("SEGMENT_V0_GCS_BUCKET")
	c.SegmentLocalExportDir = os.Getenv("SEGMENT_V0_LOCAL_EXPORT_DIR")
	if c.SegmentLocalExportDir == "" {
		c.SegmentLocalExportDir = "./tmp/segment-exports"
	}
	if c.SegmentWorkerEnabled && c.Environment != "local" && c.SegmentGCSBucket == "" {
		return Config{}, errors.New("missing SEGMENT_V0_GCS_BUCKET")
	}
	return c, nil
}
