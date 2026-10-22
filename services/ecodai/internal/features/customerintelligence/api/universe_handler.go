package api

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/labstack/echo/v5"
	apierrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/service"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/response"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
)

type UniverseHandler struct{ service *service.UniverseService }

func NewUniverseHandler(s *service.UniverseService) *UniverseHandler {
	return &UniverseHandler{service: s}
}

func (h *UniverseHandler) QueryUniverse(c *echo.Context) error {
	var req UniverseRequest
	obj, ok := decodeJSONObject(c.Request().Body)
	if !ok {
		return apierrors.InvalidRequest()
	}
	for key, raw := range obj {
		var value string
		if key != "start_date" && key != "end_date" {
			return apierrors.InvalidRequest()
		}
		if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &value) != nil {
			return apierrors.InvalidRequest()
		}
		if key == "start_date" {
			req.StartDate = strings.TrimSpace(value)
		} else {
			req.EndDate = strings.TrimSpace(value)
		}
	}
	permissions := auth.Permissions(c.Request().Context())
	out, err := h.service.Query(c.Request().Context(), model.UniverseQuery{
		StartDate: req.StartDate, EndDate: req.EndDate, Permissions: permissions,
	})
	if err != nil {
		return universeHTTPError(err)
	}
	return response.Success(c, mapUniverseView(out))
}

func universeHTTPError(err error) *apierrors.APIError {
	switch {
	case errors.Is(err, model.ErrUniverseForbidden):
		return apierrors.AccessDenied()
	case errors.Is(err, model.ErrInvalidReportingRange):
		return apierrors.InvalidRequest()
	case errors.Is(err, model.ErrUniverseUnavailable):
		return universeUnavailableError()
	case errors.Is(err, model.ErrUniverseInvalidData):
		return universeInvalidDataError()
	case errors.Is(err, model.ErrUniverseDatabase):
		return databaseError()
	default:
		return apierrors.Internal()
	}
}
