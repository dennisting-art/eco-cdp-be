package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/logger"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/response"
)

type Options struct {
	RequestTimeout time.Duration
	BodyLimitBytes int64
}

func New(options Options) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = response.ErrorHandler
	e.Use(middleware.Recover(), RequestContext(options.RequestTimeout))
	if options.BodyLimitBytes > 0 {
		e.Use(BodyLimit(options.BodyLimitBytes))
	}
	return e
}

func BodyLimit(limit int64) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, limit)
			return next(c)
		}
	}
}

func RequestContext(timeout time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()
			id := c.Request().Header.Get("X-Trace-ID")
			if id == "" || len(id) > 128 {
				var b [16]byte
				if _, err := rand.Read(b[:]); err == nil {
					id = hex.EncodeToString(b[:])
				}
			}
			ctx := logger.WithTraceID(c.Request().Context(), id)
			var cancel context.CancelFunc
			if timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}
			c.SetRequest(c.Request().WithContext(ctx))
			c.Response().Header().Set("X-Trace-ID", id)
			err := next(c)
			status := http.StatusOK
			committed := false
			if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
				status = r.Status
				committed = true
			}
			if err != nil && !committed {
				status = response.ResolveError(err).HTTPStatus
			}
			logger.Info(ctx, "request", "method", c.Request().Method, "path", c.Path(), "status", status, "duration_ms", time.Since(start).Milliseconds())
			return err
		}
	}
}
