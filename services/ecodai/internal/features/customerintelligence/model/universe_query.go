package model

import "errors"

var (
	ErrUniverseForbidden   = errors.New("universe access forbidden")
	ErrUniverseUnavailable = errors.New("universe data unavailable")
	ErrUniverseInvalidData = errors.New("invalid universe reporting data")
	ErrUniverseDatabase    = errors.New("universe database failure")
)

type UniverseQuery struct {
	StartDate, EndDate string
	Permissions        []string
}
