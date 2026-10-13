package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/domain"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/dto"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/platform/contextkeys"
	pkgErrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/platform/errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/platform/response"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/service"
)

type CardsHandler struct{ service *service.CardsService }

func NewCardsHandler(s *service.CardsService) *CardsHandler {
	return &CardsHandler{service: s}
}

func (h *CardsHandler) QueryCards(c *gin.Context) {
	var obj map[string]json.RawMessage
	dec := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 4096))
	if err := dec.Decode(&obj); err != nil || obj == nil {
		response.ErrorWithContext(c.Request.Context(), c.Writer, pkgErrors.CommonInvalidRequestV1())
		return
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		response.ErrorWithContext(c.Request.Context(), c.Writer, pkgErrors.CommonInvalidRequestV1())
		return
	}
	var req dto.CardsRequest
	for key, raw := range obj {
		if key != "scope_code" {
			response.ErrorWithContext(c.Request.Context(), c.Writer, pkgErrors.CommonInvalidRequestV1())
			return
		}
		if string(raw) == "null" {
			continue
		}
		var value string
		if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &value) != nil {
			response.ErrorWithContext(c.Request.Context(), c.Writer, pkgErrors.CommonInvalidRequestV1())
			return
		}
		req.ScopeCode = &value
	}
	permissions, _ := c.Request.Context().Value(contextkeys.PermissionsKey).([]string)
	out, err := h.service.Query(c.Request.Context(), domain.CardsQuery{ScopeCode: req.ScopeCode, Permissions: permissions})
	if err != nil {
		response.ErrorWithContext(c.Request.Context(), c.Writer, cardsHTTPError(err))
		return
	}
	response.SuccessWithContext(c.Request.Context(), c.Writer, mapCardsView(out))
}

func cardsHTTPError(err error) *pkgErrors.AppErrorV1 {
	switch {
	case errors.Is(err, domain.ErrCardsForbidden):
		return pkgErrors.AccessForbiddenV1()
	case errors.Is(err, domain.ErrCardsInvalidScope):
		return pkgErrors.CommonInvalidRequestV1()
	case errors.Is(err, domain.ErrCardsScopeMissing):
		return pkgErrors.CardsScopeNotFoundV1()
	case errors.Is(err, domain.ErrCardsUnavailable):
		return pkgErrors.CardsDataUnavailableV1()
	case errors.Is(err, domain.ErrCardsInvalidData):
		return pkgErrors.CardsInvalidDataV1()
	case errors.Is(err, domain.ErrCardsDatabase):
		return pkgErrors.CommonDatabaseErrorV1()
	default:
		return pkgErrors.UnknownInternalV1()
	}
}

func mapCardsView(view domain.CardsView) dto.CardsResponse {
	out := dto.CardsResponse{
		Status: view.Status, ScopeCode: view.ScopeCode,
		Warnings: make([]dto.CardsWarning, 0, len(view.Warnings)),
		Features: make(map[string]dto.CardsFeature, len(view.Features)),
	}
	for _, warning := range view.Warnings {
		out.Warnings = append(out.Warnings, dto.CardsWarning{
			FeatureCode: warning.FeatureCode,
			Code:        warning.Code,
			Message:     warning.Message,
		})
	}
	for index, feature := range view.Features {
		item := dto.CardsFeature{FeatureCode: feature.Code, DisplayName: feature.Name, Type: feature.Type, DisplayOrder: index + 1, IsEnabled: feature.Enabled}
		if !feature.Enabled {
			out.Features[strings.ToLower(feature.Code)] = item
			continue
		}
		switch feature.Type {
		case "TREND":
			trend := make([]dto.CardsActivityPoint, 0, len(feature.Activity))
			for _, point := range feature.Activity {
				trend = append(trend, dto.CardsActivityPoint{Date: point.Date, Active: point.Active, ActivePre: point.ActivePre, Inactive: point.Inactive})
			}
			item.Data = dto.CardsTrendData{Points: trend}
		case "COMBO_TREND":
			trend := make([]dto.CardsMetricPoint, 0, len(feature.Trend))
			for _, point := range feature.Trend {
				trend = append(trend, dto.CardsMetricPoint{Label: point.Label, Count: point.Count, Percent: point.Percent})
			}
			item.Data = dto.CardsComboTrendData{Points: trend}
		case "DISTRIBUTION":
			data := dto.CardsDistributionData{Buckets: make([]dto.CardsMetricPoint, 0, len(feature.Buckets))}
			for _, point := range feature.Buckets {
				data.Buckets = append(data.Buckets, dto.CardsMetricPoint{Label: point.Label, Count: point.Count, Percent: point.Percent})
			}
			if len(feature.Summary) > 0 {
				data.Summary = make(map[string]dto.CardsSummaryMetric, len(feature.Summary))
				for code, metric := range feature.Summary {
					data.Summary[code] = dto.CardsSummaryMetric{
						Label: metric.Label, Count: metric.Count, Percent: metric.Percent,
						Growth: metric.Growth, Compare: metric.Compare,
					}
				}
			}
			item.Data = data
		}
		out.Features[strings.ToLower(feature.Code)] = item
	}
	return out
}
