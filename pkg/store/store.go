package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// RunRecord represents a single autonomous execution log entry
type RunRecord struct {
	ID              int64     `json:"id"`
	Project         string    `json:"project"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	DurationSeconds int       `json:"duration_seconds"`
	Provider        string    `json:"provider"`
	Model           string    `json:"model"`
	Status          string    `json:"status"` // success, failed, skipped
	Commits         int       `json:"commits"`
	TasksCompleted  int       `json:"tasks_completed"`
	Error           string    `json:"error"`
	DiffSummary     string    `json:"diff_summary"`
	LogOutput       string    `json:"log_output"`
}

// Store provides persistence methods for runs and budget tracking
type Store struct {
	db *sql.DB
}

// NewStore opens or creates the SQLite database at the specified path
func NewStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return s, nil
}

// Close closes the underlying database connection
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// migrate ensures database schema exists
func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project TEXT NOT NULL,
			started_at DATETIME NOT NULL,
			finished_at DATETIME NOT NULL,
			duration_seconds INTEGER NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			status TEXT NOT NULL,
			commits INTEGER NOT NULL DEFAULT 0,
			tasks_completed INTEGER NOT NULL DEFAULT 0,
			error TEXT,
			diff_summary TEXT,
			log_output TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_runs_project ON runs(project);`,
		`CREATE INDEX IF NOT EXISTS idx_runs_started_at ON runs(started_at);`,
		`CREATE TABLE IF NOT EXISTS daily_usage (
			day TEXT NOT NULL,
			provider TEXT NOT NULL,
			run_count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (day, provider)
		);`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

// RecordRun inserts a run record into the database
func (s *Store) RecordRun(r *RunRecord) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO runs (
			project, started_at, finished_at, duration_seconds,
			provider, model, status, commits, tasks_completed,
			error, diff_summary, log_output
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Project, r.StartedAt.UTC(), r.FinishedAt.UTC(), r.DurationSeconds,
		r.Provider, r.Model, r.Status, r.Commits, r.TasksCompleted,
		r.Error, r.DiffSummary, r.LogOutput,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert run: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	r.ID = id
	return id, nil
}

// ListRuns returns run records filtered optionally by project with limit
func (s *Store) ListRuns(project string, limit int) ([]*RunRecord, error) {
	if limit <= 0 {
		limit = 20
	}

	var rows *sql.Rows
	var err error

	if project != "" {
		rows, err = s.db.Query(`
			SELECT id, project, started_at, finished_at, duration_seconds,
			       provider, model, status, commits, tasks_completed,
			       coalesce(error, ''), coalesce(diff_summary, ''), coalesce(log_output, '')
			FROM runs
			WHERE project = ?
			ORDER BY started_at DESC
			LIMIT ?`, project, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT id, project, started_at, finished_at, duration_seconds,
			       provider, model, status, commits, tasks_completed,
			       coalesce(error, ''), coalesce(diff_summary, ''), coalesce(log_output, '')
			FROM runs
			ORDER BY started_at DESC
			LIMIT ?`, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var records []*RunRecord
	for rows.Next() {
		var r RunRecord
		err := rows.Scan(
			&r.ID, &r.Project, &r.StartedAt, &r.FinishedAt, &r.DurationSeconds,
			&r.Provider, &r.Model, &r.Status, &r.Commits, &r.TasksCompleted,
			&r.Error, &r.DiffSummary, &r.LogOutput,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, &r)
	}

	return records, rows.Err()
}

// GetLastRun returns the most recent run record for a project
func (s *Store) GetLastRun(project string) (*RunRecord, error) {
	runs, err := s.ListRuns(project, 1)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return runs[0], nil
}

// IncrementProviderUsage increments daily run count for a given provider
func (s *Store) IncrementProviderUsage(provider string, day string) error {
	_, err := s.db.Exec(`
		INSERT INTO daily_usage (day, provider, run_count)
		VALUES (?, ?, 1)
		ON CONFLICT(day, provider) DO UPDATE SET run_count = run_count + 1`,
		day, provider)
	return err
}

// GetProviderDailyUsage returns the count of runs executed by a provider on a specific day
func (s *Store) GetProviderDailyUsage(provider string, day string) (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT coalesce(run_count, 0)
		FROM daily_usage
		WHERE day = ? AND provider = ?`,
		day, provider).Scan(&count)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}

// GetTotalDailyUsage returns the total count of runs across all providers on a day
func (s *Store) GetTotalDailyUsage(day string) (int, error) {
	var total int
	err := s.db.QueryRow(`
		SELECT coalesce(sum(run_count), 0)
		FROM daily_usage
		WHERE day = ?`, day).Scan(&total)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return total, nil
}
