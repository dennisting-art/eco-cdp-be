package ports

import (
	"context"
	"time"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/eco/internal/domain"
)

type CardsRepository interface {
	GetSnapshot(ctx context.Context, scopeCode string, firstMonth, lastMonth time.Time) (domain.CardsSnapshot, error)
}
