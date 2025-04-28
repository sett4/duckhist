package history

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func captureStderr(f func()) string {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	f()

	if err := w.Close(); err != nil {
		panic(fmt.Sprintf("failed to close writer: %v", err))
	}

	os.Stderr = oldStderr

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		panic(fmt.Sprintf("failed to copy output: %v", err))
	}

	return buf.String()
}

func TestSchemaVersionCheck(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.sqlite")

	t.Run("warning message when schema version is outdated", func(t *testing.T) {
		// Create a new database with schema_migrations table
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}

		// Create schema_migrations table
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			dirty BOOLEAN,
			applied_at TIMESTAMP default CURRENT_TIMESTAMP
		)`)
		if err != nil {
			t.Fatalf("failed to create schema_migrations table: %v", err)
		}

		// Insert outdated version
		_, err = db.Exec("INSERT INTO schema_migrations (version, dirty) VALUES (1, false)")
		if err != nil {
			t.Fatalf("failed to insert schema version: %v", err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}

		// Capture stderr
		output := captureStderr(func() {
			// Create manager which should trigger version check
			manager, err := NewManagerReadWrite(dbPath)
			if err != nil {
				t.Fatalf("NewManagerReadWrite failed: %v", err)
			}
			defer func() {
				if err := manager.Close(); err != nil {
					t.Fatalf("failed to close manager: %v", err)
				}
			}()
		})

		// Verify warning message
		if !strings.Contains(output, "Warning: Database schema version mismatch") {
			t.Errorf("expected warning message, got: %s", output)
		}
		if !strings.Contains(output, "Please run 'duckhist schema-migrate'") {
			t.Errorf("expected migration instruction, got: %s", output)
		}
	})

	t.Run("no warning message when schema version is up to date", func(t *testing.T) {
		// Remove previous database
		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			t.Errorf("failed to remove database: %v", err)
		}

		// Create a new database with up-to-date schema version
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}

		// Create schema_migrations table
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			dirty BOOLEAN,
			applied_at TIMESTAMP default CURRENT_TIMESTAMP
		)`)
		if err != nil {
			t.Fatalf("failed to create schema_migrations table: %v", err)
		}

		// Get latest migration version from migrate package
		latestVersion := 3 // Hardcoded to 3 based on current migrations

		// Insert latest version
		_, err = db.Exec("INSERT INTO schema_migrations (version, dirty) VALUES (?, false)", latestVersion)
		if err != nil {
			t.Fatalf("failed to insert schema version: %v", err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}

		// Capture stderr
		output := captureStderr(func() {
			// Create manager which should trigger version check
			manager, err := NewManagerReadWrite(dbPath)
			if err != nil {
				t.Fatalf("NewManagerReadWrite failed: %v", err)
			}
			defer func() {
				if err := manager.Close(); err != nil {
					t.Fatalf("failed to close manager: %v", err)
				}
			}()
		})

		// Verify no warning message
		if strings.Contains(output, "Warning: Database schema version mismatch") {
			t.Errorf("unexpected warning message: %s", output)
		}
	})
}
