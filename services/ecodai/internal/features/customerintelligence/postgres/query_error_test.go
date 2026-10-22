package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestQueryErrorPreservesCauseAndLogMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, category, repository string
		cause                      error
		wrap                       func(context.Context, string, error) error
	}{
		{"cards timeout", "timeout", "CardsRepository", context.DeadlineExceeded, cardsQueryError},
		{"universe cancel", "canceled", "UniverseRepository", context.Canceled, universeQueryError},
		{"cards database", "database", "CardsRepository", errors.New("secret database detail"), cardsQueryError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			old := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
			defer slog.SetDefault(old)
			wrapped := tc.wrap(context.Background(), "ReadSnapshot", tc.cause)
			if !errors.Is(wrapped, tc.cause) || !strings.Contains(wrapped.Error(), "ReadSnapshot") {
				t.Fatalf("cause or query name lost: %v", wrapped)
			}
			var fields map[string]any
			if err := json.Unmarshal(output.Bytes(), &fields); err != nil {
				t.Fatal(err)
			}
			if fields["repository"] != tc.repository || fields["query_name"] != "ReadSnapshot" || fields["category"] != tc.category {
				t.Fatalf("unexpected log fields: %v", fields)
			}
			_, hasOperation := fields["db_operation"]
			if hasOperation != (tc.repository == "UniverseRepository") {
				t.Fatalf("repository log metadata changed: %v", fields)
			}
		})
	}
}
