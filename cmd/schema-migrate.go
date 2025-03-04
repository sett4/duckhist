package main

import (
	"database/sql"
	"duckhist/internal/config"
	"fmt"
	"log"

	_ "github.com/marcboeker/go-duckdb"
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

		// Open database connection using DuckDB
		db, err := sql.Open("duckdb", cfg.DatabasePath)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

		// Create schema_migrations table if not exists
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`)
		if err != nil {
			log.Fatal(err)
		}

		// Get current schema version
		var currentVersion int
		err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
		if err != nil {
			log.Fatal(err)
		}

		// Apply migrations in order
		migrations := []struct {
			version int
			up      string
		}{
			{1, `CREATE TABLE IF NOT EXISTS history (
				id UUID PRIMARY KEY,
				command TEXT,
				executed_at TIMESTAMP,
				executing_host TEXT,
				executing_dir TEXT,
				executing_user TEXT
			)`},
			{2, `CREATE INDEX idx_history_id ON history (id DESC);
				CREATE INDEX idx_history_executed_at ON history (executed_at DESC);`},
			{3, `DROP INDEX IF EXISTS idx_history_executed_at;`},
		}

		// Apply each migration that hasn't been applied yet
		for _, migration := range migrations {
			if migration.version > currentVersion {
				// Execute migration within a transaction
				tx, err := db.Begin()
				if err != nil {
					log.Fatal(err)
				}

				// Apply migration
				if _, err := tx.Exec(migration.up); err != nil {
					tx.Rollback()
					log.Fatal(err)
				}

				// Record migration version
				if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", migration.version); err != nil {
					tx.Rollback()
					log.Fatal(err)
				}

				if err := tx.Commit(); err != nil {
					log.Fatal(err)
				}

				fmt.Printf("Applied migration version %d\n", migration.version)
			}
		}

		fmt.Println("Database schema is up to date")
	},
}

func init() {
	rootCmd.AddCommand(schemaMigrateCmd)
}
