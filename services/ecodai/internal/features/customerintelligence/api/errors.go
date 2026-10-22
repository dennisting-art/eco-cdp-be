package api

import (
	apierrors "gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/errors"
	"net/http"
)

func cardsScopeNotFoundError() *apierrors.APIError {
	return apierrors.New("93005", "customer intelligence scope not found", http.StatusNotFound)
}
func cardsUnavailableError() *apierrors.APIError {
	return apierrors.New("93003", "cards data unavailable", http.StatusServiceUnavailable)
}
func cardsInvalidDataError() *apierrors.APIError {
	return apierrors.New("93004", "invalid cards reporting data", http.StatusInternalServerError)
}
func universeUnavailableError() *apierrors.APIError {
	return apierrors.New("93001", "universe data unavailable", http.StatusServiceUnavailable)
}
func universeInvalidDataError() *apierrors.APIError {
	return apierrors.New("93002", "invalid universe reporting data", http.StatusInternalServerError)
}
func databaseError() *apierrors.APIError {
	return apierrors.New("10009", "database operation failed", http.StatusInternalServerError)
}
