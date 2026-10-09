package response

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/platform/contextkeys"
	pkgErrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/platform/errors"
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

func trace(ctx context.Context) string {
	value, _ := ctx.Value(contextkeys.TraceIdKey).(string)
	return value
}
func write(ctx context.Context, w http.ResponseWriter, status int, body envelope) {
	w.Header().Set("Content-Type", "application/json")
	if id := trace(ctx); id != "" {
		w.Header().Set("X-Trace-ID", id)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func SuccessWithContext(ctx context.Context, w http.ResponseWriter, data any) {
	write(ctx, w, http.StatusOK, envelope{Data: data, Message: "Success", Success: true, TraceID: trace(ctx)})
}
func ErrorWithContext(ctx context.Context, w http.ResponseWriter, err error) {
	app := pkgErrors.UnknownInternalV1()
	var typed *pkgErrors.AppErrorV1
	if errors.As(err, &typed) {
		app = typed
	}
	write(ctx, w, app.HTTPStatus, envelope{Errors: &errorDetail{Code: app.Code, Message: app.Message}, Success: false, TraceID: trace(ctx)})
}
