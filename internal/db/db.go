package db

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const Schema = `
CREATE TABLE IF NOT EXISTS nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    label TEXT UNIQUE,
    type TEXT,
    status TEXT DEFAULT 'IN',
    content TEXT,
    confidence REAL DEFAULT 1.0,
    needs_review INTEGER NOT NULL DEFAULT 0,
    created_at TEXT,
    agent_label TEXT
);

CREATE TABLE IF NOT EXISTS edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_id INTEGER,
    to_id INTEGER,
    relation TEXT,
    FOREIGN KEY(from_id) REFERENCES nodes(id),
    FOREIGN KEY(to_id) REFERENCES nodes(id)
);
CREATE TABLE IF NOT EXISTS justifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id INTEGER,
    group_id INTEGER NOT NULL DEFAULT 1,
    justifier_id INTEGER,
    FOREIGN KEY(node_id) REFERENCES nodes(id),
    FOREIGN KEY(justifier_id) REFERENCES nodes(id)
);
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL,
    event_type TEXT NOT NULL,
    node_label TEXT NOT NULL,
    agent_label TEXT,
    old_status TEXT,
    new_status TEXT,
    old_confidence REAL,
    new_confidence REAL,
    detail TEXT
);
`

type Event struct {
	ID            int64
	OccurredAt    string
	EventType     string
	NodeLabel     string
	AgentLabel    sql.NullString
	OldStatus     sql.NullString
	NewStatus     sql.NullString
	OldConfidence sql.NullFloat64
	NewConfidence sql.NullFloat64
	Detail        sql.NullString
}

func InitDB(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, err
	}

	// Enable Foreign Keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, err
	}

	// Create tables
	if _, err := db.Exec(Schema); err != nil {
		db.Close()
		return nil, err
	}

	// Migrate existing tables safely.
	_, _ = db.Exec("ALTER TABLE justifications ADD COLUMN group_id INTEGER NOT NULL DEFAULT 1")
	_, _ = db.Exec("ALTER TABLE nodes ADD COLUMN needs_review INTEGER NOT NULL DEFAULT 0")
	_, _ = db.Exec("ALTER TABLE nodes ADD COLUMN created_at TEXT")
	_, _ = db.Exec("ALTER TABLE nodes ADD COLUMN agent_label TEXT")

	return db, nil
}

func AppendEvent(db *sql.DB, occurredAt, eventType, nodeLabel string, agentLabel, oldStatus, newStatus sql.NullString, oldConfidence, newConfidence sql.NullFloat64, detail sql.NullString) error {
	_, err := db.Exec(`INSERT INTO events (
occurred_at, event_type, node_label, agent_label,
old_status, new_status, old_confidence, new_confidence, detail
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		occurredAt, eventType, nodeLabel, agentLabel,
		oldStatus, newStatus, oldConfidence, newConfidence, detail)
	return err
}

func ListEvents(db *sql.DB, nodeLabel, eventType string, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `SELECT id, occurred_at, event_type, node_label, agent_label,
old_status, new_status, old_confidence, new_confidence, detail
FROM events WHERE 1=1`
	args := make([]any, 0, 3)
	if nodeLabel != "" {
		query += " AND node_label = ?"
		args = append(args, nodeLabel)
	}
	if eventType != "" {
		query += " AND event_type = ?"
		args = append(args, eventType)
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(
			&e.ID,
			&e.OccurredAt,
			&e.EventType,
			&e.NodeLabel,
			&e.AgentLabel,
			&e.OldStatus,
			&e.NewStatus,
			&e.OldConfidence,
			&e.NewConfidence,
			&e.Detail,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func NowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
