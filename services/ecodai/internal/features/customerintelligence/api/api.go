package api

import (
	"errors"
	"github.com/labstack/echo/v5"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/service"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/selfpilot"
)

func RegisterRoutes(e *echo.Echo, authorizer auth.Authenticator, pilot *selfpilot.Service, cards *CardsHandler, universe *UniverseHandler) error {
	if cards == nil || universe == nil {
		return errors.New("missing customer intelligence handlers")
	}
	grant, err := auth.RequirePermission(service.CustomerIntelligenceViewPermission)
	if err != nil {
		return err
	}
	token := auth.RequireToken(authorizer, pilot)
	e.POST("/api/v0/customer-intelligence/cards", cards.QueryCards, token, grant)
	e.POST("/api/v0/customer-intelligence/universe", universe.QueryUniverse, token, grant)
	return nil
}
