package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNewInitConfig(t *testing.T) {
	t.Run("with empty config path", func(t *testing.T) {
		ic, err := NewInitConfig("")
		if err != nil {
			t.Fatalf("NewInitConfig failed: %v", err)
		}
		if ic.configPath != "" {
			t.Errorf("expected empty config path, got %s", ic.configPath)
		}
		if ic.home == "" {
			t.Error("home directory should not be empty")
		}
	})

	t.Run("with custom config path", func(t *testing.T) {
		customPath := "custom/path/config.toml"
		ic, err := NewInitConfig(customPath)
		if err != nil {
			t.Fatalf("NewInitConfig failed: %v", err)
		}
		if ic.configPath != customPath {
			t.Errorf("expected config path %s, got %s", customPath, ic.configPath)
		}
	})
}

func TestInitConfig_InitializeDatabase(t *testing.T) {
	t.Run("initialize with default config", func(t *testing.T) {
		// Create temporary directory for test
		tmpDir := t.TempDir()

		// Create InitConfig with temporary directory
		ic := &InitConfig{
			home:       tmpDir,
			configPath: "",
		}

		// Create config directory
		if err := ic.EnsureConfigDir(); err != nil {
			t.Fatalf("failed to create config directory: %v", err)
		}

		// Create config file with absolute path
		dbPath := filepath.Join(tmpDir, ".duckhist.duckdb")
		content := fmt.Sprintf("database_path = %q", dbPath)
		configPath := ic.GetConfigPath()
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create config file: %v", err)
		}

		// Initialize database
		if err := ic.InitializeDatabase(); err != nil {
			t.Fatalf("InitializeDatabase failed: %v", err)
		}

		// Check if database file exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Errorf("database file was not created at %s", dbPath)
		}
	})

	t.Run("initialize with custom config", func(t *testing.T) {
		// Create temporary directory for test
		tmpDir := t.TempDir()
		customConfigPath := filepath.Join(tmpDir, "custom.toml")

		// Create custom config file with absolute path
		dbPath := filepath.Join(tmpDir, "custom.duckdb")
		customConfig := fmt.Sprintf("database_path = %q", dbPath)
		if err := os.WriteFile(customConfigPath, []byte(customConfig), 0644); err != nil {
			t.Fatalf("failed to create custom config: %v", err)
		}

		// Create InitConfig with custom path
		ic := &InitConfig{
			home:       tmpDir,
			configPath: customConfigPath,
		}

		// Initialize database
		if err := ic.InitializeDatabase(); err != nil {
			t.Fatalf("InitializeDatabase failed: %v", err)
		}

		// Check if database file exists
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			t.Errorf("database file was not created at %s", dbPath)
		}
	})

	t.Run("error on invalid config", func(t *testing.T) {
		// Create temporary directory for test
		tmpDir := t.TempDir()
		invalidConfigPath := filepath.Join(tmpDir, "invalid.toml")

		// Create invalid config file
		invalidConfig := `invalid toml content`
		if err := os.WriteFile(invalidConfigPath, []byte(invalidConfig), 0644); err != nil {
			t.Fatalf("failed to create invalid config: %v", err)
		}

		// Create InitConfig with invalid config
		ic := &InitConfig{
			home:       tmpDir,
			configPath: invalidConfigPath,
		}

		// Try to initialize database
		err := ic.InitializeDatabase()
		if err == nil {
			t.Error("expected error for invalid config, got nil")
		}
	})

	t.Run("error on non-existent config", func(t *testing.T) {
		// Create temporary directory for test
		tmpDir := t.TempDir()
		nonExistentPath := filepath.Join(tmpDir, "nonexistent.toml")

		// Create InitConfig with non-existent config
		ic := &InitConfig{
			home:       tmpDir,
			configPath: nonExistentPath,
		}

		// Try to initialize database
		err := ic.InitializeDatabase()
		if err == nil {
			t.Error("expected error for non-existent config, got nil")
		}
	})
}

func TestInitConfig_GetConfigPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}

	t.Run("default config path", func(t *testing.T) {
		ic := &InitConfig{home: home}
		expected := filepath.Join(home, ".config", "duckhist", "duckhist.toml")
		if got := ic.GetConfigPath(); got != expected {
			t.Errorf("expected %s, got %s", expected, got)
		}
	})

	t.Run("custom config path", func(t *testing.T) {
		customPath := "custom/path/config.toml"
		ic := &InitConfig{configPath: customPath, home: home}
		if got := ic.GetConfigPath(); got != customPath {
			t.Errorf("expected %s, got %s", customPath, got)
		}
	})
}

func TestInitConfig_CreateDefaultConfig(t *testing.T) {
	t.Run("create default config", func(t *testing.T) {
		// Create temporary directory for test
		tmpDir := t.TempDir()

		// Create InitConfig with temporary directory
		ic := &InitConfig{
			home:       tmpDir,
			configPath: "",
		}

		// Ensure config directory exists
		if err := ic.EnsureConfigDir(); err != nil {
			t.Fatalf("failed to create config directory: %v", err)
		}

		// Create default config
		if err := ic.CreateDefaultConfig(); err != nil {
			t.Fatalf("CreateDefaultConfig failed: %v", err)
		}

		// Check if config file exists
		configPath := ic.GetConfigPath()
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Errorf("config file was not created at %s", configPath)
		}

		// Check config file content
		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config file: %v", err)
		}

		expectedContent := `# Path to DuckDB database file
database_path = "~/.duckhist.duckdb"
`
		if string(content) != expectedContent {
			t.Errorf("unexpected config content.\nexpected:\n%s\ngot:\n%s", expectedContent, string(content))
		}
	})

	t.Run("config file already exists", func(t *testing.T) {
		// Create temporary directory
		tmpDir := t.TempDir()

		ic := &InitConfig{
			home:       tmpDir,
			configPath: "",
		}

		// Create config directory
		if err := ic.EnsureConfigDir(); err != nil {
			t.Fatalf("failed to create config directory: %v", err)
		}

		// Create config file with custom content
		customContent := "custom_content"
		configPath := ic.GetConfigPath()
		if err := os.WriteFile(configPath, []byte(customContent), 0644); err != nil {
			t.Fatalf("failed to write custom config: %v", err)
		}

		// Try to create default config
		if err := ic.CreateDefaultConfig(); err != nil {
			t.Fatalf("CreateDefaultConfig failed: %v", err)
		}

		// Verify content was not overwritten
		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config file: %v", err)
		}

		if string(content) != customContent {
			t.Errorf("existing config was overwritten.\nexpected:\n%s\ngot:\n%s", customContent, string(content))
		}
	})
}

func TestInitConfig_EnsureConfigDir(t *testing.T) {
	t.Run("create new directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		ic := &InitConfig{
			home:       tmpDir,
			configPath: "",
		}

		if err := ic.EnsureConfigDir(); err != nil {
			t.Fatalf("EnsureConfigDir failed: %v", err)
		}

		configDir := filepath.Join(tmpDir, ".config", "duckhist")
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			t.Errorf("config directory was not created at %s", configDir)
		}
	})

	t.Run("directory already exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, ".config", "duckhist")

		// Create directory before test
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}

		ic := &InitConfig{
			home:       tmpDir,
			configPath: "",
		}

		if err := ic.EnsureConfigDir(); err != nil {
			t.Fatalf("EnsureConfigDir failed: %v", err)
		}

		// Verify directory still exists
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			t.Errorf("config directory does not exist at %s", configDir)
		}
	})
}
