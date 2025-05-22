package cmd

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	// "github.com/sett4/duckhist/internal/migrate" // Not needed directly, using cmd.RunMigrations
	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"github.com/stretchr/testify/assert"
)

// Helper to initialize the database schema for tests
func initializeTestDB(t *testing.T, dbPath string) {
	t.Helper()
	// cmd.RunMigrations (from schema-migrate.go) handles DB opening/closing and migrations.
	err := RunMigrations(dbPath)
	assert.NoError(t, err, "Failed to run migrations on test DB: %s", dbPath)
}

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
	// TOML format requires strings to be quoted
	cfgContent := fmt.Sprintf("database_path = %q\n", dbPath) // Use %q for proper TOML string quoting
	// Even though viper is forced to TOML by config.LoadConfig, let's use a .toml extension for clarity
	tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.toml")
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

	// Initialize schema for the test database
	initializeTestDB(t, tempDbPath)

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
: 1678886400:0;ls -l 
: 1678886403:0;echo "another command"
echo "hello world" 
`
	// Expected:
	// "ls -l" (ts 1678886400) - import
	// "echo "hello world"" (no ts) - import
	// "cd /tmp" (ts 1678886401) - import
	// "" (empty from ": 123:0;") - skip (empty command)
	// "echo "bad time"" (invalid ts) - import
	// "# A comment line..." (no ts) - import
	// "git status" (ts 1678886402) - import
	// "ls -l" (ts 1678886400) - skip (duplicate of first command)
	// "echo "another command"" (ts 1678886403) - import
	// "echo "hello world"" (no ts) - skip (duplicate of second command, timestamp doesn't matter for no_dedup=false with same command text)

	_ = createDummyHistoryFile(t, tempHomeDir, historyContent)

	// Execute the command
	t.Logf("Available commands before SetArgs for BasicImport: %v", len(rootCmd.Commands()))
	for _, c := range rootCmd.Commands() {
		t.Logf("Command: %s", c.Use)
	}
	rootCmd.SetArgs([]string{"import-history"})
	output, err := captureOutput(func() error {
		_, cmdErr := rootCmd.ExecuteC()
		return cmdErr
	})
	
	assert.NoError(t, err)
	t.Logf("Command output: %s", output)

	// Assertions
	expectedImportCount := 7
	expectedSkippedCount := 2 
	historyFilePath := filepath.Join(tempHomeDir, ".zsh_history")
	assert.Contains(t, output, fmt.Sprintf("Imported %d commands and skipped %d duplicate commands from %s", expectedImportCount, expectedSkippedCount, historyFilePath))

	// Verify database content
	db, err := sql.Open("sqlite3", tempDbPath)
	assert.NoError(t, err)
	defer db.Close()

	rows, err := db.Query("SELECT command, executed_at FROM history ORDER BY executed_at ASC") // Removed no_dedup
	assert.NoError(t, err)
	defer rows.Close()

	var commands []struct {
		Command    string
		ExecutedAt time.Time // Changed to time.Time
	}
	for rows.Next() {
		var cmdText string
		var executedAtTime time.Time // Scan into time.Time
		err = rows.Scan(&cmdText, &executedAtTime)
		assert.NoError(t, err)
		commands = append(commands, struct {
			Command    string
			ExecutedAt time.Time
		}{cmdText, executedAtTime})
	}
	assert.NoError(t, rows.Err())
	assert.Len(t, commands, expectedImportCount, "Number of commands in DB does not match")

	// Check specific commands (order might change due to comment line, adjust if needed)
	// Timestamps are primary sort key. Non-timestamped entries (like the comment) get current time.
	// So, timestamped entries should come first.

	// 1. ls -l
	assert.Equal(t, "ls -l", commands[0].Command)
	assert.Equal(t, int64(1678886400), commands[0].ExecutedAt.Unix()) // Use .Unix()

	// 2. cd /tmp
	assert.Equal(t, "cd /tmp", commands[1].Command)
	assert.Equal(t, int64(1678886401), commands[1].ExecutedAt.Unix()) // Use .Unix()
	
	// 3. git status
	assert.Equal(t, "git status", commands[2].Command)
	assert.Equal(t, int64(1678886402), commands[2].ExecutedAt.Unix())

	// 4. echo "another command"
	assert.Equal(t, "echo \"another command\"", commands[3].Command)
	assert.Equal(t, int64(1678886403), commands[3].ExecutedAt.Unix())


	// The remaining 3 commands ("echo "hello world"", "# A comment...", "echo "bad time"")
	// will have timestamps set to time.Now() during import.
	// Their relative order among themselves isn't strictly guaranteed by ORDER BY executed_at
	// if they are imported within the same second.
	// We'll check for their presence and approximate timestamp.

	nonTimestampedCommands := make(map[string]bool)
	now := time.Now()
	for i := 4; i < expectedImportCount; i++ { // Start from index 4 for non-timestamped
		cmd := commands[i]
		assert.WithinDuration(t, now, cmd.ExecutedAt, 5*time.Second, "Timestamp for non-timestamped entry should be recent: %s", cmd.Command)
		nonTimestampedCommands[cmd.Command] = true
	}

	assert.True(t, nonTimestampedCommands["echo \"hello world\""], "Command 'echo \"hello world\"' not found or in wrong group")
	assert.True(t, nonTimestampedCommands["# A comment line, should be skipped if logic implies, but current parser might treat as command"], "Command '# A comment line...' not found or in wrong group")
	assert.True(t, nonTimestampedCommands["echo \"bad time\""], "Command 'echo \"bad time\"' not found or in wrong group")
}


// TestImportHistory_Deduplication tests the deduplication logic more specifically.
func TestImportHistory_Deduplication(t *testing.T) {
	tempHomeDir := t.TempDir()
	tempDbDir := t.TempDir()
	tempDbPath := filepath.Join(tempDbDir, "test_dedup.db")

	originalGetUserHomeDir := getUserHomeDir
	getUserHomeDir = func() (string, error) { return tempHomeDir, nil }
	t.Cleanup(func() { getUserHomeDir = originalGetUserHomeDir })

	tempCfgFile := createTempConfigFile(t, tempDbPath)
	originalCfgFile := cfgFile
	cfgFile = tempCfgFile
	t.Cleanup(func() { cfgFile = originalCfgFile })

	initializeTestDB(t, tempDbPath)

	historyContent := `
