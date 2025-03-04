package main

import (
	"database/sql"
	"duckhist/internal/config"
	"fmt"
	"log"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"
)

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

		// Get absolute path to migrations directory
		migrationsPath := filepath.Join("internal", "migrations")
		absPath, err := filepath.Abs(migrationsPath)
		if err != nil {
			log.Fatal(err)
		}

		// Open database connection
		db, err := sql.Open("sqlite3", cfg.DatabasePath)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

		// Initialize sqlite driver
		driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
		if err != nil {
			log.Fatal(err)
		}

		// Initialize migrate instance
		sourceURL := fmt.Sprintf("file://%s", absPath)
		m, err := migrate.NewWithDatabaseInstance(
			sourceURL,
			"sqlite3",
			driver,
		)
		if err != nil {
			log.Fatal(err)
		}

		// Run migrations
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatal(err)
		}

		fmt.Println("Database schema has been updated successfully")
	},
}

func init() {
	rootCmd.AddCommand(schemaMigrateCmd)
}
