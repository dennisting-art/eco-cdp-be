package system

import (
	"context"
	"errors"
	"github.com/labstack/echo/v5"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/server"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/api"
	segmentsapi "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/api"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/selfpilot"
	"time"
)

type Pinger interface{ Ping(context.Context) error }
type Dependencies struct {
	Auth     auth.Authenticator
	Cards    *api.CardsHandler
	Universe *api.UniverseHandler
	DB       Pinger
	Pilot    *selfpilot.Service
	Segments *segmentsapi.Handler
}

func New(d Dependencies) (*echo.Echo, error) {
	if d.Auth == nil || d.Cards == nil || d.Universe == nil || d.DB == nil {
		return nil, errors.New("missing server dependency")
	}
	e := server.New(server.Options{RequestTimeout: 10 * time.Second, BodyLimitBytes: 4096})
	RegisterRoutes(e, d.DB, d.Auth, d.Pilot)
	if err := api.RegisterRoutes(e, d.Auth, d.Pilot, d.Cards, d.Universe); err != nil {
		return nil, err
	}
	selfpilot.RegisterRoutes(e, d.Pilot)
	if d.Segments != nil {
		if err := segmentsapi.RegisterRoutes(e, d.Auth, d.Pilot, d.Segments); err != nil {
			return nil, err
		}
	}
	return e, nil
}
