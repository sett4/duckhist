package migrate

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	nurl "net/url"
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
	fmt.Println("version", version, "dirty", dirty)
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
