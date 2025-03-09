package history

import (
	"database/sql"
	"duckhist/internal/migrate"
	"fmt"
	"os"
	"strings"
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

type HistoryQuery struct {
	manager    *Manager
	conditions []string
	args       []interface{}
	orderBy    string
	limit      *int
}

// Query creates a new HistoryQuery for building database queries
func (m *Manager) Query() *HistoryQuery {
	return &HistoryQuery{
		manager: m,
		orderBy: "id DESC",
	}
}

// InDirectory adds a condition to filter entries in the specified directory
func (q *HistoryQuery) InDirectory(dir string) *HistoryQuery {
	q.conditions = append(q.conditions, "executing_dir = ?")
	q.args = append(q.args, dir)
	return q
}

// NotInDirectory adds a condition to filter entries not in the specified directory
func (q *HistoryQuery) NotInDirectory(dir string) *HistoryQuery {
	q.conditions = append(q.conditions, "executing_dir != ?")
	q.args = append(q.args, dir)
	return q
}

// Search adds a condition to filter entries containing the search term
func (q *HistoryQuery) Search(term string) *HistoryQuery {
	q.conditions = append(q.conditions, "command LIKE ?")
	q.args = append(q.args, fmt.Sprintf("%%%s%%", term))
	return q
}

// Limit sets the maximum number of entries to return
func (q *HistoryQuery) Limit(n int) *HistoryQuery {
	q.limit = &n
	return q
}

// OrderByCurrentDirFirst sets the order to prioritize entries from the specified directory
func (q *HistoryQuery) OrderByCurrentDirFirst(dir string) *HistoryQuery {
	q.orderBy = "CASE WHEN executing_dir = ? THEN 0 ELSE 1 END, id DESC"
	q.args = append(q.args, dir)
	return q
}

// GetEntries executes the query and returns the matching entries
func (q *HistoryQuery) GetEntries() ([]Entry, error) {
	query := "SELECT id, command, executed_at, executing_host, executing_dir, executing_user, tty, sid FROM history"

	if len(q.conditions) > 0 {
		query += " WHERE " + strings.Join(q.conditions, " AND ")
	}

	query += " ORDER BY " + q.orderBy

	if q.limit != nil {
		query += " LIMIT ?"
		q.args = append(q.args, *q.limit)
	}

	rows, err := q.manager.db.Query(query, q.args...)
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
	entries, err := m.Query().GetEntries()
	if err != nil {
		return nil, err
	}

	commands := make([]string, len(entries))
	for i, entry := range entries {
		commands[i] = entry.Command
	}

	return commands, nil
}

// GetCurrentDirectoryHistory retrieves the last n commands executed in the current directory
func (m *Manager) GetCurrentDirectoryHistory(currentDir string, limit int) ([]Entry, error) {
	return m.Query().
		InDirectory(currentDir).
		Limit(limit).
		OrderByCurrentDirFirst(currentDir).
		GetEntries()
}

// GetAllHistory retrieves all commands with current directory entries first
func (m *Manager) GetAllHistory(currentDir string) ([]Entry, error) {
	return m.Query().
		OrderByCurrentDirFirst(currentDir).
		GetEntries()
}

// SearchCommands searches for commands matching the given query
// If query is empty, returns all commands
// Results are ordered with current directory entries first
func (m *Manager) SearchCommands(query string, currentDir string) ([]Entry, error) {
	if query == "" {
		return m.GetAllHistory(currentDir)
	}

	return m.Query().
		Search(query).
		OrderByCurrentDirFirst(currentDir).
		GetEntries()
}
