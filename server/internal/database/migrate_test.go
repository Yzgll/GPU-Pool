package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrate(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	schema := "migration_test_" + time.Now().Format("150405000000")
	_, err = pool.Exec(ctx, "CREATE SCHEMA "+schema)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")

	_, err = pool.Exec(ctx, "SET search_path TO "+schema)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()

	sql := `
		CREATE TABLE migration_test_table (
			id BIGINT PRIMARY KEY
		);
	`

	if err := os.WriteFile(
		filepath.Join(dir, "001_test.up.sql"),
		[]byte(sql),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(ctx, pool, dir); err != nil {
		t.Fatalf("first migration failed: %v", err)
	}

	if err := Migrate(ctx, pool, dir); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}

	var exists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = $1
			AND table_name = 'migration_test_table'
		)
	`, schema).Scan(&exists)

	if err != nil {
		t.Fatal(err)
	}

	if !exists {
		t.Fatal("migration table was not created")
	}

	var count int
	err = pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM schema_migrations",
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("expected 1 migration, got %d", count)
	}
}
