// Package testdb provides shared helpers for database integration tests.
//
// It creates a dedicated throwaway database from the connection string used
// by the app, applies the goose migration files from db/migrations, and drops
// the database when the test finishes, so tests never touch development data.
//
// Usage:
//
//	pool := testdb.Setup(t)
//	// ... use pool in the test ...
package testdb

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// testDatabaseName is the throwaway database used by the tests. It is created
// from scratch on every test run and dropped afterwards, so tests never touch
// the development database's data.
const testDatabaseName = "cinema_booking_test"

// repoRoot returns the absolute path to the repository root, computed from
// this file's own location so it works no matter which package calls us
// (Go tests always run with the working directory of the test package).
func repoRoot() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	// thisFile is <root>/internal/testdb/testdb.go
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
}

// URL returns the connection string used by the tests.
//
// It reads TEST_DATABASE_URL first, then falls back to GOOSE_DBSTRING from the
// repo root .env file (the same one main.go uses). If neither is available the
// test is skipped instead of failing, so `go test ./...` still passes without a DB.
func URL(t *testing.T) string {
	t.Helper()

	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}

	// Fall back to the .env file used by the app, so local dev works out of the box.
	envPath := filepath.Join(repoRoot(), ".env")
	if envMap, err := godotenv.Read(envPath); err == nil {
		if url := envMap["GOOSE_DBSTRING"]; url != "" {
			return url
		}
	}

	t.Skip("TEST_DATABASE_URL (or GOOSE_DBSTRING in .env) is not set; skipping database test")
	return ""
}

// Setup creates a dedicated test database (dropping any leftover from a
// previous run), applies the migration files to it, and returns a pool to it.
//
// Registered cleanups, in order:
//  1. truncate users (and restart its id sequence) after each test,
//  2. close the pool,
//  3. drop the test database.
func Setup(t *testing.T) *pgxpool.Pool {
	t.Helper()

	baseConfig, err := pgxpool.ParseConfig(URL(t))
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}

	// Connect to the default "postgres" database to be able to CREATE/DROP
	// databases. A CREATE DATABASE cannot run inside a transaction, so use a
	// plain connection instead of the pool.
	adminConfig := baseConfig.ConnConfig.Copy()
	adminConfig.Database = "postgres"

	adminConn, err := pgx.Connect(t.Context(), adminConfig.ConnString())
	if err != nil {
		t.Fatalf("connect to postgres database: %v", err)
	}
	defer adminConn.Close(t.Context())

	// Drop leftovers from a previous run, then create a fresh database.
	if _, err := adminConn.Exec(t.Context(),
		`DROP DATABASE IF EXISTS `+testDatabaseName+` WITH (FORCE)`); err != nil {
		t.Fatalf("drop test database: %v", err)
	}
	if _, err := adminConn.Exec(t.Context(),
		`CREATE DATABASE `+testDatabaseName); err != nil {
		t.Fatalf("create test database: %v", err)
	}

	// Clean up the test database after the test finishes.
	t.Cleanup(func() {
		dropConfig := baseConfig.ConnConfig.Copy()
		dropConfig.Database = "postgres"

		dropConn, err := pgx.Connect(context.Background(), dropConfig.ConnString())
		if err != nil {
			t.Errorf("connect to drop test database: %v", err)
			return
		}
		defer dropConn.Close(context.Background())

		if _, err := dropConn.Exec(context.Background(),
			`DROP DATABASE IF EXISTS `+testDatabaseName+` WITH (FORCE)`); err != nil {
			t.Errorf("drop test database: %v", err)
		}
	})

	testConfig, err := pgxpool.ParseConfig(URL(t))
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	testConfig.ConnConfig.Database = testDatabaseName

	pool, err := pgxpool.NewWithConfig(t.Context(), testConfig)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(t.Context()); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	ApplyMigrations(t, pool)

	// Reset the table (and its id sequence) before every test for isolation.
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `TRUNCATE users RESTART IDENTITY CASCADE`); err != nil {
			t.Errorf("truncate users table: %v", err)
		}
	})

	return pool
}

// ApplyMigrations runs the Up section of every migration file in db/migrations
// against the given pool.
func ApplyMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	migrations, err := filepath.Glob(filepath.Join(repoRoot(), "db", "migrations", "*.sql"))
	if err != nil {
		t.Fatalf("list migration files: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("no migration files found in db/migrations")
	}

	for _, path := range migrations {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}

		// Goose files contain an Up and a Down section; we only execute the Up part.
		upSection := strings.SplitN(string(contents), "-- +goose Down", 2)[0]

		if _, err := pool.Exec(t.Context(), upSection); err != nil {
			t.Fatalf("apply migration %s: %v", path, err)
		}
	}
}
