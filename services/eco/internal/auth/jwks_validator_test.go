package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidatorAuthorizationOutcomes(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	server := testJWKSServer(key, http.StatusOK)
	defer server.Close()
	validator := NewValidator(Config{Issuer: "https://partner.example", JWKSURL: server.URL, Audience: "eco-cdp-be"})

	valid := testToken(t, key, claims("eco-cdp-be", time.Now().Add(time.Minute), []string{"cdphub.cust_intel.view"}))
	if allowed, err := validator.Allowed(context.Background(), valid); err != nil || !allowed {
		t.Fatalf("valid token: allowed=%t err=%v", allowed, err)
	}

	withoutGrant := testToken(t, key, claims("eco-cdp-be", time.Now().Add(time.Minute), []string{"other.access"}))
	if allowed, err := validator.Allowed(context.Background(), withoutGrant); err != nil || !allowed {
		t.Fatalf("valid token without JWT grant: allowed=%t err=%v", allowed, err)
	}

	expired := testToken(t, key, claims("eco-cdp-be", time.Now().Add(-time.Minute), []string{"cdphub.cust_intel.view"}))
	if _, err := validator.Allowed(context.Background(), expired); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired token error=%v, want invalid token", err)
	}
}

func TestValidatorMapsJWKSOutage(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	server := testJWKSServer(key, http.StatusServiceUnavailable)
	defer server.Close()
	validator := NewValidator(Config{Issuer: "https://partner.example", JWKSURL: server.URL, Audience: "eco-cdp-be"})
	token := testToken(t, key, claims("eco-cdp-be", time.Now().Add(time.Minute), []string{"cdphub.cust_intel.view"}))
	if _, err := validator.Allowed(context.Background(), token); !errors.Is(err, ErrValidationUnavailable) {
		t.Fatalf("JWKS outage error=%v, want unavailable", err)
	}
}

func TestValidatorRejectsMalformedJWKSURLWithoutPanic(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	validator := NewValidator(Config{Issuer: "https://partner.example", JWKSURL: "://invalid", Audience: "eco-cdp-be"})
	token := testToken(t, key, claims("eco-cdp-be", time.Now().Add(time.Minute), []string{"cdphub.cust_intel.view"}))
	if _, err := validator.Allowed(context.Background(), token); !errors.Is(err, ErrValidationUnavailable) {
		t.Fatalf("malformed JWKS URL error=%v, want unavailable", err)
	}
}

func TestValidateConfig(t *testing.T) {
	valid := Config{Issuer: "https://partner.example", JWKSURL: "https://partner.example/.well-known/jwks.json", Audience: "eco-cdp-be"}
	if err := ValidateConfig(valid); err != nil {
		t.Fatalf("valid config error=%v", err)
	}
	valid.JWKSURL = "ftp://partner.example/keys"
	if err := ValidateConfig(valid); err == nil {
		t.Fatal("expected unsupported URL scheme to be rejected")
	}
}

func TestValidatorAcceptsStringAudience(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	server := testJWKSServer(key, http.StatusOK)
	defer server.Close()
	validator := NewValidator(Config{Issuer: "https://partner.example", JWKSURL: server.URL, Audience: "api"})
	token := testToken(t, key, map[string]any{
		"sub": "user-1", "iss": "https://partner.example", "aud": "api",
		"iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Minute).Unix(),
	})
	if allowed, err := validator.Allowed(context.Background(), token); err != nil || !allowed {
		t.Fatalf("string audience token: allowed=%t err=%v", allowed, err)
	}
}

func TestValidatorRejectsWrongAudience(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	server := testJWKSServer(key, http.StatusOK)
	defer server.Close()
	validator := NewValidator(Config{Issuer: "https://partner.example", JWKSURL: server.URL, Audience: "api"})
	token := testToken(t, key, claims("other-api", time.Now().Add(time.Minute), nil))
	if _, err := validator.Allowed(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong audience error=%v, want invalid token", err)
	}
}

func testJWKSServer(key *rsa.PrivateKey, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "test-key",
			"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
		}}})
	}))
}

func claims(audience string, expiry time.Time, scope []string) map[string]any {
	return map[string]any{
		"sub": "user-1", "iss": "https://partner.example", "aud": []string{audience},
		"iat": time.Now().Add(-time.Minute).Unix(), "exp": expiry.Unix(), "scope": scope,
	}
}

func testToken(t *testing.T, key *rsa.PrivateKey, tokenClaims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(tokenClaims)
	if err != nil {
		t.Fatal(err)
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}
