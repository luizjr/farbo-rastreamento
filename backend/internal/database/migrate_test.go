package database

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pedrofarbo/farbo-rastreamento/backend/migrations"
)

// Várias instâncias sobem juntas num deploy e todas chamam Migrate no mesmo
// banco. Precisa de um Postgres descartável em FARBO_TEST_DATABASE_URL; cada
// execução usa um schema próprio, apagado no fim.
func TestConcurrentMigrate(t *testing.T) {
	dsn := os.Getenv("FARBO_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("defina FARBO_TEST_DATABASE_URL (Postgres descartável) para rodar o teste com banco")
	}
	ctx := context.Background()
	schema := "test_migrate_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("conectando no Postgres de teste: %v", err)
	}
	defer func() { _ = admin.Close(context.Background()) }()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Logf("não foi possível apagar o schema %s: %v", schema, err)
		}
	})

	const instances = 4
	pools := make([]*DB, instances)
	for i := range pools {
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		cfg.ConnConfig.RuntimeParams["search_path"] = schema
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Close()
		pools[i] = &DB{Pool: pool}
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	errs := make([]error, instances)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, db := range pools {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = db.Migrate(ctx, log)
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("instância %d: %v", i, err)
		}
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var applied int
	if err := pools[0].QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != len(entries) {
		t.Errorf("migrations aplicadas: %d, esperado %d (uma vez cada)", applied, len(entries))
	}
}
