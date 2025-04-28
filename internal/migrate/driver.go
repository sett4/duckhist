package migrate

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	nurl "net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/sett4/duckhist/internal/embedded"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/hashicorp/go-multierror"
	_ "github.com/mattn/go-sqlite3"
)

// Register driver with golang-migrate
func init() {
	database.Register("sqlite3", &SQLite{})
}

// GetLatestMigrationVersion returns the latest migration version from embedded migrations
func GetLatestMigrationVersion() (int, error) {
	migrationsFS := embedded.GetMigrationsFS()

	var latestVersion int
	migrationRegex := regexp.MustCompile(`^(\d+)_.+\.up\.sql$`)

	err := fs.WalkDir(migrationsFS, "migrations", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && path != "migrations" {
			return fs.SkipDir
		}

		if !d.IsDir() {
			filename := filepath.Base(path)
			matches := migrationRegex.FindStringSubmatch(filename)
			if len(matches) > 1 {
				version, err := strconv.Atoi(matches[1])
				if err != nil {
					return nil // Skip files with invalid version numbers
				}

				if version > latestVersion {
					latestVersion = version
				}
			}
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to walk migrations directory: %w", err)
	}

	return latestVersion, nil
}

// CheckSchemaVersion checks if the current database schema version matches the required version
func CheckSchemaVersion(db *sql.DB) (bool, int, int, error) {
	// Get the latest migration version
	requiredVersion, err := GetLatestMigrationVersion()
	if err != nil {
		return false, 0, 0, fmt.Errorf("failed to get latest migration version: %w", err)
	}

	// Check if schema_migrations table exists
	var tableExists bool
	err = db.QueryRow("SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type='table' AND name='schema_migrations')").Scan(&tableExists)
	if err != nil {
		return false, 0, requiredVersion, fmt.Errorf("failed to check if schema_migrations table exists: %w", err)
	}

	if !tableExists {
		// Table doesn't exist, schema needs migration
		return false, 0, requiredVersion, nil
	}

	// Get current version from database
	var currentVersion int
	var dirty bool
	err = db.QueryRow("SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&currentVersion, &dirty)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No migrations applied yet
			return false, 0, requiredVersion, nil
		}
		return false, 0, requiredVersion, fmt.Errorf("failed to get current schema version: %w", err)
	}

	// Check if versions match
	return currentVersion == requiredVersion, currentVersion, requiredVersion, nil
}

// SQLite is a migrate driver for SQLite
type SQLite struct {
	db   *sql.DB
	lock sync.Mutex
}

// Open returns a new driver instance configured with parameters
func (s *SQLite) Open(dsn string) (database.Driver, error) {
	purl, err := nurl.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing url: %w", err)
	}
	dbfile := strings.Replace(migrate.FilterCustomQuery(purl).String(), "sqlite3://", "", 1)
	db, err := sql.Open("sqlite3", dbfile)
	if err != nil {
		return nil, fmt.Errorf("opening '%s': %w", dbfile, err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging: %w", err)
	}
	s.db = db

	if err := s.ensureVersionTable(); err != nil {
		return nil, fmt.Errorf("ensuring version table: %w", err)
	}

	return s, nil
}

// Close closes the underlying database instance
func (s *SQLite) Close() error {
	return s.db.Close()
}

// Lock acquires a database lock for migrations
func (s *SQLite) Lock() error {
	s.lock.Lock()
	return nil
}

// Unlock releases the database lock for migrations
func (s *SQLite) Unlock() error {
	s.lock.Unlock()
	return nil
}

// Run applies a migration to the database
func (s *SQLite) Run(migration io.Reader) error {
	migr, err := io.ReadAll(migration)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(string(migr)); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("migration failed: %v, rollback failed: %v", err, rbErr)
		}
		return fmt.Errorf("migration failed: %v", err)
	}

	return tx.Commit()
}

// SetVersion sets the current migration version
func (s *SQLite) SetVersion(version int, dirty bool) error {
	_, err := s.db.Exec("INSERT INTO schema_migrations (version, dirty, applied_at) VALUES (?, ?, CURRENT_TIMESTAMP) ON CONFLICT(version) DO UPDATE SET dirty = EXCLUDED.dirty, applied_at = CURRENT_TIMESTAMP", version, dirty)
	return err
}

// Version returns the current migration version
func (s *SQLite) Version() (int, bool, error) {
	var version int
	var dirty bool
	err := s.db.QueryRow("SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version, &dirty)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return version, dirty, nil
}

// Drop drops the database
func (s *SQLite) Drop() error {
	_, err := s.db.Exec("DROP TABLE IF EXISTS schema_migrations")
	return err
}

func (s *SQLite) ensureVersionTable() (err error) {
	if err = s.Lock(); err != nil {
		return err
	}

	defer func() {
		if e := s.Unlock(); e != nil {
			if err == nil {
				err = e
			} else {
				err = multierror.Append(err, e)
			}
		}
	}()

	const createSchemaMigrationsTableQuery = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    dirty BOOLEAN,
    applied_at TIMESTAMP default CURRENT_TIMESTAMP
);`

	if _, err := s.db.Exec(createSchemaMigrationsTableQuery); err != nil {
		return fmt.Errorf("creating version table via '%s': %w", createSchemaMigrationsTableQuery, err)
	}

	return nil
}
