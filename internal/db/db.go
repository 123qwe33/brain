package db

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

const Schema = `
CREATE TABLE IF NOT EXISTS nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    label TEXT UNIQUE,
    type TEXT,
    status TEXT DEFAULT 'IN',
    content TEXT,
    confidence REAL
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
`

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

	// Migrate existing justifications table if group_id column is absent
	_, _ = db.Exec("ALTER TABLE justifications ADD COLUMN group_id INTEGER NOT NULL DEFAULT 1")

	return db, nil
}
