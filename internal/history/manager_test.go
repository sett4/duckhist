package history

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sett4/duckhist/internal/migrate"
	_ "github.com/mattn/go-sqlite3"
	"github.com/oklog/ulid/v2"
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

func TestParseSearchQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  [][]SearchTermCondition
	}{
		{
			name:  "empty string",
			query: "",
			want:  nil,
		},
		{
			name:  "single term",
			query: "term1",
			want:  [][]SearchTermCondition{{{Term: "term1", IsNegated: false}}},
		},
		{
			name:  "implicit AND",
			query: "term1 term2",
			want:  [][]SearchTermCondition{{{Term: "term1", IsNegated: false}, {Term: "term2", IsNegated: false}}},
		},
		{
			name:  "explicit OR",
			query: "term1 OR term2",
			want:  [][]SearchTermCondition{{{Term: "term1", IsNegated: false}}, {{Term: "term2", IsNegated: false}}},
		},
		{
			name:  "explicit NOT",
			query: "NOT term1",
			want:  [][]SearchTermCondition{{{Term: "term1", IsNegated: true}}},
		},
		{
			name:  "combined",
			query: "term1 term2 OR term3 NOT term4",
			want: [][]SearchTermCondition{
				{{Term: "term1", IsNegated: false}, {Term: "term2", IsNegated: false}},
				{{Term: "term3", IsNegated: false}, {Term: "term4", IsNegated: true}},
			},
		},
		{
			name:  "edge case OR only",
			query: "OR",
			want:  [][]SearchTermCondition{}, // Or nil, depending on strictness. Current parser makes it empty.
		},
		{
			name:  "edge case NOT only",
			query: "NOT",
			// Based on current ParseSearchQuery, "NOT" alone is treated as a literal term if not followed by another.
			// If we want it to be empty, ParseSearchQuery needs adjustment.
			// For now, testing current behavior.
			want: [][]SearchTermCondition{{{Term: "NOT", IsNegated: false}}},
		},
		{
			name:  "edge case term1 OR", // Trailing OR
			query: "term1 OR",
			want:  [][]SearchTermCondition{{{Term: "term1", IsNegated: false}}}, // "OR" is ignored
		},
		{
			name:  "edge case term1 NOT", // Trailing NOT
			query: "term1 NOT",
			// "NOT" is treated as a literal term here by strings.Fields if it's the last field.
			want: [][]SearchTermCondition{{{Term: "term1", IsNegated: false}, {Term: "NOT", IsNegated: false}}},
		},
		{
			name:  "multiple ORs",
			query: "term1 OR term2 OR term3",
			want: [][]SearchTermCondition{
				{{Term: "term1", IsNegated: false}},
				{{Term: "term2", IsNegated: false}},
				{{Term: "term3", IsNegated: false}},
			},
		},
		{
			name:  "NOT at end of AND group (actually middle)",
			query: "term1 NOT term2 term3",
			want: [][]SearchTermCondition{
				{{Term: "term1", IsNegated: false}, {Term: "term2", IsNegated: true}, {Term: "term3", IsNegated: false}},
			},
		},
		{
			name:  "NOT with leading/trailing spaces",
			query: "  NOT  term1  ",
			want:  [][]SearchTermCondition{{{Term: "term1", IsNegated: true}}},
		},
		{
			name:  "complex with multiple NOTs",
			query: "NOT term1 OR NOT term2 term3 NOT term4",
			want: [][]SearchTermCondition{
				{{Term: "term1", IsNegated: true}},
				{{Term: "term2", IsNegated: true}, {Term: "term3", IsNegated: false}, {Term: "term4", IsNegated: true}},
			},
		},
		{
            name:  "empty query string",
            query: "",
            want:  nil,
        },
        {
            name:  "query with only spaces",
            query: "   ",
            // strings.Fields will result in an empty slice, leading to nil from ParseSearchQuery
            want:  nil, 
        },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSearchQuery(tt.query)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSearchQuery(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

// setupInMemoryTestDB creates an in-memory SQLite database and applies the schema.
func setupInMemoryTestDB(t *testing.T) *Manager {
	t.Helper()

	// Using ":memory:" for in-memory SQLite database
	// Adding "?_foreign_keys=on" to ensure foreign key constraints are enabled,
	// and cache=shared to allow potential multiple connections if needed (though not strictly for this test).
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on&cache=shared")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}

	// Apply migrations
	err = migrate.Migrate(db, migrate.BuiltinMigrations())
	if err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}
	
	// Check schema version (optional, but good practice)
	// We don't need to capture stderr here as it's a test setup.
	ok, current, required, errCheck := migrate.CheckSchemaVersion(db)
	if errCheck != nil {
		t.Logf("Warning: Failed to check schema version during test setup: %v", errCheck)
	} else if !ok {
		t.Logf("Warning: Schema version mismatch in test DB. Current: %d, Required: %d", current, required)
	}


	manager := &Manager{db: db}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("failed to close manager: %v", err)
		}
	})

	return manager
}

// normalizeEntriesForComparison extracts commands and sorts them for stable comparison.
func normalizeEntriesForComparison(entries []Entry) []string {
	cmds := make([]string, len(entries))
	for i, e := range entries {
		cmds[i] = e.Command
	}
	sort.Strings(cmds) // Sort for stable comparison
	return cmds
}


