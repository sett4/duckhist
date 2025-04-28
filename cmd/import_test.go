package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sett4/duckhist/internal/history"
)

func TestRunImport(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create config file
	configPath := filepath.Join(tmpDir, "config.toml")
	dbPath := filepath.Join(tmpDir, "test.sqlite")
	content := fmt.Sprintf("database_path = %q", dbPath)
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	// Run migrations to initialize database schema
	if err := RunMigrations(dbPath); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	t.Run("successful import", func(t *testing.T) {
		// Create test CSV file
		csvPath := filepath.Join(tmpDir, "test.csv")
		csvContent := `id,command,executed_at,executing_host,executing_dir,executing_user,sid,tty
,ls -la,2024-01-01T12:00:00Z,testhost,/test/dir,testuser,123,/dev/pts/1
,pwd,2024-01-01T12:01:00Z,testhost,/test/dir,testuser,123,/dev/pts/1
`
		if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
			t.Fatalf("failed to create CSV file: %v", err)
		}

		// Set up command flags
		cfgFile = configPath
		importFile = csvPath

		// Run import command
		if err := runImport(nil, nil); err != nil {
			t.Fatalf("runImport failed: %v", err)
		}

		// Verify imported commands
		manager, err := history.NewManagerReadOnly(dbPath)
		if err != nil {
			t.Fatalf("failed to create history manager: %v", err)
		}
		defer func() {
			if err := manager.Close(); err != nil {
				t.Errorf("failed to close manager: %v", err)
			}
		}()

		entries, err := manager.Query().GetEntries()
		if err != nil {
			t.Fatalf("failed to get entries: %v", err)
		}

		if len(entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(entries))
		}

		expected := map[string]struct{}{
			"ls -la": {},
			"pwd":    {},
		}

		for _, entry := range entries {
			if _, ok := expected[entry.Command]; !ok {
				t.Errorf("unexpected command: %s", entry.Command)
			}
			if !entry.Timestamp.Equal(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)) &&
				!entry.Timestamp.Equal(time.Date(2024, 1, 1, 12, 1, 0, 0, time.UTC)) {
				t.Errorf("unexpected timestamp: %v", entry.Timestamp)
			}
			if entry.Hostname != "testhost" {
				t.Errorf("expected hostname 'testhost', got %q", entry.Hostname)
			}
			if entry.Directory != "/test/dir" {
				t.Errorf("expected directory '/test/dir', got %q", entry.Directory)
			}
			if entry.Username != "testuser" {
				t.Errorf("expected username 'testuser', got %q", entry.Username)
			}
			if entry.SID != "123" {
				t.Errorf("expected SID '123', got %q", entry.SID)
			}
			if entry.TTY != "/dev/pts/1" {
				t.Errorf("expected TTY '/dev/pts/1', got %q", entry.TTY)
			}
		}
	})

	t.Run("missing required column", func(t *testing.T) {
		// Create test CSV file without required column
		csvPath := filepath.Join(tmpDir, "test_missing_column.csv")
		csvContent := `id,executed_at,executing_host,executing_dir,executing_user,sid,tty
1,2024-01-01T12:00:00Z,testhost,/test/dir,testuser,123,/dev/pts/1
`
		if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
			t.Fatalf("failed to create CSV file: %v", err)
		}

		// Set up command flags
		cfgFile = configPath
		importFile = csvPath

		// Run import command
		if err := runImport(nil, nil); err == nil {
			t.Error("expected error for missing required column")
		}
	})

	t.Run("invalid CSV file", func(t *testing.T) {
		// Set up command flags
		cfgFile = configPath
		importFile = "nonexistent.csv"

		// Run import command
		if err := runImport(nil, nil); err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}