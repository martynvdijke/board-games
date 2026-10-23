package trmnl

import (
	"context"
	"testing"

	"boardgames/internal/config"
	"boardgames/internal/db"
	"database/sql"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.OpenMemory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("DATABASE_PATH", ":memory:")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
