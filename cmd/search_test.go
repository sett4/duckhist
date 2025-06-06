package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sett4/duckhist/internal/history"
)

func TestSearchCommand(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "duckhist-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to remove temp directory: %v", err)
		}
	}()

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

	// Initialize the database with migrations
	if err := RunMigrations(dbPath); err != nil {
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
		isDup, err := manager.AddCommand(tc.command, tc.directory, "", "", "localhost", "testuser", time.Now(), false)
		if err != nil {
			t.Fatalf("Failed to add command: %v", err)
		}
		if isDup {
			t.Errorf("Expected command not to be duplicate: %s", tc.command)
		}
	}
	if err := manager.Close(); err != nil {
		t.Errorf("failed to close manager: %v", err)
	}

	// Test the search functionality
	// Note: We can't fully test the interactive UI in a unit test,
	// but we can verify that the command doesn't error out
	// and that the database queries work correctly

	// Test that the FindByCommand method works
	manager, err = history.NewManagerReadOnly(dbPath)
	if err != nil {
		t.Fatalf("Failed to create history manager: %v", err)
	}
	defer func() {
		if err := manager.Close(); err != nil {
			t.Errorf("failed to close manager: %v", err)
		}
	}()

	// Test empty query (should return all commands)
	results, err := manager.FindByCommand("", currentDir)
	if err != nil {
		t.Fatalf("FindByCommand failed: %v", err)
	}
	if len(results) != len(testCommands) {
		t.Errorf("Expected %d results, got %d", len(testCommands), len(results))
	}

	// Test specific query
	results, err = manager.FindByCommand("git", currentDir)
	if err != nil {
		t.Fatalf("FindByCommand failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if len(results) > 0 && results[0].Command != "git status" {
		t.Errorf("Expected 'git status', got '%s'", results[0].Command)
	}

	// Test that current directory commands come first
	results, err = manager.FindHistory(currentDir, nil)
	if err != nil {
		t.Fatalf("FindHistory failed: %v", err)
	}
	if len(results) < 3 || results[0].Directory != currentDir {
		t.Errorf("Current directory commands should be listed first")
	}
}

func TestShortenPath(t *testing.T) {
	// Test cases
	thuruTests := []struct {
		path      string
		maxLength int
		expected  string
	}{
		{"/home/user/documents", 50, "/home/user/documents"},
		{"/home/user/documents/../file.txt", 50, "/home/user/file.txt"},
		{"./documents/file.txt", 50, "documents/file.txt"},
		{"/home/user/documents/file.txt", 50, "/home/user/documents/file.txt"},
		{"/home/user/documents/file.txt", 23, "/h/u/documents/file.txt"},
		{"/home/user/documents/file.txt", 20, "/h/u/d/file.txt"},
		{"/home/user/documents/file.txt", 14, ".../d/file.txt"},
		{"/home/user/documents/file.txt", 05, ".../file.txt"},
	}

	for _, test := range thuruTests {
		result := ShortenPath(test.path, test.maxLength)
		if result != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, result)
		}
	}
}
