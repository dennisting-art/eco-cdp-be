package postgres

import (
	"context"
	"errors"
	"fmt"

	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/logger"
)

func queryError(ctx context.Context, message, repository, query string, err error, attributes ...any) error {
	category := "database"
	if errors.Is(err, context.DeadlineExceeded) {
		category = "timeout"
	} else if errors.Is(err, context.Canceled) {
		category = "canceled"
	}
	fields := []any{"service", "ecodai", "repository", repository, "query_name", query}
	fields = append(fields, attributes...)
	fields = append(fields, "category", category)
	logger.Error(ctx, message, fields...)
	return fmt.Errorf("%s: %w", query, err)
}
