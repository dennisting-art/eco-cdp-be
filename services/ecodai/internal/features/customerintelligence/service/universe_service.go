package service

import (
	"context"
	"fmt"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/auth"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
)

type UniverseService struct{ repo UniverseRepository }

func NewUniverseService(repo UniverseRepository) *UniverseService {
	return &UniverseService{repo: repo}
}

func (s *UniverseService) Query(ctx context.Context, req model.UniverseQuery) (model.UniverseView, error) {
	var out model.UniverseView
	allowed := auth.AllowsGrants(req.Permissions, CustomerIntelligenceViewPermission)
	if !allowed {
		return out, model.ErrUniverseForbidden
	}
	requested, err := requestedRange(req.StartDate, req.EndDate)
	if err != nil {
		return out, err
	}
	snap, err := s.repo.GetLatestCharts(ctx, requested)
	if err != nil {
		return out, fmt.Errorf("%w: %w", model.ErrUniverseDatabase, err)
	}
	if snap.Range.End.IsZero() {
		return out, model.ErrUniverseUnavailable
	}
	return buildUniverseContext(ctx, snap)
}
