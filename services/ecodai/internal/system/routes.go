package system

import (
	"github.com/labstack/echo/v5"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/response"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/docs"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/selfpilot"
	"net/http"
)

func RegisterRoutes(e *echo.Echo, db Pinger, authorizer auth.Authenticator, pilot *selfpilot.Service) {
	e.GET("/openapi.yaml", func(c *echo.Context) error { return c.Blob(http.StatusOK, "application/yaml", docs.OpenAPI) })
	e.GET("/docs/openapi.yaml", func(c *echo.Context) error { return c.Blob(http.StatusOK, "application/yaml", docs.OpenAPI) })
	e.GET("/docs/swagger-ui/", func(c *echo.Context) error { return c.Blob(http.StatusOK, "text/html; charset=utf-8", docs.SwaggerUI) })
	e.GET("/healthz", func(c *echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"status": "ok"}) })
	e.GET("/readyz", func(c *echo.Context) error {
		if err := db.Ping(c.Request().Context()); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	e.GET("/api/v0/internal/access", func(c *echo.Context) error { return response.Success(c, map[string]bool{"allowed": true}) }, auth.RequireToken(authorizer, pilot))
}
