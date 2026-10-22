package service

import (
	"context"
	"fmt"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"strings"
)

const defaultCardsScope = "AEON"

type CardsService struct {
	repo CardsRepository
}

func NewCardsService(repo CardsRepository) *CardsService {
	return &CardsService{repo: repo}
}

func (s *CardsService) Query(ctx context.Context, req model.CardsQuery) (model.CardsView, error) {
	var out model.CardsView
	allowed := auth.AllowsGrants(req.Permissions, CustomerIntelligenceViewPermission)
	if !allowed {
		return out, model.ErrCardsForbidden
	}
	scopeCode := defaultCardsScope
	if req.ScopeCode != nil && *req.ScopeCode != "" {
		scopeCode = *req.ScopeCode
		if scopeCode != strings.TrimSpace(scopeCode) || len(scopeCode) > 30 {
			return out, model.ErrCardsInvalidScope
		}
	}
	requested, err := requestedRange(req.StartDate, req.EndDate)
	if err != nil {
		return out, err
	}
	snap, err := s.repo.GetSnapshot(ctx, scopeCode, requested)
	if err != nil {
		return out, fmt.Errorf("%w: %w", model.ErrCardsDatabase, err)
	}
	if !snap.ScopeFound || !snap.ScopeAvailable {
		return out, model.ErrCardsScopeMissing
	}
	if snap.Range.End.IsZero() && hasEnabledCards(snap.Features) {
		return out, model.ErrCardsUnavailable
	}
	return buildCardsContext(ctx, scopeCode, snap.Range.Start, snap.Range.End, snap)
}

func hasEnabledCards(features []model.CardsFeature) bool {
	for _, feature := range features {
		if feature.Enabled {
			return true
		}
	}
	return false
}
