package auth

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type validatorStub struct {
	claims Claims
	err    error
	calls  int
}

func (s *validatorStub) Validate(context.Context, string) (Claims, error) {
	s.calls++
	return s.claims, s.err
}

type sourceStub struct {
	permissions []string
	err         error
	calls       int
}

func (s *sourceStub) Resolve(context.Context, string) ([]string, error) {
	s.calls++
	return s.permissions, s.err
}

func TestAuthorizerCachesPermissionSetAfterRevalidatingToken(t *testing.T) {
	v := &validatorStub{claims: Claims{Subject: "user-1", ExpiresAt: time.Now().Add(time.Minute).Unix()}}
	s := &sourceStub{permissions: []string{"cdphub.cust_intel.view"}}
	a, err := NewAuthorizer(v, s, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	first, err := a.Authenticate(context.Background(), "token")
	if err != nil {
		t.Fatal(err)
	}
	if !Allows(first, "cdphub.cust_intel.view") || Allows(first, "cdphub.aud_build.update") {
		t.Fatal("permission set authorized incorrect operation")
	}
	second, err := a.Authenticate(context.Background(), "token")
	if err != nil || !reflect.DeepEqual(first, second) || v.calls != 2 || s.calls != 1 {
		t.Fatalf("cache behavior: first=%v second=%v validator=%d source=%d err=%v", first, second, v.calls, s.calls, err)
	}
	v.err = ErrInvalidToken
	if _, err := a.Authenticate(context.Background(), "token"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired or revoked JWT accepted from cache: %v", err)
	}
}

func TestAuthorizerRejectsUnusableConfiguration(t *testing.T) {
	for _, ttl := range []time.Duration{0, time.Minute + time.Second} {
		if _, err := NewAuthorizer(&validatorStub{}, &sourceStub{}, ttl); err == nil {
			t.Fatalf("accepted ttl %s", ttl)
		}
	}
}
