package history

import (
	"database/sql"
	"duckhist/internal/migrate"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/marcboeker/go-duckdb"
	"github.com/oklog/ulid/v2"
)

type Entry struct {
	ID        string
	Command   string
	Timestamp time.Time
	Hostname  string
	Directory string
	Username  string
	TTY       string
	SID       string
}

type Manager struct {
	db *sql.DB
}

// checkSchemaVersion checks if the database schema version matches the required version
// and prints a warning if they don't match
func checkSchemaVersion(db *sql.DB) {
	ok, current, required, err := migrate.CheckSchemaVersion(db)
	if err != nil {
		// Just log the error and continue, don't prevent operation
		fmt.Fprintf(os.Stderr, "Warning: Failed to check schema version: %v\n", err)
		return
	}

	if !ok {
		fmt.Fprintf(os.Stderr, "Warning: Database schema version mismatch. Current: %d, Required: %d\n", current, required)
		fmt.Fprintf(os.Stderr, "Please run 'duckhist schema-migrate' to update the schema\n")
	}
}

// NewManagerReadWrite creates a new Manager with read-write access to the database
func NewManagerReadWrite(dbPath string) (*Manager, error) {
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, err
	}

	// Check schema version
	checkSchemaVersion(db)

	return &Manager{db: db}, nil
}

// NewManagerReadOnly creates a new Manager with read-only access to the database
func NewManagerReadOnly(dbPath string) (*Manager, error) {
	db, err := sql.Open("duckdb", dbPath+"?access_mode=READ_ONLY")
	if err != nil {
		return nil, err
	}

	// Check schema version
	checkSchemaVersion(db)

	manager := &Manager{db: db}
	return manager, nil
}

func (m *Manager) Close() error {
	return m.db.Close()
}

// isDuplicate checks if the command already exists in the same context
func (m *Manager) isDuplicate(command string, directory string, hostname string, username string) (bool, error) {
	var count int
	err := m.db.QueryRow(`
		SELECT COUNT(*)
		FROM history
		WHERE command = ?
		AND executing_dir = ?
		AND executing_host = ?
		AND executing_user = ?`,
		command, directory, hostname, username).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to check for duplicate: %w", err)
	}

	return count > 0, nil
}

func (m *Manager) AddCommand(command string, directory string, tty string, sid string, hostname string, username string, noDedup bool) (bool, error) {
	executedAt := time.Now()

	if directory == "" {
		var err error
		directory, err = os.Getwd()
		if err != nil {
			return false, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	var isDup bool
	var err error

	if !noDedup {
		// Check for duplicates
		isDup, err = m.isDuplicate(command, directory, hostname, username)
		if err != nil {
			return false, err
		}

		if isDup {
			return true, nil
		}
	}

	id := uuid.Must(uuid.FromBytes(ulid.Make().Bytes()))

	_, err = m.db.Exec(`
        INSERT INTO history (
            id, command, executed_at, executing_host, 
            executing_dir, executing_user, tty, sid
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, command, executedAt, hostname, directory, username, tty, sid)
	return false, err
}

func (m *Manager) ListCommands() ([]string, error) {
	rows, err := m.db.Query(`SELECT command FROM history ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commands []string
	for rows.Next() {
		var command string
		if err := rows.Scan(&command); err != nil {
			return nil, err
		}
		commands = append(commands, command)
	}

	return commands, rows.Err()
}

// GetCurrentDirectoryHistory retrieves the last n commands executed in the current directory
func (m *Manager) GetCurrentDirectoryHistory(currentDir string, limit int) ([]Entry, error) {
	query := `
		SELECT id, command, executed_at, executing_host, executing_dir, executing_user, tty, sid
		FROM history
		WHERE executing_dir = ?
		ORDER BY id DESC
		LIMIT ?
	`

	rows, err := m.db.Query(query, currentDir, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.Command, &entry.Timestamp, &entry.Hostname, &entry.Directory, &entry.Username, &entry.TTY, &entry.SID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// GetFullHistory retrieves all commands excluding those from the current directory
func (m *Manager) GetFullHistory(currentDir string) ([]Entry, error) {
	query := `
		SELECT id, command, executed_at, executing_host, executing_dir, executing_user, tty, sid
		FROM history
		WHERE executing_dir != ?
		ORDER BY id DESC
	`

	rows, err := m.db.Query(query, currentDir)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.Command, &entry.Timestamp, &entry.Hostname, &entry.Directory, &entry.Username, &entry.TTY, &entry.SID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// GetAllHistory retrieves all commands with current directory entries first
func (m *Manager) GetAllHistory(currentDir string) ([]Entry, error) {
	query := `
		SELECT id, command, executed_at, executing_host, executing_dir, executing_user, tty, sid
		FROM history
		ORDER BY 
			CASE WHEN executing_dir = ? THEN 0 ELSE 1 END,
			id DESC
	`

	rows, err := m.db.Query(query, currentDir)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.Command, &entry.Timestamp, &entry.Hostname, &entry.Directory, &entry.Username, &entry.TTY, &entry.SID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// SearchCommands searches for commands matching the given query
// If query is empty, returns all commands
// Results are ordered with current directory entries first
func (m *Manager) SearchCommands(query string, currentDir string) ([]Entry, error) {
	if query == "" {
		return m.GetAllHistory(currentDir)
	}

	searchQuery := fmt.Sprintf("%%%s%%", query)
	sqlQuery := `
		SELECT id, command, executed_at, executing_host, executing_dir, executing_user, tty, sid
		FROM history
		WHERE command LIKE ?
		ORDER BY 
			CASE WHEN executing_dir = ? THEN 0 ELSE 1 END,
			id DESC
	`

	rows, err := m.db.Query(sqlQuery, searchQuery, currentDir)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.Command, &entry.Timestamp, &entry.Hostname, &entry.Directory, &entry.Username, &entry.TTY, &entry.SID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}
