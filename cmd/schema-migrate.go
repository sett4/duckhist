package cmd

import (
	"fmt"
	"log"

	"github.com/sett4/duckhist/internal/config"
	"github.com/sett4/duckhist/internal/embedded"

	_ "github.com/sett4/duckhist/internal/migrate"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/spf13/cobra"
)

// RunMigrations applies database migrations to the specified database
func RunMigrations(dbPath string) error {
	// Create source driver from embedded filesystem
	sourceDriver, err := iofs.New(embedded.GetMigrationsFS(), "migrations")
	if err != nil {
		return fmt.Errorf("failed to create source driver: %w", err)
	}

	// Create migration instance
	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, fmt.Sprintf("sqlite3://%s", dbPath))
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer func() {
		sourceErr, dbErr := m.Close()
		if sourceErr != nil {
			log.Printf("failed to close source: %v", sourceErr)
		}
		if dbErr != nil {
			log.Printf("failed to close database: %v", dbErr)
		}
	}()

	// Apply all up migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	// Get current version
	version, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("failed to get schema version: %w", err)
	}

	fmt.Printf("Database schema is up to date\n")
	fmt.Printf("Database path: %s\n", dbPath)
	fmt.Printf("Schema version: %d (dirty: %v)\n", version, dirty)
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

func init() {
	rootCmd.AddCommand(schemaMigrateCmd)
}
