package migrate

import (
	"database/sql"
	"duckhist/internal/embedded"
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

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/hashicorp/go-multierror"
	_ "github.com/marcboeker/go-duckdb"
)

// Register driver with golang-migrate
func init() {
	database.Register("duckdb", &DuckDB{})
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
	err = db.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'schema_migrations')").Scan(&tableExists)
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

// DuckDB is a migrate driver for DuckDB
type DuckDB struct {
	db       *sql.DB
	lock     sync.Mutex
	filePath string
}

// Open returns a new driver instance configured with parameters
func (d *DuckDB) Open(dsn string) (database.Driver, error) {
	purl, err := nurl.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing url: %w", err)
	}
	dbfile := strings.Replace(migrate.FilterCustomQuery(purl).String(), "duckdb://", "", 1)
	db, err := sql.Open("duckdb", dbfile)
	if err != nil {
		return nil, fmt.Errorf("opening '%s': %w", dbfile, err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging: %w", err)
	}
	d.db = db

	if err := d.ensureVersionTable(); err != nil {
		return nil, fmt.Errorf("ensuring version table: %w", err)
	}

	return d, nil
}

// Close closes the underlying database instance
func (d *DuckDB) Close() error {
	return d.db.Close()
}

// Lock acquires a database lock for migrations
func (d *DuckDB) Lock() error {
	d.lock.Lock()
	return nil
}

// Unlock releases the database lock for migrations
func (d *DuckDB) Unlock() error {
	d.lock.Unlock()
	return nil
}

// Run applies a migration to the database
func (d *DuckDB) Run(migration io.Reader) error {
	migr, err := io.ReadAll(migration)
	if err != nil {
		return err
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(string(migr)); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

var ii = 0

// SetVersion sets the current migration version
func (d *DuckDB) SetVersion(version int, dirty bool) error {
	_, err := d.db.Exec("INSERT INTO schema_migrations (version, dirty, applied_at) VALUES (?, ?, now()) ON CONFLICT DO UPDATE SET dirty = EXCLUDED.dirty, applied_at = now()", version, dirty)
	return err
}

// Version returns the current migration version
func (d *DuckDB) Version() (int, bool, error) {
	var version int
	var dirty bool
	err := d.db.QueryRow("SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version, &dirty)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return version, dirty, nil
}

// Drop drops the database
func (d *DuckDB) Drop() error {
	_, err := d.db.Exec("DROP TABLE IF EXISTS schema_migrations")
	return err
}

func (d *DuckDB) ensureVersionTable() (err error) {
	if err = d.Lock(); err != nil {
		return err
	}

	defer func() {
		if e := d.Unlock(); e != nil {
			if err == nil {
				err = e
			} else {
				err = multierror.Append(err, e)
			}
		}
	}()

	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS schema_migrations (version BIGINT PRIMARY KEY, dirty BOOLEAN, applied_at TIMESTAMP default current_timestamp);`)

	if _, err := d.db.Exec(query); err != nil {
		return fmt.Errorf("creating version table via '%s': %w", query, err)
	}

	return nil
}
