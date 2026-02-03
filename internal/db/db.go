package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

func New(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	d := &DB{db}
	if err := d.migrate(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *DB) migrate() error {
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS schedulers (
			id TEXT PRIMARY KEY,
			cron TEXT NOT NULL,
			tools TEXT, -- JSON array of tools
			prompt TEXT,
			adapter TEXT,
			target TEXT,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS skills (
			name TEXT PRIMARY KEY,
			description TEXT,
			parameters TEXT, -- JSON schema
			code TEXT, -- If implementing dynamic skills
			enabled BOOLEAN DEFAULT 1
		);`,
		`CREATE TABLE IF NOT EXISTS prompts (
			name TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			description TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bucket TEXT NOT NULL, -- 'note', 'list', etc.
			key TEXT, -- e.g. list name
			value TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS credentials (
			service TEXT PRIMARY KEY,
			token TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, schema := range schemas {
		if _, err := d.Exec(schema); err != nil {
			return fmt.Errorf("migration failed: %v\nquery: %s", err, schema)
		}
	}
	return nil
}

// -- Schedulers --

type SchedulerJob struct {
	ID          string
	Cron        string
	ToolsJSON   string
	Prompt      string
	Adapter     string
	Target      string
	Description string
}

func (d *DB) AddJob(id, cron, tools, prompt, adapter, target, desc string) error {
	_, err := d.Exec(`INSERT OR REPLACE INTO schedulers (id, cron, tools, prompt, adapter, target, description) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, cron, tools, prompt, adapter, target, desc)
	return err
}

func (d *DB) ListJobs() ([]SchedulerJob, error) {
	rows, err := d.Query(`SELECT id, cron, tools, prompt, adapter, target, description FROM schedulers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []SchedulerJob
	for rows.Next() {
		var j SchedulerJob
		if err := rows.Scan(&j.ID, &j.Cron, &j.ToolsJSON, &j.Prompt, &j.Adapter, &j.Target, &j.Description); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// -- Memories (Lists/Notes) --

func (d *DB) AddMemory(bucket, key, value string) error {
	_, err := d.Exec(`INSERT INTO memories (bucket, key, value) VALUES (?, ?, ?)`, bucket, key, value)
	return err
}

func (d *DB) ListMemories(bucket, key string) ([]string, error) {
	rows, err := d.Query(`SELECT value FROM memories WHERE bucket = ? AND key = ? ORDER BY created_at ASC`, bucket, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			return nil, err
		}
		items = append(items, val)
	}
	return items, nil
}

func (d *DB) RemoveMemory(bucket, key, value string) error {
	// Simple remove by value match
	_, err := d.Exec(`DELETE FROM memories WHERE bucket = ? AND key = ? AND value = ?`, bucket, key, value)
	return err
}
