package selfpilot

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const Audience = "eco-cdp-be"
const Scope = "eco_cdp.self_pilot"
const Profile = "cdphub_tester"
const MaxTTL = 12 * time.Hour
const profileUserID = "d2614122-b323-4450-bda9-27f441e59c92"
const profileOrganizationID = "997c1b26-3b2a-4323-86ee-627116d048fb"
const profileEmail = "cdphub-tester@self-pilot.invalid"
const profileRoleID = 1

var invalidToken = errors.New("invalid self-pilot token")

type Claims struct {
	Subject        string   `json:"sub"`
	Issuer         string   `json:"iss"`
	Audience       string   `json:"aud"`
	Scope          string   `json:"scope"`
	IssuedAt       int64    `json:"iat"`
	ExpiresAt      int64    `json:"exp"`
	ID             string   `json:"jti"`
	Profile        string   `json:"profile"`
	UserID         string   `json:"user_id"`
	OrganizationID string   `json:"organization_id"`
	Email          string   `json:"email"`
	RoleID         int      `json:"role_id"`
	Permissions    []string `json:"permissions"`
	IsSystemAdmin  bool     `json:"is_system_admin"`
}

type Service struct {
	key        *rsa.PrivateKey
	keyID      string
	issuer     string
	defaultTTL time.Duration
	now        func() time.Time
}

type Config struct {
	Enabled  bool
	Issuer   string
	TokenTTL time.Duration
}

func New(environment string, cfg Config) (*Service, error) {
	if !cfg.Enabled || (environment != "local" && environment != "development" && environment != "uat") {
		return nil, errors.New("self-pilot unavailable in this environment")
	}
	if strings.TrimSpace(cfg.Issuer) == "" || cfg.TokenTTL <= 0 || cfg.TokenTTL > MaxTTL {
		return nil, errors.New("invalid self-pilot configuration")
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &Service{key: key, keyID: uuid.NewString(), issuer: cfg.Issuer, defaultTTL: cfg.TokenTTL, now: time.Now}, nil
}

func (s *Service) Issuer() string { return s.issuer }

func (s *Service) Issue(profile string, ttl time.Duration) (string, Claims, error) {
	if profile != Profile {
		return "", Claims{}, errors.New("unknown profile")
	}
	if ttl == 0 {
		ttl = s.defaultTTL
	}
	if ttl < time.Second || ttl > MaxTTL {
		return "", Claims{}, errors.New("invalid token lifetime")
	}
	now := s.now().UTC().Truncate(time.Second)
	c := Claims{
		Subject: Profile, Issuer: s.issuer, Audience: Audience, Scope: Scope,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(ttl).Unix(), ID: uuid.NewString(),
		Profile: Profile, UserID: profileUserID, OrganizationID: profileOrganizationID,
		Email: profileEmail, RoleID: profileRoleID, Permissions: []string{"cdphub.*"},
	}
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": s.keyID})
	body, _ := json.Marshal(c)
	signed := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	hash := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, hash[:])
	if err != nil {
		return "", Claims{}, err
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig), c, nil
}

// HasIssuer selects the token family. The unverified payload grants no authority.
func HasIssuer(token, issuer string) bool {
	parts := strings.Split(token, ".")
	if len(parts) < 2 || len(parts[1]) > 8192 {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var marker struct {
		Issuer string `json:"iss"`
	}
	return json.Unmarshal(raw, &marker) == nil && marker.Issuer == issuer
}

func (s *Service) Validate(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(token) > 16384 {
		return Claims{}, invalidToken
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, invalidToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
		KeyID     string `json:"kid"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Algorithm != "RS256" || header.Type != "JWT" || header.KeyID != s.keyID {
		return Claims{}, invalidToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, invalidToken
	}
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(&s.key.PublicKey, crypto.SHA256, hash[:], signature) != nil {
		return Claims{}, invalidToken
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, invalidToken
	}
	var c Claims
	if json.Unmarshal(body, &c) != nil {
		return Claims{}, invalidToken
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(body, &raw) != nil || string(raw["is_system_admin"]) != "false" {
		return Claims{}, invalidToken
	}
	now := s.now().Unix()
	if c.Issuer != s.issuer || c.Audience != Audience || c.Scope != Scope || c.Subject != Profile || c.Profile != Profile || c.IssuedAt > now || c.ExpiresAt <= now || c.ExpiresAt <= c.IssuedAt || c.ExpiresAt-c.IssuedAt > int64(MaxTTL.Seconds()) {
		return Claims{}, invalidToken
	}
	if _, err = uuid.Parse(c.ID); err != nil {
		return Claims{}, invalidToken
	}
	if _, err = uuid.Parse(c.UserID); err != nil {
		return Claims{}, invalidToken
	}
	if _, err = uuid.Parse(c.OrganizationID); err != nil {
		return Claims{}, invalidToken
	}
	if c.UserID != profileUserID || c.OrganizationID != profileOrganizationID || c.Email != profileEmail || c.RoleID != profileRoleID || c.IsSystemAdmin || len(c.Permissions) != 1 || c.Permissions[0] != "cdphub.*" {
		return Claims{}, invalidToken
	}
	return c, nil
}