func TestFindByCommand_MultiKeyword(t *testing.T) {
	manager := setupInMemoryTestDB(t)
	now := time.Now()

	// Sample entries to insert
	// Keep IDs simple for easier debugging if needed, though ULID is used in AddCommand.
	// For testing FindByCommand, we only care about the Command field primarily.
	// The other fields are just to satisfy AddCommand.
	baseTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	entriesToInsert := []struct{ cmd string }{
		{cmd: "git commit -m fix"},      // ID 1 (conceptual)
		{cmd: "go run main.go"},         // ID 2
		{cmd: "docker build . -t myapp"},// ID 3
		{cmd: "git push origin main"},   // ID 4
		{cmd: "echo 'hello world'"},     // ID 5
		{cmd: "cat file.txt | grep fix"},// ID 6
		{cmd: "another git operation"},  // ID 7
		{cmd: "just main things"},       // ID 8
		{cmd: "docker ps"},              // ID 9
		{cmd: "non relevant command"},   // ID 10
	}

	for i, e := range entriesToInsert {
		// AddCommand generates its own ULID, so we don't specify it here.
		// Using fixed timestamp for deterministic ordering if not overridden by relevance.
		_, err := manager.AddCommand(e.cmd, "/test/dir", "pts/0", "session1", "testhost", "testuser", baseTime.Add(time.Duration(i)*time.Second), true)
		if err != nil {
			t.Fatalf("failed to add command %q: %v", e.cmd, err)
		}
	}

	tests := []struct {
		name         string
		query        string
		currentDir   string // Assuming currentDir doesn't affect keyword logic for these tests
		expectedCmds []string
	}{
		{
			name:         "single term 'fix'",
			query:        "fix",
			expectedCmds: []string{"git commit -m fix", "cat file.txt | grep fix"},
		},
		{
			name:         "implicit AND 'git commit'",
			query:        "git commit",
			expectedCmds: []string{"git commit -m fix"},
		},
		{
			name:         "explicit OR 'git OR docker'",
			query:        "git OR docker",
			expectedCmds: []string{"git commit -m fix", "git push origin main", "another git operation", "docker build . -t myapp", "docker ps"},
		},
		{
			name:         "term with NOT 'main NOT go'",
			query:        "main NOT go",
			expectedCmds: []string{"git push origin main", "just main things"},
		},
		{
			name:         "explicit AND 'fix AND git'", // Though AND is implicit, testing "AND" keyword if supported (it's not, "AND" is a search term)
			// Current parser treats "AND" as a literal search term.
			// So, "fix AND git" means: command contains "fix" AND "AND" AND "git". No results.
			// If we want "AND" to be a keyword, ParseSearchQuery needs changes.
			// Testing current behavior.
			query:        "fix AND git",
			expectedCmds: []string{},
		},
		{
            name:         "implicit AND equivalent 'fix git'",
            query:        "fix git",
            expectedCmds: []string{"git commit -m fix"},
        },
		{
			name:         "explicit OR 'echo OR grep'",
			query:        "echo OR grep",
			expectedCmds: []string{"echo 'hello world'", "cat file.txt | grep fix"},
		},
		{
			name:         "no results 'nonexistentterm'",
			query:        "nonexistentterm",
			expectedCmds: []string{},
		},
		{
			name:         "empty query (should return all, but FindByCommand handles this by calling FindHistory)",
			query:        "",
			expectedCmds: []string{
				"git commit -m fix", "go run main.go", "docker build . -t myapp",
				"git push origin main", "echo 'hello world'", "cat file.txt | grep fix",
				"another git operation", "just main things", "docker ps", "non relevant command",
			},
		},
		{
			name:         "complex query 'git NOT commit OR docker build'",
			query:        "git NOT commit OR docker build",
			expectedCmds: []string{"git push origin main", "another git operation", "docker build . -t myapp"},
		},
		{
			name:         "complex query with only NOTs 'NOT commit NOT run'", // This means (NOT commit) OR (NOT run)
			query:        "NOT commit OR NOT run",
			// Expected: all commands that don't have "commit" OR don't have "run".
			// Easier to list what's excluded: "git commit -m fix" (has commit), "go run main.go" (has run)
			// Corrected Expectation:
			// (NOT commit) OR (NOT run) means any command that either doesn't have "commit"
			// OR doesn't have "run" (or has neither).
			// "git commit -m fix" matches "NOT run".
			// "go run main.go" matches "NOT commit".
			// All others match both. So, all commands should be returned.
			expectedCmds: []string{
				"git commit -m fix", "go run main.go", "docker build . -t myapp",
				"git push origin main", "echo 'hello world'", "cat file.txt | grep fix",
				"another git operation", "just main things", "docker ps", "non relevant command",
			},
		},
		{
			name: "query that parses to empty 'OR'",
			query: "OR", // ParseSearchQuery returns empty for this
			expectedCmds: []string{}, // FindByCommand returns empty for this case
		},
		{
			name: "query that parses to literal 'NOT'",
			query: "NOT", // ParseSearchQuery returns [[{Term: "NOT", IsNegated: false}]]
			expectedCmds: []string{}, // No commands contain "NOT" as a standalone word
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := manager.FindByCommand(tt.query, tt.currentDir)
			if err != nil {
				t.Fatalf("FindByCommand(%q, %q) failed: %v", tt.query, tt.currentDir, err)
			}

			gotCmds := normalizeEntriesForComparison(entries)
			expectedCmdsSorted := make([]string, len(tt.expectedCmds))
			copy(expectedCmdsSorted, tt.expectedCmds)
			sort.Strings(expectedCmdsSorted) // Ensure expected are sorted for comparison

			if !reflect.DeepEqual(gotCmds, expectedCmdsSorted) {
				t.Errorf("FindByCommand(%q, %q)\n  got: %v\n want: %v", tt.query, tt.currentDir, gotCmds, expectedCmdsSorted)
			}
		})
	}
}
