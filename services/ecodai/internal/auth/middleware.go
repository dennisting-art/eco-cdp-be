package auth

import (
	"context"
	"errors"

	"github.com/labstack/echo/v5"
	apierrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/selfpilot"
)

type Authenticator interface {
	Authenticate(context.Context, string) (Identity, error)
}
type identityKey struct{}
type permissionsKey struct{}

func Permissions(ctx context.Context) []string {
	permissions, _ := ctx.Value(permissionsKey{}).([]string)
	return permissions
}
func IdentityFrom(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityKey{}).(Identity)
	return identity, ok
}

func RequireToken(authorizer Authenticator, pilot *selfpilot.Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			token, ok := Bearer(c.Request().Header.Get("Authorization"))
			if !ok {
				return apierrors.Unauthorized()
			}
			var identity Identity
			var err error
			if pilot != nil && selfpilot.HasIssuer(token, pilot.Issuer()) {
				var claims selfpilot.Claims
				claims, err = pilot.Validate(token)
				if err != nil {
					err = ErrInvalidToken
				} else {
					identity = Identity{Subject: claims.UserID, Permissions: claims.Permissions}
				}
			} else {
				identity, err = authorizer.Authenticate(c.Request().Context(), token)
			}
			if err != nil {
				if errors.Is(err, ErrInvalidToken) {
					return apierrors.Unauthorized()
				}
				return apierrors.ServiceUnavailable()
			}
			ctx := context.WithValue(c.Request().Context(), identityKey{}, identity)
			ctx = context.WithValue(ctx, permissionsKey{}, identity.Permissions)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

func RequirePermission(permission string) (echo.MiddlewareFunc, error) {
	if permission == "" {
		return nil, errors.New("protected route missing permission")
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			identity, ok := IdentityFrom(c.Request().Context())
			if !ok || !Allows(identity, permission) {
				return apierrors.AccessDenied()
			}
			return next(c)
		}
	}, nil
}
