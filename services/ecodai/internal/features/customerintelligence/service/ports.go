package service

import (
	"context"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
)

type CardsRepository interface {
	GetSnapshot(ctx context.Context, scopeCode string, requested *model.ReportingRange) (model.CardsSnapshot, error)
}

type UniverseRepository interface {
	GetLatestCharts(ctx context.Context, requested *model.ReportingRange) (model.UniverseSnapshot, error)
}
