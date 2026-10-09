package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidToken          = errors.New("invalid token")
	ErrValidationUnavailable = errors.New("token validation unavailable")
)

type Config struct {
	Issuer   string
	JWKSURL  string
	Audience string
}

type Validator struct {
	issuer, jwksURL, audience string
	client                    *http.Client
	mu                        sync.Mutex
	keys                      map[string]*rsa.PublicKey
	refreshed                 time.Time
}

// Claims are the verified Partner Portal identity claims.
type Claims struct {
	Subject   string        `json:"sub"`
	Issuer    string        `json:"iss"`
	Audience  audienceClaim `json:"aud"`
	ExpiresAt int64         `json:"exp"`
	IssuedAt  int64         `json:"iat"`
	Scopes    ScopeClaim    `json:"scope"`
}

func NewValidator(config Config) *Validator {
	return &Validator{
		issuer: config.Issuer, jwksURL: config.JWKSURL, audience: config.Audience,
		client: &http.Client{Timeout: 3 * time.Second},
	}
}

func ValidateConfig(config Config) error {
	if strings.TrimSpace(config.Issuer) == "" || strings.TrimSpace(config.JWKSURL) == "" || strings.TrimSpace(config.Audience) == "" {
		return errors.New("required auth configuration is missing")
	}
	for _, value := range []string{config.Issuer, config.JWKSURL} {
		parsed, err := url.ParseRequestURI(value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return errors.New("auth URL is invalid")
		}
	}
	return nil
}

func (v *Validator) Allowed(ctx context.Context, token string) (bool, error) {
	_, err := v.Validate(ctx, token)
	if err != nil {
		return false, err
	}
	return true, nil
}

// Validate verifies a token's signature and required standard claims.
func (v *Validator) Validate(ctx context.Context, token string) (Claims, error) {
	if len(token) > 16384 {
		return Claims{}, ErrInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}
	var claims Claims
	if decode(parts[0], &header) != nil || decode(parts[1], &claims) != nil || header.Algorithm != "RS256" || header.KeyID == "" {
		return Claims{}, ErrInvalidToken
	}
	key, err := v.key(ctx, header.KeyID)
	if err != nil {
		return Claims{}, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig) != nil {
		return Claims{}, ErrInvalidToken
	}
	if claims.Subject == "" || claims.Issuer != v.issuer || claims.IssuedAt <= 0 || claims.IssuedAt > time.Now().Unix()+60 || claims.ExpiresAt <= claims.IssuedAt {
		return Claims{}, ErrInvalidToken
	}
	if claims.ExpiresAt <= time.Now().Unix() {
		return Claims{}, ErrInvalidToken
	}
	for _, audience := range claims.Audience {
		if audience == v.audience {
			return claims, nil
		}
	}
	return Claims{}, ErrInvalidToken
}

func (c Claims) HasScope(required string) bool {
	for _, scope := range c.Scopes {
		if scope == required {
			return true
		}
	}
	return false
}

type audienceClaim []string

func (a *audienceClaim) UnmarshalJSON(value []byte) error {
	var single string
	if err := json.Unmarshal(value, &single); err == nil {
		*a = []string{single}
		return nil
	}
	var multiple []string
	if err := json.Unmarshal(value, &multiple); err != nil {
		return err
	}
	*a = multiple
	return nil
}

type ScopeClaim []string

func (s *ScopeClaim) UnmarshalJSON(value []byte) error {
	var single string
	if err := json.Unmarshal(value, &single); err == nil {
		*s = strings.Fields(single)
		return nil
	}
	var multiple []string
	if err := json.Unmarshal(value, &multiple); err != nil {
		return err
	}
	*s = multiple
	return nil
}

// TokenIssuer reads an untrusted issuer solely to select a configured validator.
// Callers must validate the token before granting access.
func TokenIssuer(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", ErrInvalidToken
	}
	var claims struct {
		Issuer string `json:"iss"`
	}
	if decode(parts[1], &claims) != nil || claims.Issuer == "" {
		return "", ErrInvalidToken
	}
	return claims.Issuer, nil
}

func (v *Validator) key(ctx context.Context, id string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	if key := v.keys[id]; key != nil && time.Since(v.refreshed) < 5*time.Minute {
		v.mu.Unlock()
		return key, nil
	}
	v.mu.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: jwks request", ErrValidationUnavailable)
	}
	response, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: jwks request", ErrValidationUnavailable)
	}
	defer response.Body.Close()
	var body struct {
		Keys []struct {
			KeyID string `json:"kid"`
			N     string `json:"n"`
			E     string `json:"e"`
		} `json:"keys"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body) != nil {
		return nil, fmt.Errorf("%w: jwks response", ErrValidationUnavailable)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, item := range body.Keys {
		n, ne := base64.RawURLEncoding.DecodeString(item.N)
		e, ee := base64.RawURLEncoding.DecodeString(item.E)
		if ne != nil || ee != nil {
			continue
		}
		exponent := 0
		for _, b := range e {
			exponent = exponent<<8 + int(b)
		}
		keys[item.KeyID] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}
	}
	v.mu.Lock()
	v.keys = keys
	v.refreshed = time.Now()
	key := v.keys[id]
	v.mu.Unlock()
	if key != nil {
		return key, nil
	}
	return nil, ErrInvalidToken
}

func decode(value string, target any) error {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}