: 100:0;command1
: 200:0;command2
: 100:0;command1 
: 300:0;command3
command2 
: 400:0;command1
command4
command4
`
	// Expected:
	// command1 (ts 100) - import
	// command2 (ts 200) - import
	// command1 (ts 100) - skip (exact duplicate)
	// command3 (ts 300) - import
	// command2 (no ts) - skip (duplicate of command2, different timestamp but same command text)
	// command1 (ts 400) - skip (text "command1" already seen, noDedup=false means only text matters)
	// command4 (no ts) - import (first encounter of "command4")
	// command4 (no ts) - skip (text "command4" already seen)

	_ = createDummyHistoryFile(t, tempHomeDir, historyContent)
	historyFilePath := filepath.Join(tempHomeDir, ".zsh_history")

	rootCmd.SetArgs([]string{"import-history"})
	output, err := captureOutput(func() error {
		_, cmdErr := rootCmd.ExecuteC()
		return cmdErr
	})

	assert.NoError(t, err)
	t.Logf("Deduplication test output: %s", output)

	expectedImportCount := 4 // Corrected
	expectedSkippedCount := 4 // Corrected
	assert.Contains(t, output, fmt.Sprintf("Imported %d commands and skipped %d duplicate commands from %s", expectedImportCount, expectedSkippedCount, historyFilePath))

	db, errDb := sql.Open("sqlite3", tempDbPath)
	assert.NoError(t, errDb)
	defer db.Close()

	rows, errDb := db.Query("SELECT command, executed_at FROM history ORDER BY executed_at ASC, rowid ASC") // rowid for stable sort for no-ts
	assert.NoError(t, errDb)
	defer rows.Close()

	var importedCommands []struct {
		Command    string
		ExecutedAt time.Time
	}
	for rows.Next() {
		var cmdText string
		var executedAtTime time.Time
		errDb = rows.Scan(&cmdText, &executedAtTime)
		assert.NoError(t, errDb)
		importedCommands = append(importedCommands, struct {
			Command    string
			ExecutedAt time.Time
		}{cmdText, executedAtTime})
	}
	assert.NoError(t, rows.Err())
	assert.Len(t, importedCommands, expectedImportCount, "Number of commands in DB does not match expected import count")

	// Verify imported commands and their timestamps
	// 1. command1 (ts 100)
	assert.Equal(t, "command1", importedCommands[0].Command)
	assert.Equal(t, int64(100), importedCommands[0].ExecutedAt.Unix())

	// 2. command2 (ts 200)
	assert.Equal(t, "command2", importedCommands[1].Command)
	assert.Equal(t, int64(200), importedCommands[1].ExecutedAt.Unix())

	// 3. command3 (ts 300)
	assert.Equal(t, "command3", importedCommands[2].Command)
	assert.Equal(t, int64(300), importedCommands[2].ExecutedAt.Unix())
	
	// 4. command4 (no ts - should be recent)
	// This is the first "command4" from the history file.
	assert.Equal(t, "command4", importedCommands[3].Command)
	assert.WithinDuration(t, time.Now(), importedCommands[3].ExecutedAt, 5*time.Second, "Timestamp for 'command4' should be recent")

	// Verify that "command1" with ts 100 was imported.
	// Verify that "command1" with ts 400 was SKIPPED.
	// Verify that "command2" without timestamp was SKIPPED.
	// And second "command4" was skipped
	// This is implicitly checked by expectedImportCount and the specific checks above.
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

	// Initialize schema even for file not found, as the manager might still be created
	// and check schema version on an empty (but existing) db file.
	initializeTestDB(t, tempDbPath)

	// Ensure history file does NOT exist (it shouldn't by default in tempHomeDir)

	t.Logf("Available commands before SetArgs for FileNotFound: %v", len(rootCmd.Commands()))
	for _, c := range rootCmd.Commands() {
		t.Logf("Command: %s", c.Use)
	}
	rootCmd.SetArgs([]string{"import-history"})
	output, err := captureOutput(func() error {
		_, cmdErr := rootCmd.ExecuteC()
		return cmdErr
	})

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

// The getUserHomeDir variable is defined in cmd/importhistory.go
// Tests will assign to it directly.
