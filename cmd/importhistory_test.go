package cmd

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sett4/duckhist/internal/config"
	"github.com/sett4/duckhist/internal/history"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// Helper function to capture stdout
func captureOutput(f func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdErr := f()

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = oldStdout

	return string(out), cmdErr
}

// Helper to create a temporary config file for tests
func createTempConfigFile(t *testing.T, dbPath string) string {
	t.Helper()
	cfgContent := fmt.Sprintf("database_path: %s\n", dbPath)
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	assert.NoError(t, err)
	_, err = tmpFile.WriteString(cfgContent)
	assert.NoError(t, err)
	assert.NoError(t, tmpFile.Close())
	return tmpFile.Name()
}

// Helper to create a dummy history file
func createDummyHistoryFile(t *testing.T, dir string, content string) string {
	t.Helper()
	historyFilePath := filepath.Join(dir, ".zsh_history")
	err := os.WriteFile(historyFilePath, []byte(content), 0644)
	assert.NoError(t, err)
	return historyFilePath
}

// TestImportHistory_BasicImport tests the basic functionality of import-history.
func TestImportHistory_BasicImport(t *testing.T) {
	// 1. Setup a Temporary Test Environment
	tempHomeDir := t.TempDir()
	tempDbDir := t.TempDir()
	tempDbPath := filepath.Join(tempDbDir, "test_history.db")

	// Override os.UserHomeDir for this test
	originalGetUserHomeDir := getUserHomeDir
	getUserHomeDir = func() (string, error) { return tempHomeDir, nil }
	t.Cleanup(func() { getUserHomeDir = originalGetUserHomeDir })

	// Create temp config file
	tempCfgFile := createTempConfigFile(t, tempDbPath)
	originalCfgFile := cfgFile // Assuming cfgFile is accessible and can be reset
	cfgFile = tempCfgFile
	t.Cleanup(func() { cfgFile = originalCfgFile })
	
	// Create sample .zsh_history
	historyContent := `
: 1678886400:0;ls -l
echo "hello world"
: 1678886401:0;   cd /tmp   
: 123:0;
: invalid_timestamp:0;echo "bad time"
    
# A comment line, should be skipped if logic implies, but current parser might treat as command
: 1678886402:0;git status
`
	_ = createDummyHistoryFile(t, tempHomeDir, historyContent)

	// Execute the command
	rootCmd.SetArgs([]string{"import-history"})
	output, err := captureOutput(rootCmd.ExecuteC) // ExecuteC to get the command itself for error checking
	
	assert.NoError(t, err)
	t.Logf("Command output: %s", output)

	// Assertions
	expectedImportCount := 5 // "ls -l", "echo "hello world"", "cd /tmp", "echo "bad time"", "git status"
	                         // Empty line and ": 123:0;" (empty command) are skipped.
	assert.Contains(t, output, fmt.Sprintf("Imported %d commands from %s", expectedImportCount, filepath.Join(tempHomeDir, ".zsh_history")))

	// Verify database content
	db, err := sql.Open("sqlite3", tempDbPath)
	assert.NoError(t, err)
	defer db.Close()

	rows, err := db.Query("SELECT command, executed_at, no_dedup FROM history ORDER BY executed_at ASC")
	assert.NoError(t, err)
	defer rows.Close()

	var commands []struct {
		Command    string
		ExecutedAt int64
		NoDedup    bool
	}
	for rows.Next() {
		var cmdText string
		var executedAt int64
		var noDedup bool
		err = rows.Scan(&cmdText, &executedAt, &noDedup)
		assert.NoError(t, err)
		commands = append(commands, struct {
			Command    string
			ExecutedAt int64
			NoDedup    bool
		}{cmdText, executedAt, noDedup})
	}
	assert.NoError(t, rows.Err())
	assert.Len(t, commands, expectedImportCount, "Number of commands in DB does not match")

	// Check specific commands
	// 1. ls -l
	assert.Equal(t, "ls -l", commands[0].Command)
	assert.Equal(t, int64(1678886400), commands[0].ExecutedAt)
	assert.True(t, commands[0].NoDedup)

	// 2. cd /tmp
	assert.Equal(t, "cd /tmp", commands[1].Command)
	assert.Equal(t, int64(1678886401), commands[1].ExecutedAt)
	assert.True(t, commands[1].NoDedup)
	
	// 3. git status
	assert.Equal(t, "git status", commands[2].Command)
	assert.Equal(t, int64(1678886402), commands[2].ExecutedAt)
	assert.True(t, commands[2].NoDedup)

	// 4. echo "hello world" (no timestamp in file)
	assert.Equal(t, "echo \"hello world\"", commands[3].Command)
	assert.WithinDuration(t, time.Now(), time.Unix(commands[3].ExecutedAt, 0), 5*time.Second, "Timestamp for 'echo \"hello world\"' should be recent")
	assert.True(t, commands[3].NoDedup)
	
	// 5. echo "bad time" (invalid timestamp in file)
	assert.Equal(t, "echo \"bad time\"", commands[4].Command)
	assert.WithinDuration(t, time.Now(), time.Unix(commands[4].ExecutedAt, 0), 5*time.Second, "Timestamp for 'echo \"bad time\"' should be recent")
	assert.True(t, commands[4].NoDedup)
}


// TestImportHistory_FileNotFound tests behavior when history file is not found.
func TestImportHistory_FileNotFound(t *testing.T) {
	tempHomeDir := t.TempDir()
	tempDbDir := t.TempDir()
	tempDbPath := filepath.Join(tempDbDir, "test_empty.db")

	originalGetUserHomeDir := getUserHomeDir
	getUserHomeDir = func() (string, error) { return tempHomeDir, nil }
	t.Cleanup(func() { getUserHomeDir = originalGetUserHomeDir })

	tempCfgFile := createTempConfigFile(t, tempDbPath)
	originalCfgFile := cfgFile
	cfgFile = tempCfgFile
	t.Cleanup(func() { cfgFile = originalCfgFile })

	// Ensure history file does NOT exist (it shouldn't by default in tempHomeDir)

	rootCmd.SetArgs([]string{"import-history"})
	output, err := captureOutput(rootCmd.ExecuteC)

	assert.NoError(t, err) // The command itself should not error, just print a message
	t.Logf("Command output: %s", output)
	assert.Contains(t, output, fmt.Sprintf("History file not found: %s", filepath.Join(tempHomeDir, ".zsh_history")))

	// Verify database is empty or not created (if manager handles that)
	// For this test, we'll ensure it's empty if created.
	db, err := sql.Open("sqlite3", tempDbPath)
	if err == nil { // if db was created
		defer db.Close()
		rows, errDb := db.Query("SELECT COUNT(*) FROM history")
		assert.NoError(t, errDb)
		var count int
		if rows.Next() {
			errDb = rows.Scan(&count)
			assert.NoError(t, errDb)
		}
		rows.Close()
		assert.Equal(t, 0, count, "Database should be empty if history file not found")
	} else {
		// If the db file itself is not found, that's also acceptable if the manager doesn't create it.
		// However, NewManagerReadWrite likely creates it.
		assert.NoError(t, err, "Opening the database should not fail, even if it's empty")
	}
}

// Mock for the global cfgFile variable in cmd package
// This assumes cfgFile is accessible. If it's not, this won't work
// and config loading needs to be handled differently, perhaps by passing config directly.
// var cfgFile string // This would be in cmd/root.go or similar

// This is a placeholder for the actual getUserHomeDir variable
// that would need to be defined in cmd/importhistory.go
var getUserHomeDir func() (string, error) = os.UserHomeDir
