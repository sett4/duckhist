package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"duckhist/internal/history"
)

func TestSearchCommand(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "duckhist-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test database file
	dbPath := filepath.Join(tempDir, "test.duckdb")

	// Create a test config file
	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
database_path = "` + dbPath + `"
current_directory_history_limit = 5
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Set the config file path for the test
	cfgFile = configPath

	// Initialize the database
	if err := initializeTestDatabase(dbPath); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Add some test commands
	manager, err := history.NewManagerReadWrite(dbPath)
	if err != nil {
		t.Fatalf("Failed to create history manager: %v", err)
	}

	// Current directory
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Add test commands
	testCommands := []struct {
		command   string
		directory string
	}{
		{"ls -la", currentDir},
		{"git status", currentDir},
		{"go build", currentDir},
		{"echo hello", "/tmp"},
		{"cat file.txt", "/tmp"},
	}

	for _, tc := range testCommands {
		if err := manager.AddCommand(tc.command, tc.directory, "", "", "localhost", "testuser"); err != nil {
			t.Fatalf("Failed to add command: %v", err)
		}
	}
	manager.Close()

	// Test the search functionality
	// Note: We can't fully test the interactive UI in a unit test,
	// but we can verify that the command doesn't error out
	// and that the database queries work correctly

	// Test that the SearchCommands method works
	manager, err = history.NewManagerReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Failed to create history manager: %v", err)
	}
	defer manager.Close()

	// Test empty query (should return all commands)
	results, err := manager.SearchCommands("", currentDir)
	if err != nil {
		t.Fatalf("SearchCommands failed: %v", err)
	}
	if len(results) != len(testCommands) {
		t.Errorf("Expected %d results, got %d", len(testCommands), len(results))
	}

	// Test specific query
	results, err = manager.SearchCommands("git", currentDir)
	if err != nil {
		t.Fatalf("SearchCommands failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if len(results) > 0 && results[0].Command != "git status" {
		t.Errorf("Expected 'git status', got '%s'", results[0].Command)
	}

	// Test that current directory commands come first
	results, err = manager.GetAllHistory(currentDir)
	if err != nil {
		t.Fatalf("GetAllHistory failed: %v", err)
	}
	if len(results) < 3 || results[0].Directory != currentDir {
		t.Errorf("Current directory commands should be listed first")
	}
}

// Helper function to initialize a test database
func initializeTestDatabase(dbPath string) error {
	// Open the database directly
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Execute the schema creation SQL
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS history (
			id UUID PRIMARY KEY,
			command TEXT NOT NULL,
			executed_at TIMESTAMP NOT NULL,
			executing_host TEXT NOT NULL,
			executing_dir TEXT NOT NULL,
			executing_user TEXT NOT NULL,
			tty TEXT,
			sid TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_history_executed_at ON history (executed_at);
		CREATE INDEX IF NOT EXISTS idx_history_executing_dir ON history (executing_dir);
	`)
	return err
}
