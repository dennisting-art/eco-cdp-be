package api

import (
	"encoding/json"
	"errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/model"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
	apierrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/response"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/segments/service"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/selfpilot"
)

type Handler struct {
	Service     *service.Service
	ListingOnly bool
}

func RegisterRoutes(e *echo.Echo, a auth.Authenticator, pilot *selfpilot.Service, h *Handler) error {
	if h == nil || h.Service == nil {
		return errors.New("missing segments handler")
	}
	grant, err := auth.RequirePermission("cdphub.cust_intel.view")
	if err != nil {
		return err
	}
	token := auth.RequireToken(a, pilot)
	e.GET("/api/v0/segments/list", h.List, token, grant)
	e.GET("/api/v0/segments/:id", h.Get, token, grant)
	e.POST("/api/v0/segments", h.Create, token, grant)
	if !h.ListingOnly {
		e.GET("/api/v0/segments/requests/:request_id", h.Status, token, grant)
		e.POST("/api/v0/segments/requests", h.Request, token, grant)
	}
	return nil
}

func (h *Handler) List(c *echo.Context) error {
	page, size := 1, 20
	var err error
	if s := c.QueryParam("page"); s != "" {
		page, err = strconv.Atoi(s)
		if err != nil || page < 1 {
			return apierrors.InvalidRequest()
		}
	}
	if s := c.QueryParam("pageSize"); s != "" {
		size, err = strconv.Atoi(s)
		if err != nil || size < 1 || size > 100 {
			return apierrors.InvalidRequest()
		}
	}
	out, total, err := h.Service.List(c.Request().Context(), model.ListFilter{Search: c.QueryParam("search"), Tags: c.QueryParams()["tags"], Page: page, PageSize: size})
	if err != nil {
		return mapError(err)
	}
	return response.Success(c, map[string]any{"segments": out, "page": page, "pageSize": size, "total": total})
}
func (h *Handler) Get(c *echo.Context) error {
	out, err := h.Service.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapError(err)
	}
	return response.Success(c, out)
}
func decode(body io.Reader, out any) error {
	d := json.NewDecoder(body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return apierrors.InvalidRequest()
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return apierrors.InvalidRequest()
	}
	return nil
}
func caller(c *echo.Context) (string, error) {
	id, ok := auth.IdentityFrom(c.Request().Context())
	if !ok || strings.TrimSpace(id.Subject) == "" {
		return "", apierrors.Unauthorized()
	}
	return id.Subject, nil
}
func (h *Handler) Create(c *echo.Context) error {
	var in model.CreateSegmentInput
	if err := decode(c.Request().Body, &in); err != nil {
		return err
	}
	subject, err := caller(c)
	if err != nil {
		return err
	}
	out, err := h.Service.CreateSegment(c.Request().Context(), in, subject)
	if err != nil {
		return mapError(err)
	}
	return response.Created(c, out)
}
func (h *Handler) Request(c *echo.Context) error {
	var in model.RequestInput
	if err := decode(c.Request().Body, &in); err != nil {
		return err
	}
	subject, err := caller(c)
	if err != nil {
		return err
	}
	estimate, req, err := h.Service.Request(c.Request().Context(), in, subject, c.Request().Header.Get("Idempotency-Key"))
	if err != nil {
		return mapError(err)
	}
	if estimate != nil {
		return response.Success(c, estimate)
	}
	return response.Success(c, map[string]any{"request_id": req.RequestID, "status": req.Status})
}
func (h *Handler) Status(c *echo.Context) error {
	out, err := h.Service.GetRequest(c.Request().Context(), c.Param("request_id"))
	if err != nil {
		return mapError(err)
	}
	data := map[string]any{"request_id": out.RequestID, "status": out.Status, "progress": out.Progress}
	if out.BuildID != nil {
		data["build_id"] = *out.BuildID
	}
	if out.Status == "READY" {
		data["total_count"] = out.TotalCount
		if out.BuildConsentCount != nil {
			data["build_consent_count"] = *out.BuildConsentCount
		}
		data["consent_count"] = out.ConsentCount
		if out.ConsentCheckedAt != nil {
			data["consent_checked_at"] = *out.ConsentCheckedAt
		}
		data["bucket"] = out.Bucket
	}
	if out.Status == "FAILED" && out.ErrorCode != nil {
		data["error_code"] = *out.ErrorCode
	}
	return response.Success(c, data)
}
func mapError(err error) error {
	var typed *apierrors.APIError
	if errors.As(err, &typed) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apierrors.New("404", "not found", http.StatusNotFound)
	}
	return err
}
