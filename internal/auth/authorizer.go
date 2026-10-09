package auth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/permission"
)

var ErrPermissionDenied = errors.New("permission denied")

type Identity struct {
	Subject     string
	Permissions []string
}

type tokenValidator interface {
	Validate(context.Context, string) (Claims, error)
}
type permissionSource interface {
	Resolve(context.Context, string) ([]string, error)
}

type cacheEntry struct {
	permissions []string
	expires     time.Time
}

type Authorizer struct {
	validator tokenValidator
	source    permissionSource
	ttl       time.Duration
	mu        sync.Mutex
	cache     map[[32]byte]cacheEntry
}

func NewAuthorizer(validator tokenValidator, source permissionSource, ttl time.Duration) (*Authorizer, error) {
	if validator == nil || source == nil || ttl <= 0 || ttl > time.Minute {
		return nil, errors.New("invalid authorization configuration")
	}
	return &Authorizer{validator: validator, source: source, ttl: ttl, cache: map[[32]byte]cacheEntry{}}, nil
}

func (a *Authorizer) Authenticate(ctx context.Context, token string) (Identity, error) {
	validationStarted := time.Now()
	claims, err := a.validator.Validate(ctx, token)
	slog.Info("auth_jwt_validation", "duration_ms", time.Since(validationStarted).Milliseconds(), "ok", err == nil)
	if err != nil {
		return Identity{}, err
	}
	key := sha256.Sum256([]byte(token))
	now := time.Now()
	a.mu.Lock()
	cached, ok := a.cache[key]
	a.mu.Unlock()
	if ok && now.Before(cached.expires) {
		slog.Info("auth_permission_lookup", "duration_ms", 0, "cache_hit", true)
		return Identity{Subject: claims.Subject, Permissions: append([]string(nil), cached.permissions...)}, nil
	}
	lookupStarted := time.Now()
	permissions, err := a.source.Resolve(ctx, token)
	slog.Info("auth_permission_lookup", "duration_ms", time.Since(lookupStarted).Milliseconds(), "cache_hit", false, "ok", err == nil)
	if err != nil {
		return Identity{}, err
	}
	expires := now.Add(a.ttl)
	if tokenExpiry := time.Unix(claims.ExpiresAt, 0); tokenExpiry.Before(expires) {
		expires = tokenExpiry
	}
	a.mu.Lock()
	if len(a.cache) >= 4096 {
		for key, entry := range a.cache {
			if !now.Before(entry.expires) {
				delete(a.cache, key)
			}
		}
		if len(a.cache) >= 4096 {
			// A full cache is safe to discard: it is only a latency optimization.
			clear(a.cache)
		}
	}
	a.cache[key] = cacheEntry{permissions: append([]string(nil), permissions...), expires: expires}
	a.mu.Unlock()
	return Identity{Subject: claims.Subject, Permissions: permissions}, nil
}

func Allows(identity Identity, required string) bool {
	return permission.Allows(identity.Permissions, required)
}

type IntrospectionClient struct {
	URL    string
	Client *http.Client
}

func ValidateIntrospectionURL(endpoint string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(endpoint))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("invalid introspection URL")
	}
	return nil
}

func (c *IntrospectionClient) Resolve(ctx context.Context, token string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, nil)
	if err != nil {
		return nil, ErrValidationUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrValidationUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrInvalidToken
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrValidationUnavailable
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Active      bool     `json:"active"`
			Permissions []string `json:"permissions"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body) != nil || !body.Success {
		return nil, ErrValidationUnavailable
	}
	if !body.Data.Active {
		return nil, ErrInvalidToken
	}
	return body.Data.Permissions, nil
}

func Bearer(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.ContainsAny(parts[1], " \t\r\n") {
		return "", false
	}
	return parts[1], true
}
