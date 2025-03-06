package migrate

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/marcboeker/go-duckdb"
)

func TestCheckSchemaVersion(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.duckdb")

	t.Run("schema_migrations table does not exist", func(t *testing.T) {
		// Create a new database without schema_migrations table
		db, err := sql.Open("duckdb", dbPath)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}
		defer db.Close()

		// Check schema version
		ok, current, required, err := CheckSchemaVersion(db)
		if err != nil {
			t.Fatalf("CheckSchemaVersion failed: %v", err)
		}

		// Verify results
		if ok {
			t.Errorf("expected ok to be false, got true")
		}
		if current != 0 {
			t.Errorf("expected current version to be 0, got %d", current)
		}
		if required <= 0 {
			t.Errorf("expected required version to be > 0, got %d", required)
		}
	})

	t.Run("schema version is outdated", func(t *testing.T) {
		// Create a new database with schema_migrations table
		db, err := sql.Open("duckdb", dbPath)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}
		defer db.Close()

		// Create schema_migrations table
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			dirty BOOLEAN,
			applied_at TIMESTAMP default current_timestamp
		)`)
		if err != nil {
			t.Fatalf("failed to create schema_migrations table: %v", err)
		}

		// Insert outdated version
		_, err = db.Exec("INSERT INTO schema_migrations (version, dirty) VALUES (1, false)")
		if err != nil {
			t.Fatalf("failed to insert schema version: %v", err)
		}

		// Check schema version
		ok, current, required, err := CheckSchemaVersion(db)
		if err != nil {
			t.Fatalf("CheckSchemaVersion failed: %v", err)
		}

		// Verify results
		if ok {
			t.Errorf("expected ok to be false, got true")
		}
		if current != 1 {
			t.Errorf("expected current version to be 1, got %d", current)
		}
		if required <= current {
			t.Errorf("expected required version to be > %d, got %d", current, required)
		}
	})

	t.Run("schema version is up to date", func(t *testing.T) {
		// Create a new database with schema_migrations table
		os.Remove(dbPath) // Remove previous database
		db, err := sql.Open("duckdb", dbPath)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}
		defer db.Close()

		// Create schema_migrations table
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			dirty BOOLEAN,
			applied_at TIMESTAMP default current_timestamp
		)`)
		if err != nil {
			t.Fatalf("failed to create schema_migrations table: %v", err)
		}

		// Get latest migration version
		latestVersion, err := GetLatestMigrationVersion()
		if err != nil {
			t.Fatalf("GetLatestMigrationVersion failed: %v", err)
		}

		// Insert latest version
		_, err = db.Exec("INSERT INTO schema_migrations (version, dirty) VALUES (?, false)", latestVersion)
		if err != nil {
			t.Fatalf("failed to insert schema version: %v", err)
		}

		// Check schema version
		ok, current, required, err := CheckSchemaVersion(db)
		if err != nil {
			t.Fatalf("CheckSchemaVersion failed: %v", err)
		}

		// Verify results
		if !ok {
			t.Errorf("expected ok to be true, got false")
		}
		if current != required {
			t.Errorf("expected current version to equal required version, got %d != %d", current, required)
		}
	})
}

func TestGetLatestMigrationVersion(t *testing.T) {
	// Get latest migration version
	version, err := GetLatestMigrationVersion()
	if err != nil {
		t.Fatalf("GetLatestMigrationVersion failed: %v", err)
	}

	// Verify results
	if version <= 0 {
		t.Errorf("expected version to be > 0, got %d", version)
	}
}
