package selfpilot

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
	apierrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/response"
)

type issuer struct {
	service *Service
	mu      sync.Mutex
	window  time.Time
	count   int
}

func RegisterRoutes(e *echo.Echo, service *Service) {
	if service == nil {
		return
	}
	i := &issuer{service: service}
	e.POST("/api/v0/internal/self-pilot/token", i.issue)
}

func (i *issuer) issue(c *echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	i.mu.Lock()
	if time.Since(i.window) >= time.Minute {
		i.window = time.Now()
		i.count = 0
	}
	i.count++
	limited := i.count > 30
	i.mu.Unlock()
	if limited {
		return apierrors.New("429", "too many requests", http.StatusTooManyRequests)
	}
	var body map[string]json.RawMessage
	decoder := json.NewDecoder(c.Request().Body)
	err := decoder.Decode(&body)
	if err != nil && err != io.EOF || len(body) != 0 {
		return apierrors.InvalidRequest()
	}
	if err == nil && decoder.Decode(new(any)) != io.EOF {
		return apierrors.InvalidRequest()
	}
	token, claims, err := i.service.Issue(Profile, 0)
	if err != nil {
		return apierrors.ServiceUnavailable()
	}
	return response.Success(c, map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": claims.ExpiresAt - claims.IssuedAt, "profile": claims.Profile})
}
