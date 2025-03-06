package history

import (
	"database/sql"
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

// NewManagerReadWrite creates a new Manager with read-write access to the database
func NewManagerReadWrite(dbPath string) (*Manager, error) {
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, err
	}

	return &Manager{db: db}, nil
}

// NewManagerReadOnly creates a new Manager with read-only access to the database
func NewManagerReadOnly(dbPath string) (*Manager, error) {
	db, err := sql.Open("duckdb", dbPath+"?access_mode=READ_ONLY")
	if err != nil {
		return nil, err
	}

	manager := &Manager{db: db}
	return manager, nil
}

func (m *Manager) Close() error {
	return m.db.Close()
}

func (m *Manager) AddCommand(command string, directory string, tty string, sid string, hostname string, username string) error {
	executedAt := time.Now()

	if directory == "" {
		var err error
		directory, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	id := uuid.Must(uuid.FromBytes(ulid.Make().Bytes()))

	_, err := m.db.Exec(`
        INSERT INTO history (
            id, command, executed_at, executing_host, 
            executing_dir, executing_user, tty, sid
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, command, executedAt, hostname, directory, username, tty, sid)
	return err
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
