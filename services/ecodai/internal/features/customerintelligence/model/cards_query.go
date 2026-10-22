package model

import "errors"

var (
	ErrCardsForbidden    = errors.New("cards access forbidden")
	ErrCardsInvalidScope = errors.New("invalid cards scope")
	ErrCardsScopeMissing = errors.New("cards scope not found")
	ErrCardsUnavailable  = errors.New("cards data unavailable")
	ErrCardsInvalidData  = errors.New("invalid cards reporting data")
	ErrCardsDatabase     = errors.New("cards database failure")
)

type CardsQuery struct {
	ScopeCode          *string
	StartDate, EndDate string
	Permissions        []string
}
