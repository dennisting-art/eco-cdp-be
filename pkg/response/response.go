package response

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	apierrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/errors"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/logger"
)

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type envelope struct {
	Data    any          `json:"data,omitempty"`
	Errors  *errorDetail `json:"errors,omitempty"`
	Message string       `json:"message,omitempty"`
	Success bool         `json:"success"`
	TraceID string       `json:"trace_id,omitempty"`
}

func Success(c *echo.Context, data any) error {
	return c.JSON(http.StatusOK, envelope{Data: data, Message: "Success", Success: true, TraceID: logger.TraceID(c.Request().Context())})
}

func Created(c *echo.Context, data any) error {
	return c.JSON(http.StatusCreated, envelope{Data: data, Message: "Success", Success: true, TraceID: logger.TraceID(c.Request().Context())})
}

// ResolveError defines the client-visible error contract used by the response
// writer and request logging. Unexpected Echo 5xx errors use the safe fallback.
func ResolveError(err error) *apierrors.APIError {
	app := apierrors.Internal()
	var typed *apierrors.APIError
	if errors.As(err, &typed) {
		app = typed
	} else if errors.Is(err, echo.ErrNotFound) {
		app = apierrors.New("404", "not found", http.StatusNotFound)
	} else if errors.Is(err, echo.ErrMethodNotAllowed) {
		app = apierrors.New("405", "method not allowed", http.StatusMethodNotAllowed)
	} else {
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) && httpErr.Code >= 400 && httpErr.Code < 500 {
			app = apierrors.New(strconv.Itoa(httpErr.Code), strings.ToLower(http.StatusText(httpErr.Code)), httpErr.Code)
		}
	}
	if app.HTTPStatus < 400 || app.HTTPStatus > 599 {
		app = apierrors.Internal()
	}
	return app
}

func ErrorHandler(c *echo.Context, err error) {
	if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
		return
	}
	app := ResolveError(err)
	if c.Request().Method == http.MethodHead {
		_ = c.NoContent(app.HTTPStatus)
		return
	}
	if err := c.JSON(app.HTTPStatus, envelope{Errors: &errorDetail{Code: app.Code, Message: app.Message}, Success: false, TraceID: logger.TraceID(c.Request().Context())}); err != nil {
		logger.Error(c.Request().Context(), "write error response", "category", "http_write")
	}
}
