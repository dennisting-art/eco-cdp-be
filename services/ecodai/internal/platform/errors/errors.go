package errors

import "net/http"

type AppErrorV1 struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *AppErrorV1) Error() string { return e.Message }
func app(code, message string, status int) *AppErrorV1 {
	return &AppErrorV1{Code: code, Message: message, HTTPStatus: status}
}

func AccessForbiddenV1() *AppErrorV1 { return app("40007", "access denied", http.StatusForbidden) }
func CardsDataUnavailableV1() *AppErrorV1 {
	return app("93003", "cards data unavailable", http.StatusServiceUnavailable)
}
func CardsInvalidDataV1() *AppErrorV1 {
	return app("93004", "invalid cards reporting data", http.StatusInternalServerError)
}
func CardsScopeNotFoundV1() *AppErrorV1 {
	return app("93005", "customer intelligence scope not found", http.StatusNotFound)
}
func CommonDatabaseErrorV1() *AppErrorV1 {
	return app("10009", "database operation failed", http.StatusInternalServerError)
}
func CommonInvalidRequestV1() *AppErrorV1 {
	return app("10001", "invalid request", http.StatusBadRequest)
}
func UnknownInternalV1() *AppErrorV1 {
	return app("99999", "internal server error", http.StatusInternalServerError)
}
func UniverseDataUnavailableV1() *AppErrorV1 {
	return app("93001", "universe data unavailable", http.StatusServiceUnavailable)
}
func UniverseInvalidDataV1() *AppErrorV1 {
	return app("93002", "invalid universe reporting data", http.StatusInternalServerError)
}
