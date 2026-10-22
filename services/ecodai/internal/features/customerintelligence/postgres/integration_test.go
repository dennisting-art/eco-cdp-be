package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/database"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/model"
	"gitlab.com/aeon360/internal/tech-team/data-and-ai/eco-cdp-be/services/ecodai/internal/features/customerintelligence/service"
)

// ECODAI_TEST_DSN must point to a disposable PostgreSQL database.
func migrationTestPool(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	dsn := os.Getenv("ECODAI_TEST_DSN")
	if dsn == "" {
		t.Skip("ECODAI_TEST_DSN is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := "ecodai_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") })
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
	t.Cleanup(pool.Close)
	return pool, ctx
}

func TestFreshCustomerIntelligenceMigration(t *testing.T) {
	pool, ctx := migrationTestPool(t)
	for _, path := range []string{"../../../../migrations/000001_customer_intelligence.up.sql", "testdata/universe.sql"} {
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
	view, err := service.NewUniverseService(NewUniverseRepository(db)).Query(ctx, model.UniverseQuery{Permissions: []string{"cdphub.cust_intel.view"}})
	if err != nil {
		t.Fatal(err)
	}
	if view.AsOfDate.IsZero() || len(view.Opcos) != 4 || view.Group.MemberMatrixActivity != nil || view.DataStatuses["member_matrix_activity"] != "stale" {
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
	cards, err := service.NewCardsService(NewCardsRepository(db)).Query(ctx, model.CardsQuery{Permissions: []string{"cdphub.cust_intel.view"}})
	if err != nil {
		t.Fatal(err)
	}
	if cards.ScopeCode != "AEON" || len(cards.Features) != 1 {
		t.Fatalf("unexpected cards response: %#v", cards)
	}
}

func TestCardsHistoricalSourceDateAndIncompleteMonth(t *testing.T) {
	pool, ctx := migrationTestPool(t)
	migration, err := os.ReadFile("../../../../migrations/000001_customer_intelligence.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO ecosystem_scope (scope_code,scope_type,display_name,display_order,is_available) VALUES ('AEON','GROUP','AEON',1,TRUE)`,
		`INSERT INTO feature_summary (feature_code,display_name,display_order,feature_type) VALUES ('ACTIVE_OPCO','Activity',1,'TREND'),('PENETRATION_OPCO','Penetration',2,'COMBO_TREND'),('TIER','Tier',3,'DISTRIBUTION')`,
		`INSERT INTO ecosystem_scope_feature (scope_code,feature_code,is_enabled) VALUES ('AEON','ACTIVE_OPCO',TRUE),('AEON','PENETRATION_OPCO',TRUE),('AEON','TIER',TRUE)`,
		`INSERT INTO feature_bucket_dim (feature_code,bucket_code,display_name,display_order) VALUES ('TIER',1,'One',1)`,
		`INSERT INTO member_daily_summary (as_of_date,scope_code,active_member_count,inactive_member_count) VALUES ('2026-09-29','AEON',29,1),('2026-09-30','AEON',30,1)`,
		`INSERT INTO transaction_daily_summary (as_of_date,scope_code,member_net_sales,non_member_net_sales) SELECT d::date,'AEON',10,10 FROM generate_series('2026-09-01'::date,'2026-09-30'::date,'1 day'::interval) d WHERE d::date <> '2026-09-17'::date`,
		`INSERT INTO member_daily_distribution (as_of_date,scope_code,feature_code,bucket_code,customer_count,ratio) VALUES ('2026-09-29','AEON','TIER',1,10,1)`,
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	db := &database.PostgresDB{Pool: pool}
	svc := service.NewCardsService(NewCardsRepository(db))
	permissions := []string{"cdphub.cust_intel.view"}
	latest, err := svc.Query(ctx, model.CardsQuery{Permissions: permissions})
	if err != nil {
		t.Fatal(err)
	}
	if latest.EndDate != "2026-09-30" || latest.Features[0].DataStatus != "partial" || len(latest.Features[0].Activity) != 1 || *latest.Features[0].Activity[0].Active != 30 || latest.Features[1].DataStatus != "partial" || len(latest.Features[1].Trend) != 0 || latest.Features[2].DataStatus != "stale" {
		t.Fatalf("latest source coverage: %+v", latest)
	}
	historical, err := svc.Query(ctx, model.CardsQuery{StartDate: "2026-09-01", EndDate: "2026-09-29", Permissions: permissions})
	if err != nil {
		t.Fatal(err)
	}
	if historical.Features[0].DataStatus != "" || len(historical.Features[0].Activity) != 1 || *historical.Features[0].Activity[0].Active != 29 || historical.Features[2].DataStatus != "" {
		t.Fatalf("historical end-date isolation: %+v", historical)
	}
}

func TestReferenceSeedMigrationRoundTrip(t *testing.T) {
	pool, ctx := migrationTestPool(t)
	for _, path := range []string{
		"../../../../migrations/000001_customer_intelligence.up.sql",
		"../../../../migrations/000002_seed_ecosystem_scope_features.up.sql",
		"../../../../migrations/000002_seed_ecosystem_scope_features.up.sql",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	var scopes, features, buckets, associations int
	err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM ecosystem_scope),
		(SELECT count(*) FROM feature_summary),
		(SELECT count(*) FROM feature_bucket_dim),
		(SELECT count(*) FROM ecosystem_scope_feature)`).Scan(&scopes, &features, &buckets, &associations)
	if err != nil || scopes != 5 || features != 9 || buckets != 28 || associations != 7 {
		t.Fatalf("seed counts: scopes=%d features=%d buckets=%d associations=%d err=%v", scopes, features, buckets, associations, err)
	}
	var available bool
	if err := pool.QueryRow(ctx, "SELECT is_available FROM ecosystem_scope WHERE scope_code = 'ADB'").Scan(&available); err != nil || available {
		t.Fatalf("ADB availability=%t err=%v", available, err)
	}
	path := "../../../../migrations/000002_seed_ecosystem_scope_features.down.sql"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(data)); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	err = pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM ecosystem_scope),
		(SELECT count(*) FROM feature_summary),
		(SELECT count(*) FROM feature_bucket_dim),
		(SELECT count(*) FROM ecosystem_scope_feature)`).Scan(&scopes, &features, &buckets, &associations)
	if err != nil || scopes != 0 || features != 1 || buckets != 0 || associations != 0 {
		t.Fatalf("rollback counts: scopes=%d features=%d buckets=%d associations=%d err=%v", scopes, features, buckets, associations, err)
	}
}

func TestSyntheticUniverseFixturePreservesReferenceCatalog(t *testing.T) {
	pool, ctx := migrationTestPool(t)
	for _, path := range []string{
		"../../../../migrations/000001_customer_intelligence.up.sql",
		"../../../../migrations/000002_seed_ecosystem_scope_features.up.sql",
		"testdata/universe.sql",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	var scopes, aliases, unavailable, tier, legacyTier int
	err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM ecosystem_scope),
		(SELECT count(*) FROM ecosystem_scope WHERE scope_code IN ('ACO','ABK')),
		(SELECT count(*) FROM ecosystem_scope WHERE scope_code IN ('ACSM','ADB') AND is_available = FALSE),
		(SELECT count(*) FROM feature_summary WHERE feature_code = 'TIER'),
		(SELECT count(*) FROM feature_summary WHERE feature_code = 'MBR_TIER')`).Scan(&scopes, &aliases, &unavailable, &tier, &legacyTier)
	if err != nil || scopes != 5 || aliases != 0 || unavailable != 2 || tier != 1 || legacyTier != 0 {
		t.Fatalf("catalog changed by fixture: scopes=%d aliases=%d unavailable=%d tier=%d legacyTier=%d err=%v", scopes, aliases, unavailable, tier, legacyTier, err)
	}
}
