package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"duckhist/internal/history"

	_ "github.com/marcboeker/go-duckdb"
)

func TestCommandAdder_AddCommand(t *testing.T) {
	t.Run("add command with default directory", func(t *testing.T) {
		// Create temporary directory for test
		tmpDir := t.TempDir()

		// Create config file
		configPath := filepath.Join(tmpDir, "config.toml")
		dbPath := filepath.Join(tmpDir, "test.duckdb")
		content := fmt.Sprintf("database_path = %q", dbPath)
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config file: %v", err)
		}

		// Initialize database with schema
		db, err := sql.Open("duckdb", dbPath)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}

		// Create history table
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS history (
			id UUID PRIMARY KEY,
			command TEXT,
			executed_at TIMESTAMP,
			executing_host TEXT,
			executing_dir TEXT,
			executing_user TEXT,
			tty TEXT,
			sid TEXT
		)`)
		if err != nil {
			t.Fatalf("failed to create history table: %v", err)
		}
		db.Close()

		// Create CommandAdder
		adder := NewCommandAdder(configPath, false)

		// Add command
		command := "ls -la"
		if err := adder.AddCommand(command, "", "", ""); err != nil {
			t.Fatalf("AddCommand failed: %v", err)
		}

		// Verify command was added
		manager, err := history.NewManagerReadWrite(dbPath)
		if err != nil {
			t.Fatalf("failed to create history manager: %v", err)
		}
		defer manager.Close()

		// Get current directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get current directory: %v", err)
		}

		// Check if command exists in history
		entries, err := manager.GetCurrentDirectoryHistory(currentDir, 1)
		if err != nil {
			t.Fatalf("failed to get commands: %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("expected 1 command, got %d", len(entries))
		}
		if entries[0].Command != command {
			t.Errorf("expected command %q, got %q", command, entries[0].Command)
		}
		if entries[0].Directory != currentDir {
			t.Errorf("expected directory %q, got %q", currentDir, entries[0].Directory)
		}
	})

	t.Run("add command with specified directory", func(t *testing.T) {
		// Create temporary directory for test
		tmpDir := t.TempDir()

		// Create config file
		configPath := filepath.Join(tmpDir, "config.toml")
		dbPath := filepath.Join(tmpDir, "test.duckdb")
		content := fmt.Sprintf("database_path = %q", dbPath)
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config file: %v", err)
		}

		// Initialize database with schema
		db, err := sql.Open("duckdb", dbPath)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}

		// Create history table
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS history (
			id UUID PRIMARY KEY,
			command TEXT,
			executed_at TIMESTAMP,
			executing_host TEXT,
			executing_dir TEXT,
			executing_user TEXT,
			tty TEXT,
			sid TEXT
		)`)
		if err != nil {
			t.Fatalf("failed to create history table: %v", err)
		}
		db.Close()

		// Create CommandAdder
		adder := NewCommandAdder(configPath, false)

		// Add command with specified directory
		command := "ls -la"
		specifiedDir := "/specified/directory"
		if err := adder.AddCommand(command, specifiedDir, "", ""); err != nil {
			t.Fatalf("AddCommand failed: %v", err)
		}

		// Verify command was added
		manager, err := history.NewManagerReadWrite(dbPath)
		if err != nil {
			t.Fatalf("failed to create history manager: %v", err)
		}
		defer manager.Close()

		// Check if command exists in history
		entries, err := manager.GetCurrentDirectoryHistory(specifiedDir, 1)
		if err != nil {
			t.Fatalf("failed to get commands: %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("expected 1 command, got %d", len(entries))
		}
		if entries[0].Command != command {
			t.Errorf("expected command %q, got %q", command, entries[0].Command)
		}
		if entries[0].Directory != specifiedDir {
			t.Errorf("expected directory %q, got %q", specifiedDir, entries[0].Directory)
		}
	})

	t.Run("empty command", func(t *testing.T) {
		// Create temporary directory for test
		tmpDir := t.TempDir()

		// Create config file
		configPath := filepath.Join(tmpDir, "config.toml")
		dbPath := filepath.Join(tmpDir, "test.duckdb")
		content := fmt.Sprintf("database_path = %q", dbPath)
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config file: %v", err)
		}

		// Create CommandAdder
		adder := NewCommandAdder(configPath, false)

		// Try to add empty command
		err := adder.AddCommand("", "", "", "")
		if err == nil {
			t.Error("expected error for empty command, got nil")
		}
		if err.Error() != "empty command" {
			t.Errorf("expected error message %q, got %q", "empty command", err.Error())
		}
	})

	t.Run("verbose output", func(t *testing.T) {
		// Create temporary directory for test
		tmpDir := t.TempDir()

		// Create config file
		configPath := filepath.Join(tmpDir, "config.toml")
		dbPath := filepath.Join(tmpDir, "test.duckdb")
		content := fmt.Sprintf("database_path = %q", dbPath)
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config file: %v", err)
		}

		// Initialize database with schema
		db, err := sql.Open("duckdb", dbPath)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}

		// Create history table
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS history (
			id UUID PRIMARY KEY,
			command TEXT,
			executed_at TIMESTAMP,
			executing_host TEXT,
			executing_dir TEXT,
			executing_user TEXT,
			tty TEXT,
			sid TEXT
		)`)
		if err != nil {
			t.Fatalf("failed to create history table: %v", err)
		}
		db.Close()

		// Create CommandAdder with verbose mode
		adder := NewCommandAdder(configPath, true)

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Add command
		command := "ls -la"
		if err := adder.AddCommand(command, "", "", ""); err != nil {
			t.Fatalf("AddCommand failed: %v", err)
		}

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Read captured output
		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		expectedOutput := fmt.Sprintf("Command added to history: %s\n", command)
		if output != expectedOutput {
			t.Errorf("expected output %q, got %q", expectedOutput, output)
		}
	})

	t.Run("invalid config path", func(t *testing.T) {
		adder := NewCommandAdder("nonexistent/config.toml", false)
		err := adder.AddCommand("ls", "", "", "")
		if err == nil {
			t.Error("expected error for invalid config path, got nil")
		}
	})

	t.Run("with TTY and SID", func(t *testing.T) {
		// Create temporary directory for test
		tmpDir := t.TempDir()

		// Create config file
		configPath := filepath.Join(tmpDir, "config.toml")
		dbPath := filepath.Join(tmpDir, "test.duckdb")
		content := fmt.Sprintf("database_path = %q", dbPath)
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config file: %v", err)
		}

		// Initialize database with schema
		db, err := sql.Open("duckdb", dbPath)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}

		// Create history table
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS history (
			id UUID PRIMARY KEY,
			command TEXT,
			executed_at TIMESTAMP,
			executing_host TEXT,
			executing_dir TEXT,
			executing_user TEXT,
			tty TEXT,
			sid TEXT
		)`)
		if err != nil {
			t.Fatalf("failed to create history table: %v", err)
		}
		db.Close()

		// Create CommandAdder with TTY and SID
		tty := "/dev/pts/1"
		sid := "12345"
		adder := NewCommandAdder(configPath, false)

		// Add command
		command := "ls -la"
		if err := adder.AddCommand(command, "", tty, sid); err != nil {
			t.Fatalf("AddCommand failed: %v", err)
		}

		// Verify command was added
		manager, err := history.NewManagerReadWrite(dbPath)
		if err != nil {
			t.Fatalf("failed to create history manager: %v", err)
		}
		defer manager.Close()

		// Get current directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get current directory: %v", err)
		}

		// Check if command exists in history
		entries, err := manager.GetCurrentDirectoryHistory(currentDir, 1)
		if err != nil {
			t.Fatalf("failed to get commands: %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("expected 1 command, got %d", len(entries))
		}
		if entries[0].TTY != tty {
			t.Errorf("expected TTY %q, got %q", tty, entries[0].TTY)
		}
		if entries[0].SID != sid {
			t.Errorf("expected SID %q, got %q", sid, entries[0].SID)
		}
	})
}
