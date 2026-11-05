// Package logstore keeps the Agent Log and the Execution Log in SQLite
// (yaam.db). Neither log ever holds result rows.
package logstore

import (
	"database/sql"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

// AgentEntry is one Agent Log entry: a Request or a schema lookup.
type AgentEntry struct {
	ID         int64    `json:"id"`
	Time       string   `json:"time"`
	AgentName  string   `json:"agentName"`
	Project    string   `json:"project"`
	Connection string   `json:"connection"`
	Kind       string   `json:"kind"` // request | schema
	Purpose    string   `json:"purpose"`
	SQL        string   `json:"sql"`
	Status     string   `json:"status"` // run | waiting | rejected | cancelled | failed
	DurationMs *float64 `json:"durationMs"`
	Rows       *int64   `json:"rows"`
	Error      string   `json:"error"`
}

// ExecEntry is one Execution Log entry: SQL that actually ran.
type ExecEntry struct {
	ID         int64   `json:"id"`
	Time       string  `json:"time"`
	By         string  `json:"by"` // user | agent
	AgentName  string  `json:"agentName"`
	Project    string  `json:"project"`
	Connection string  `json:"connection"`
	Source     string  `json:"source"` // editor | grid | agent
	SQL        string  `json:"sql"`
	DurationMs float64 `json:"durationMs"`
	Rows       *int64  `json:"rows"`
	Error      string  `json:"error"`
}

const schema = `
CREATE TABLE IF NOT EXISTS agent_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	time TEXT NOT NULL,
	agent_name TEXT NOT NULL,
	project TEXT NOT NULL,
	connection TEXT NOT NULL,
	kind TEXT NOT NULL,
	purpose TEXT NOT NULL,
	sql TEXT NOT NULL,
	status TEXT NOT NULL,
	duration_ms REAL,
	rows INTEGER,
	error TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS execution_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	time TEXT NOT NULL,
	by TEXT NOT NULL,
	agent_name TEXT NOT NULL,
	project TEXT NOT NULL,
	connection TEXT NOT NULL,
	source TEXT NOT NULL,
	sql TEXT NOT NULL,
	duration_ms REAL NOT NULL,
	rows INTEGER,
	error TEXT NOT NULL
);`

// Store is the SQLite database holding both logs. After every insert it
// deletes the rows beyond the configured limit, oldest first.
type Store struct {
	db *sql.DB

	mu         sync.Mutex
	agentLimit int
	execLimit  int
}

func Open(path string, agentLimit, execLimit int) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &Store{db: db, agentLimit: agentLimit, execLimit: execLimit}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// SetLimits changes both limits and prunes at once.
func (s *Store) SetLimits(agentLimit, execLimit int) error {
	s.mu.Lock()
	s.agentLimit, s.execLimit = agentLimit, execLimit
	s.mu.Unlock()
	if err := s.prune("agent_log", agentLimit); err != nil {
		return err
	}
	return s.prune("execution_log", execLimit)
}

func (s *Store) limits() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agentLimit, s.execLimit
}

// prune keeps the newest limit rows of table.
func (s *Store) prune(table string, limit int) error {
	_, err := s.db.Exec(`DELETE FROM `+table+` WHERE id <= (SELECT id FROM `+table+` ORDER BY id DESC LIMIT 1 OFFSET ?)`, limit)
	return err
}

// InsertAgent stores e and sets e.ID.
func (s *Store) InsertAgent(e *AgentEntry) error {
	res, err := s.db.Exec(`INSERT INTO agent_log (time, agent_name, project, connection, kind, purpose, sql, status, duration_ms, rows, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Time, e.AgentName, e.Project, e.Connection, e.Kind, e.Purpose, e.SQL, e.Status, e.DurationMs, e.Rows, e.Error)
	if err != nil {
		return err
	}
	if e.ID, err = res.LastInsertId(); err != nil {
		return err
	}
	limit, _ := s.limits()
	return s.prune("agent_log", limit)
}

// UpdateAgent rewrites the outcome of an existing entry in place.
func (s *Store) UpdateAgent(e *AgentEntry) error {
	_, err := s.db.Exec(`UPDATE agent_log SET status = ?, duration_ms = ?, rows = ?, error = ? WHERE id = ?`,
		e.Status, e.DurationMs, e.Rows, e.Error, e.ID)
	return err
}

// CancelWaiting marks every entry still waiting for Approval as cancelled.
func (s *Store) CancelWaiting() error {
	_, err := s.db.Exec(`UPDATE agent_log SET status = 'cancelled' WHERE status = 'waiting'`)
	return err
}

// InsertExec stores e and sets e.ID.
func (s *Store) InsertExec(e *ExecEntry) error {
	res, err := s.db.Exec(`INSERT INTO execution_log (time, by, agent_name, project, connection, source, sql, duration_ms, rows, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Time, e.By, e.AgentName, e.Project, e.Connection, e.Source, e.SQL, e.DurationMs, e.Rows, e.Error)
	if err != nil {
		return err
	}
	if e.ID, err = res.LastInsertId(); err != nil {
		return err
	}
	_, limit := s.limits()
	return s.prune("execution_log", limit)
}

// startBefore maps before <= 0 (no cursor) to past the newest entry.
func startBefore(before int64) int64 {
	if before <= 0 {
		return 1<<63 - 1
	}
	return before
}

// ListAgent returns up to limit entries with id < before, newest first.
func (s *Store) ListAgent(before int64, limit int) ([]AgentEntry, error) {
	before = startBefore(before)
	rows, err := s.db.Query(`SELECT id, time, agent_name, project, connection, kind, purpose, sql, status, duration_ms, rows, error
		FROM agent_log WHERE id < ? ORDER BY id DESC LIMIT ?`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AgentEntry{}
	for rows.Next() {
		var e AgentEntry
		if err := rows.Scan(&e.ID, &e.Time, &e.AgentName, &e.Project, &e.Connection, &e.Kind, &e.Purpose, &e.SQL,
			&e.Status, &e.DurationMs, &e.Rows, &e.Error); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListExec returns up to limit entries with id < before, newest first.
func (s *Store) ListExec(before int64, limit int) ([]ExecEntry, error) {
	before = startBefore(before)
	rows, err := s.db.Query(`SELECT id, time, by, agent_name, project, connection, source, sql, duration_ms, rows, error
		FROM execution_log WHERE id < ? ORDER BY id DESC LIMIT ?`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExecEntry{}
	for rows.Next() {
		var e ExecEntry
		if err := rows.Scan(&e.ID, &e.Time, &e.By, &e.AgentName, &e.Project, &e.Connection, &e.Source, &e.SQL,
			&e.DurationMs, &e.Rows, &e.Error); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
