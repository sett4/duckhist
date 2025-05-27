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

func TestHistoryQuery_Search_KQL(t *testing.T) {
	// Helper function to compare string slices
	compareStringSlices := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	// Helper function to compare interface slices
	compareInterfaceSlices := func(a, b []interface{}) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if fmt.Sprintf("%v", a[i]) != fmt.Sprintf("%v", b[i]) {
				return false
			}
		}
		return true
	}

	tests := []struct {
		name              string
		query             string
		expectedConditions []string
		expectedArgs      []interface{}
		expectParseError  bool
	}{
		{
			name:              "simple term",
			query:             "my query",
			expectedConditions: []string{"command LIKE ?"},
			expectedArgs:      []interface{}{"%my query%"},
		},
		{
			name:              "command field",
			query:             "command:my_command",
			expectedConditions: []string{"command LIKE ?"},
			expectedArgs:      []interface{}{"%my_command%"},
		},
		{
			name:              "dir field",
			query:             "dir:/some/path",
			expectedConditions: []string{"executing_dir LIKE ?"},
			expectedArgs:      []interface{}{"%/some/path%"},
		},
		{
			name:              "directory field",
			query:             "directory:/another/path",
			expectedConditions: []string{"executing_dir LIKE ?"},
			expectedArgs:      []interface{}{"%/another/path%"},
		},
		{
			name:              "host field",
			query:             "host:my_hostname",
			expectedConditions: []string{"executing_host LIKE ?"},
			expectedArgs:      []interface{}{"%my_hostname%"},
		},
		{
			name:              "hostname field",
			query:             "hostname:other_host",
			expectedConditions: []string{"executing_host LIKE ?"},
			expectedArgs:      []interface{}{"%other_host%"},
		},
		{
			name:  "combination of terms",
			query: "term1 command:cmd_term dir:/d hostname:h",
			expectedConditions: []string{
				"command LIKE ?",
				"command LIKE ?",
				"executing_dir LIKE ?",
				"executing_host LIKE ?",
			},
			expectedArgs: []interface{}{
				"%term1%",
				"%cmd_term%",
				"%/d%",
				"%h%",
			},
		},
		{
			name:              "quoted simple term",
			query:             `"my command with spaces"`,
			expectedConditions: []string{"command LIKE ?"},
			expectedArgs:      []interface{}{"%my command with spaces%"},
		},
		{
			name:              "parsing error fallback",
			query:             "command: value_without_quotes_or_colon_after_field",
			expectedConditions: []string{"command LIKE ?"},
			expectedArgs:      []interface{}{"%command: value_without_quotes_or_colon_after_field%"},
			expectParseError:  true,
		},
		{
			name:              "empty query",
			query:             "",
			expectedConditions: nil, // Or []string{} depending on initialization
			expectedArgs:      nil, // Or []interface{}{}
		},
		{
			name:  "term with colon not as field",
			query: "term:with:colon",
			expectedConditions: []string{"command LIKE ?"},
			expectedArgs:      []interface{}{"%term:with:colon%"},
		},
		{
			name:  "field then simple term",
			query: "command:cmd term2",
			expectedConditions: []string{"command LIKE ?", "command LIKE ?"},
			expectedArgs:      []interface{}{"%cmd%", "%term2%"},
		},
		{
			name: "KQL command field with quoted value",
			query: `command:"quoted command"`,
			expectedConditions: []string{"command LIKE ?"},
			expectedArgs: []interface{}{"%quoted command%"},
		},
		{
			name: "KQL dir field with quoted value",
			query: `dir:"/quoted/path with spaces"`,
			expectedConditions: []string{"executing_dir LIKE ?"},
			expectedArgs: []interface{}{"%\"/quoted/path with spaces\"%"}, // Note: The unquote is for the parser, not for the LIKE value.
		},
		{
			name: "KQL host field with quoted value",
			query: `host:"quoted hostname"`,
			expectedConditions: []string{"executing_host LIKE ?"},
			expectedArgs: []interface{}{"%quoted hostname%"},
		},
		{
            name:  "multiple simple terms and field terms",
            query: `term1 command:cmd dir:"/some dir/" host:myhost term2`,
            expectedConditions: []string{
                "command LIKE ?",
                "command LIKE ?",
                "executing_dir LIKE ?",
                "executing_host LIKE ?",
                "command LIKE ?",
            },
            expectedArgs: []interface{}{
                "%term1%",
                "%cmd%",
                "%\"/some dir/\"%",
                "%myhost%",
                "%term2%",
            },
        },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hq := &HistoryQuery{} // No manager needed as we are testing Search's direct effect

			var stderrOutput string
			if tt.expectParseError {
				stderrOutput = captureStderr(func() {
					hq.Search(tt.query)
				})
			} else {
				hq.Search(tt.query)
			}

			if tt.expectParseError {
				if !strings.Contains(stderrOutput, "KQL parsing error") {
					t.Errorf("expected KQL parsing error message on stderr, got: %s", stderrOutput)
				}
			} else {
				if strings.Contains(stderrOutput, "KQL parsing error") {
					t.Errorf("unexpected KQL parsing error message on stderr: %s", stderrOutput)
				}
			}

			if !compareStringSlices(hq.conditions, tt.expectedConditions) {
				t.Errorf("expected conditions %v, got %v", tt.expectedConditions, hq.conditions)
			}
			if !compareInterfaceSlices(hq.args, tt.expectedArgs) {
				t.Errorf("expected args %v, got %v", tt.expectedArgs, hq.args)
			}
		})
	}
}
