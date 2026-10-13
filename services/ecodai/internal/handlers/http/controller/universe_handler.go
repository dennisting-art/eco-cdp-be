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

type UniverseHandler struct{ service *service.UniverseService }

func NewUniverseHandler(s *service.UniverseService) *UniverseHandler {
	return &UniverseHandler{service: s}
}

func (h *UniverseHandler) QueryUniverse(c *gin.Context) {
	var req dto.UniverseRequest
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
	for key, raw := range obj {
		var value string
		if key != "start_date" && key != "end_date" {
			response.ErrorWithContext(c.Request.Context(), c.Writer, pkgErrors.CommonInvalidRequestV1())
			return
		}
		if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &value) != nil {
			response.ErrorWithContext(c.Request.Context(), c.Writer, pkgErrors.CommonInvalidRequestV1())
			return
		}
		if key == "start_date" {
			req.StartDate = strings.TrimSpace(value)
		} else {
			req.EndDate = strings.TrimSpace(value)
		}
	}
	permissions, _ := c.Request.Context().Value(contextkeys.PermissionsKey).([]string)
	out, err := h.service.Query(c.Request.Context(), domain.UniverseQuery{
		StartDate: req.StartDate, EndDate: req.EndDate, Permissions: permissions,
	})
	if err != nil {
		response.ErrorWithContext(c.Request.Context(), c.Writer, universeHTTPError(err))
		return
	}
	response.SuccessWithContext(c.Request.Context(), c.Writer, mapUniverseView(out))
}

func universeHTTPError(err error) *pkgErrors.AppErrorV1 {
	switch {
	case errors.Is(err, domain.ErrUniverseForbidden):
		return pkgErrors.AccessForbiddenV1()
	case errors.Is(err, domain.ErrUniverseDateUnsupported):
		out := pkgErrors.CommonInvalidRequestV1()
		out.Message = "date filtering is not yet supported"
		return out
	case errors.Is(err, domain.ErrUniverseUnavailable):
		return pkgErrors.UniverseDataUnavailableV1()
	case errors.Is(err, domain.ErrUniverseInvalidData):
		return pkgErrors.UniverseInvalidDataV1()
	case errors.Is(err, domain.ErrUniverseDatabase):
		return pkgErrors.CommonDatabaseErrorV1()
	default:
		return pkgErrors.UnknownInternalV1()
	}
}
