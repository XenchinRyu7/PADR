package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/padr-runner/padr/pkg/store"
)

func TestStoreRunsAndUsage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "padr-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "state", "padr.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	now := time.Now()
	run := &store.RunRecord{
		Project:         "test-erp",
		StartedAt:       now.Add(-10 * time.Minute),
		FinishedAt:      now,
		DurationSeconds: 600,
		Provider:        "groq",
		Model:           "openai/gpt-oss-120b",
		Status:          "success",
		Commits:         2,
		TasksCompleted:  1,
	}

	id, err := s.RecordRun(run)
	if err != nil {
		t.Fatalf("RecordRun failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive run ID, got %d", id)
	}

	runs, err := s.ListRuns("test-erp", 10)
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Status != "success" || runs[0].Commits != 2 {
		t.Errorf("unexpected run record data: %+v", runs[0])
	}

	today := time.Now().Format("2006-01-02")
	if err := s.IncrementProviderUsage("groq", today); err != nil {
		t.Fatalf("IncrementProviderUsage failed: %v", err)
	}
	if err := s.IncrementProviderUsage("groq", today); err != nil {
		t.Fatalf("IncrementProviderUsage failed: %v", err)
	}

	count, err := s.GetProviderDailyUsage("groq", today)
	if err != nil {
		t.Fatalf("GetProviderDailyUsage failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 usage count, got %d", count)
	}

	total, err := s.GetTotalDailyUsage(today)
	if err != nil {
		t.Fatalf("GetTotalDailyUsage failed: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 total usage count, got %d", total)
	}
}
