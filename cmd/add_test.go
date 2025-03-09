package main

import (
	"bytes"
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

		// Run migrations to initialize database schema
		if err := RunMigrations(dbPath); err != nil {
			t.Fatalf("failed to run migrations: %v", err)
		}

		// Create CommandAdder
		adder := NewCommandAdder(configPath, false)

		// Get current directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get current directory: %v", err)
		}

		// Add command
		command := "ls -la"
		hostname, _ := os.Hostname()
		username := os.Getenv("USER")
		if err := adder.AddCommand(command, currentDir, "", "", hostname, username); err != nil {
			t.Fatalf("AddCommand failed: %v", err)
		}

		// Verify command was added
		manager, err := history.NewManagerReadWrite(dbPath)
		if err != nil {
			t.Fatalf("failed to create history manager: %v", err)
		}
		defer manager.Close()

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

		// Run migrations to initialize database schema
		if err := RunMigrations(dbPath); err != nil {
			t.Fatalf("failed to run migrations: %v", err)
		}

		// Create CommandAdder
		adder := NewCommandAdder(configPath, false)

		// Add command with specified directory
		command := "ls -la"
		specifiedDir := "/specified/directory"
		hostname, _ := os.Hostname()
		username := os.Getenv("USER")
		if err := adder.AddCommand(command, specifiedDir, "", "", hostname, username); err != nil {
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
		hostname, _ := os.Hostname()
		username := os.Getenv("USER")
		err := adder.AddCommand("", "", "", "", hostname, username)
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

		// Run migrations to initialize database schema
		if err := RunMigrations(dbPath); err != nil {
			t.Fatalf("failed to run migrations: %v", err)
		}

		// Create CommandAdder with verbose mode
		adder := NewCommandAdder(configPath, true)

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Add command
		command := "ls -la"
		hostname, _ := os.Hostname()
		username := os.Getenv("USER")
		if err := adder.AddCommand(command, "", "", "", hostname, username); err != nil {
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
		hostname, _ := os.Hostname()
		username := os.Getenv("USER")
		err := adder.AddCommand("ls", "", "", "", hostname, username)
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

		// Run migrations to initialize database schema
		if err := RunMigrations(dbPath); err != nil {
			t.Fatalf("failed to run migrations: %v", err)
		}

		// Create CommandAdder with TTY and SID
		tty := "/dev/pts/1"
		sid := "12345"
		adder := NewCommandAdder(configPath, false)

		// Get current directory
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get current directory: %v", err)
		}

		// Add command
		command := "ls -la"
		hostname, _ := os.Hostname()
		username := os.Getenv("USER")
		if err := adder.AddCommand(command, currentDir, tty, sid, hostname, username); err != nil {
			t.Fatalf("AddCommand failed: %v", err)
		}

		// Verify command was added
		manager, err := history.NewManagerReadWrite(dbPath)
		if err != nil {
			t.Fatalf("failed to create history manager: %v", err)
		}
		defer manager.Close()

		// Check if command exists in history
		entries, err := manager.GetCurrentDirectoryHistory(currentDir, 1)
		if err != nil {
			t.Fatalf("failed to get commands: %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("expected 1 command, got %d", len(entries))
		}
		if entries[0].SID != sid {
			t.Errorf("expected SID %q, got %q", sid, entries[0].SID)
		}
	})
}

func TestAddCmd_TTY(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create config file
	configPath := filepath.Join(tmpDir, "config.toml")
	dbPath := filepath.Join(tmpDir, "test.duckdb")
	content := fmt.Sprintf("database_path = %q", dbPath)
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	// Run migrations to initialize database schema
	if err := RunMigrations(dbPath); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Save original environment and config
	originalTTY := os.Getenv("TTY")
	originalCfgFile := cfgFile
	defer func() {
		os.Setenv("TTY", originalTTY)
		cfgFile = originalCfgFile
	}()

	rootCmd.ResetCommands()
	rootCmd.AddCommand(addCmd)

	t.Run("TTY from env", func(t *testing.T) {
		// Set environment variable
		envTTY := "/dev/pts/test1"
		os.Setenv("TTY", envTTY)

		// Reset global variables
		tty = ""
		cfgFile = configPath

		rootCmd.SetArgs([]string{"add", "--config", cfgFile, "--", "hogehoge1"})
		if err := rootCmd.Execute(); err != nil {
			t.Errorf("failed to execute add command: %v", err)
		}

		// Create history manager
		manager, err := history.NewManagerReadOnly(dbPath)
		if err != nil {
			t.Fatalf("failed to create history manager: %v", err)
		}
		defer manager.Close()

		list, err := manager.GetFullHistory("")
		if len(list) != 1 {
			t.Errorf("failed to execute add command: %v", list)
		}
		if list[0].TTY != envTTY {
			t.Errorf("unexpected tty: %s", list[0].TTY)
		}
	})

	// t.Run("TTY from arg", func(t *testing.T) {
	// 	// Set environment variable
	// 	envTTY := "/dev/pts/test1"
	// 	os.Setenv("TTY", envTTY)
	// 	argTTY := "/dev/pts/testA"

	// 	// Reset global variables
	// 	tty = ""
	// 	cfgFile = configPath

	// 	rootCmd.SetArgs([]string{"add", "--config", cfgFile, "--tty", argTTY, "--", "hogehoge2"})
	// 	if err := rootCmd.Execute(); err != nil {
	// 		t.Errorf("failed to execute add command: %v", err)
	// 	}

	// 	// Create history manager
	// 	manager, err := history.NewManagerReadOnly(dbPath)
	// 	if err != nil {
	// 		t.Fatalf("failed to create history manager: %v", err)
	// 	}
	// 	defer manager.Close()

	// 	list, err := manager.GetFullHistory("")
	// 	// fmt.Println(list)
	// 	if len(list) != 1 {
	// 		t.Errorf("failed to execute add command: %v", list)
	// 	}
	// 	if list[0].TTY != argTTY {
	// 		t.Errorf("unexpected tty: %s", list[0].TTY)
	// 	}

	// })

}
