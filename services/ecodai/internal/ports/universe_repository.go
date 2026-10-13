package ports

import (
	"context"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/domain"
)

type UniverseRepository interface {
	GetLatestCharts(ctx context.Context) (domain.UniverseSnapshot, error)
}
