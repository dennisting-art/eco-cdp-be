package errors

import "net/http"

// APIError carries a safe, stable error contract for HTTP clients.
type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
	Cause      error
}

func (e *APIError) Error() string { return e.Message }
func (e *APIError) Unwrap() error { return e.Cause }
func New(code, message string, status int) *APIError {
	return &APIError{Code: code, Message: message, HTTPStatus: status}
}
func Wrap(code, message string, status int, cause error) *APIError {
	return &APIError{Code: code, Message: message, HTTPStatus: status, Cause: cause}
}

func InvalidRequest() *APIError {
	return New("10001", "invalid request", http.StatusBadRequest)
}

func Unauthorized() *APIError {
	return New("401", "unauthorized", http.StatusUnauthorized)
}

func AccessDenied() *APIError {
	return New("40007", "access denied", http.StatusForbidden)
}

func ServiceUnavailable() *APIError {
	return New("503", "service unavailable", http.StatusServiceUnavailable)
}

func Internal() *APIError {
	return New("99999", "internal server error", http.StatusInternalServerError)
}
