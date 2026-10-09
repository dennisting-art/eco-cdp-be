package repository

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/internal/domain"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/internal/service"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/pkg/database"
)

// ECO_TEST_DSN must point to a disposable PostgreSQL database.
func TestFreshCustomerIntelligenceMigration(t *testing.T) {
	dsn := os.Getenv("ECO_TEST_DSN")
	if dsn == "" {
		t.Skip("ECO_TEST_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "eco_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") }()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, path := range []string{"../../migrations/000001_customer_intelligence.up.sql", "testdata/universe.sql"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	var segmentTable bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('segments_v0') IS NOT NULL").Scan(&segmentTable); err != nil {
		t.Fatal(err)
	}
	if segmentTable {
		t.Fatal("first migration contains segment schema")
	}
	db := &database.PostgresDB{Pool: pool}
	view, err := service.NewUniverseService(NewUniverseRepository(db)).Query(ctx, domain.UniverseQuery{Permissions: []string{"cdphub.cust_intel.view"}})
	if err != nil {
		t.Fatal(err)
	}
	if view.AsOfDate.IsZero() || len(view.Opcos) != 4 || view.Group.MemberMatrixActivity == nil {
		t.Fatalf("unexpected universe response: status=%s date=%v opcos=%d matrix=%v", view.Status, view.AsOfDate, len(view.Opcos), view.Group.MemberMatrixActivity)
	}
	_, err = pool.Exec(ctx, `INSERT INTO feature_summary (feature_code,display_name,display_order,feature_type,feature_context) VALUES ('ACTIVE_OPCO','Activity',1,'TREND','CARDS')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO ecosystem_scope_feature (scope_code,feature_code,is_enabled) VALUES ('AEON','ACTIVE_OPCO',TRUE)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO member_daily_summary (scope_code,as_of_date,active_member_count,inactive_member_count) VALUES ('AEON',CURRENT_DATE,100,20)`)
	if err != nil {
		t.Fatal(err)
	}
	cards, err := service.NewCardsService(NewCardsRepository(db)).Query(ctx, domain.CardsQuery{Permissions: []string{"cdphub.cust_intel.view"}})
	if err != nil {
		t.Fatal(err)
	}
	if cards.ScopeCode != "AEON" || len(cards.Features) != 1 {
		t.Fatalf("unexpected cards response: %#v", cards)
	}
}
