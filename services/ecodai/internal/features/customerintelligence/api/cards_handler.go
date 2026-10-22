package api

import (
	"encoding/json"
	"errors"

	"github.com/labstack/echo/v5"
	apierrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/service"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/response"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
)

type CardsHandler struct{ service *service.CardsService }

func NewCardsHandler(s *service.CardsService) *CardsHandler {
	return &CardsHandler{service: s}
}

func (h *CardsHandler) QueryCards(c *echo.Context) error {
	obj, ok := decodeJSONObject(c.Request().Body)
	if !ok {
		return apierrors.InvalidRequest()
	}
	var req CardsRequest
	for key, raw := range obj {
		if key != "scope_code" && key != "start_date" && key != "end_date" {
			return apierrors.InvalidRequest()
		}
		if string(raw) == "null" && key == "scope_code" {
			continue
		}
		var value string
		if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &value) != nil {
			return apierrors.InvalidRequest()
		}
		switch key {
		case "scope_code":
			req.ScopeCode = &value
		case "start_date":
			req.StartDate = value
		case "end_date":
			req.EndDate = value
		}
	}
	permissions := auth.Permissions(c.Request().Context())
	out, err := h.service.Query(c.Request().Context(), model.CardsQuery{ScopeCode: req.ScopeCode, StartDate: req.StartDate, EndDate: req.EndDate, Permissions: permissions})
	if err != nil {
		return cardsHTTPError(err)
	}
	return response.Success(c, mapCardsView(out))
}

func cardsHTTPError(err error) *apierrors.APIError {
	switch {
	case errors.Is(err, model.ErrCardsForbidden):
		return apierrors.AccessDenied()
	case errors.Is(err, model.ErrCardsInvalidScope):
		return apierrors.InvalidRequest()
	case errors.Is(err, model.ErrInvalidReportingRange):
		return apierrors.InvalidRequest()
	case errors.Is(err, model.ErrCardsScopeMissing):
		return cardsScopeNotFoundError()
	case errors.Is(err, model.ErrCardsUnavailable):
		return cardsUnavailableError()
	case errors.Is(err, model.ErrCardsInvalidData):
		return cardsInvalidDataError()
	case errors.Is(err, model.ErrCardsDatabase):
		return databaseError()
	default:
		return apierrors.Internal()
	}
}
