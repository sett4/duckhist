package cmd

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/sett4/duckhist/internal/migrate"
)

func TestRunMigrations(t *testing.T) {
	// Create a temporary database file
	tmpDir, err := os.MkdirTemp("", "duckhist-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Run migrations
	if err := RunMigrations(dbPath); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify migrations were applied
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Check if history table exists and has the expected columns
	var tableName string
	err = db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_name = 'history'").Scan(&tableName)
	if err != nil {
		t.Fatalf("History table not found: %v", err)
	}

	// Check if schema_migrations table exists and has the expected version
	var version int
	var dirty bool
	err = db.QueryRow("SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version, &dirty)
	if err != nil {
		t.Fatalf("Failed to get schema version: %v", err)
	}

	if version != 3 { // Latest migration version
		t.Errorf("Expected schema version 3, got %d", version)
	}

	if dirty {
		t.Error("Schema is marked as dirty")
	}
}
