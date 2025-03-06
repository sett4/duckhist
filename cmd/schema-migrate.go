package main

import (
	"database/sql"
	"duckhist/internal/config"
	"duckhist/internal/embedded"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	_ "github.com/marcboeker/go-duckdb"
	"github.com/spf13/cobra"
)

// RunMigrations applies database migrations to the specified database
func RunMigrations(dbPath string) error {
	// Open database connection using DuckDB
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create schema_migrations table if not exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Get current schema version
	var currentVersion int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to get current schema version: %w", err)
	}

	// Get all migration files from embedded filesystem
	migrations, err := loadEmbeddedMigrations()
	if err != nil {
		// Fallback to file system migrations if embedded migrations fail
		migrations, err = loadMigrations("internal/migrations")
		if err != nil {
			return fmt.Errorf("failed to load migrations: %w", err)
		}
	}

	// Debug: Print loaded migrations
	fmt.Println("Loaded migrations:")
	for _, m := range migrations {
		fmt.Printf("Version %d: %d bytes of SQL\n", m.Version, len(m.UpSQL))
	}

	// Apply each migration that hasn't been applied yet
	for _, migration := range migrations {
		if migration.Version > currentVersion {
			// Execute migration within a transaction
			tx, err := db.Begin()
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}

			// Apply migration
			if _, err := tx.Exec(migration.UpSQL); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to apply migration version %d: %w", migration.Version, err)
			}

			// Record migration version
			if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", migration.Version); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to record migration version %d: %w", migration.Version, err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit transaction: %w", err)
			}

			fmt.Printf("Applied migration version %d\n", migration.Version)
		}
	}

	fmt.Println("Database schema is up to date")
	return nil
}

var schemaMigrateCmd = &cobra.Command{
	Use:   "schema-migrate",
	Short: "Update database schema to the latest version",
	Long:  `Update database schema to the latest version using migration files.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load config to get database path
		cfg, err := config.LoadConfig("")
		if err != nil {
			log.Fatal(err)
		}

		// Run migrations
		if err := RunMigrations(cfg.DatabasePath); err != nil {
			log.Fatal(err)
		}
	},
}

// Migration represents a database migration
type Migration struct {
	Version int
	UpSQL   string
	DownSQL string
}

// loadEmbeddedMigrations loads all migration files from the embedded filesystem
func loadEmbeddedMigrations() ([]Migration, error) {
	// Regular expression to extract version from filename
	versionRegex := regexp.MustCompile(`^0*(\d+)_.*\.(?:up|down)\.sql$`)

	// Map to store migrations by version
	migrationsMap := make(map[int]*Migration)

	// Get the embedded filesystem
	migrationsFS := embedded.GetMigrationsFS()

	// Walk through embedded migrations directory
	err := fs.WalkDir(migrationsFS, "migrations", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip non-SQL files
		if !strings.HasSuffix(d.Name(), ".sql") {
			return nil
		}

		// Extract version from filename
		matches := versionRegex.FindStringSubmatch(d.Name())
		if len(matches) < 2 {
			return nil
		}

		// Parse version
		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return err
		}

		// Read file content
		content, err := fs.ReadFile(migrationsFS, path)
		if err != nil {
			return err
		}

		// Create migration if it doesn't exist
		if _, exists := migrationsMap[version]; !exists {
			migrationsMap[version] = &Migration{Version: version}
		}

		// Set up or down SQL based on filename
		if strings.HasSuffix(d.Name(), ".up.sql") {
			migrationsMap[version].UpSQL = string(content)
		} else if strings.HasSuffix(d.Name(), ".down.sql") {
			migrationsMap[version].DownSQL = string(content)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert map to slice and sort by version
	migrations := make([]Migration, 0, len(migrationsMap))
	for _, m := range migrationsMap {
		migrations = append(migrations, *m)
	}

	// Sort migrations by version
	for i := 0; i < len(migrations)-1; i++ {
		for j := i + 1; j < len(migrations); j++ {
			if migrations[i].Version > migrations[j].Version {
				migrations[i], migrations[j] = migrations[j], migrations[i]
			}
		}
	}

	return migrations, nil
}

// loadMigrations loads all migration files from the specified directory
// This function is kept for backward compatibility and as a fallback
func loadMigrations(migrationsDir string) ([]Migration, error) {
	// Check if migrations directory exists
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("migrations directory does not exist: %s", migrationsDir)
	}

	// Regular expression to extract version from filename
	versionRegex := regexp.MustCompile(`^0*(\d+)_.*\.(?:up|down)\.sql$`)

	// Map to store migrations by version
	migrationsMap := make(map[int]*Migration)

	// Walk through migrations directory
	err := filepath.Walk(migrationsDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip non-SQL files
		if !strings.HasSuffix(info.Name(), ".sql") {
			return nil
		}

		// Extract version from filename
		matches := versionRegex.FindStringSubmatch(info.Name())
		if len(matches) < 2 {
			return nil
		}

		// Parse version
		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return err
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Create migration if it doesn't exist
		if _, exists := migrationsMap[version]; !exists {
			migrationsMap[version] = &Migration{Version: version}
		}

		// Set up or down SQL based on filename
		if strings.HasSuffix(info.Name(), ".up.sql") {
			migrationsMap[version].UpSQL = string(content)
		} else if strings.HasSuffix(info.Name(), ".down.sql") {
			migrationsMap[version].DownSQL = string(content)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert map to slice and sort by version
	migrations := make([]Migration, 0, len(migrationsMap))
	for _, m := range migrationsMap {
		migrations = append(migrations, *m)
	}

	// Sort migrations by version
	for i := 0; i < len(migrations)-1; i++ {
		for j := i + 1; j < len(migrations); j++ {
			if migrations[i].Version > migrations[j].Version {
				migrations[i], migrations[j] = migrations[j], migrations[i]
			}
		}
	}

	return migrations, nil
}

// ForceSchemaVersion forces the schema version to the specified value
func ForceSchemaVersion(dbPath string, version int) error {
	// Open database connection using DuckDB
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create schema_migrations table if not exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Delete all records from schema_migrations
	_, err = db.Exec("DELETE FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("failed to delete from schema_migrations: %w", err)
	}

	// Insert the specified version
	_, err = db.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, CURRENT_TIMESTAMP)", version)
	if err != nil {
		return fmt.Errorf("failed to insert version %d: %w", version, err)
	}

	fmt.Printf("Forced schema version to %d\n", version)
	return nil
}

var (
	forceVersionConfig string
	forceVersionTo     int
)

var forceVersionCmd = &cobra.Command{
	Use:   "force-version",
	Short: "Force schema version to a specific value",
	Long:  `Force schema version to a specific value in the database.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load config to get database path
		cfg, err := config.LoadConfig(forceVersionConfig)
		if err != nil {
			log.Fatal(err)
		}

		// Force version
		if err := ForceSchemaVersion(cfg.DatabasePath, forceVersionTo); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(schemaMigrateCmd)

	// Add flags to forceVersionCmd
	forceVersionCmd.Flags().StringVarP(&forceVersionConfig, "config", "c", "", "config file path")
	forceVersionCmd.Flags().IntVarP(&forceVersionTo, "update-to", "u", 2, "version to force")

	rootCmd.AddCommand(forceVersionCmd)
}
