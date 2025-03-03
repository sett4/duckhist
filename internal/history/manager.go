package history

import (
	"database/sql"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/marcboeker/go-duckdb"
	"github.com/oklog/ulid/v2"
)

type Manager struct {
	db *sql.DB
}

func NewManager() (*Manager, error) {
	db, err := sql.Open("duckdb", os.Getenv("HOME")+"/.duckhist.duckdb")
	if err != nil {
		return nil, err
	}

	manager := &Manager{db: db}
	if err := manager.initTable(); err != nil {
		db.Close()
		return nil, err
	}

	return manager, nil
}

func (m *Manager) Close() error {
	return m.db.Close()
}

func (m *Manager) initTable() error {
	_, err := m.db.Exec(`CREATE TABLE IF NOT EXISTS history (
		id UUID,
		command TEXT,
		executed_at TIMESTAMP,
		executing_host TEXT,
		executing_dir TEXT,
		executing_user TEXT
	)`)
	return err
}

func (m *Manager) AddCommand(command string) error {
	executedAt := time.Now()
	executingHost, _ := os.Hostname()
	executingDir, _ := os.Getwd()
	executingUser := os.Getenv("USER")

	id := uuid.Must(uuid.FromBytes(ulid.Make().Bytes()))

	_, err := m.db.Exec(`INSERT INTO history (id, command, executed_at, executing_host, executing_dir, executing_user) 
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, command, executedAt, executingHost, executingDir, executingUser)
	return err
}

func (m *Manager) ListCommands() ([]string, error) {
	rows, err := m.db.Query(`SELECT command FROM history ORDER BY executed_at DESC`)
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
