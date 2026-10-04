package runner_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/runner"
	"github.com/padr-runner/padr/pkg/store"
)

func setupTestGitProject(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "padr-runner-git-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v, out: %s", args, err, string(out))
		}
	}

	run("init")
	run("config", "user.name", "Test Runner")
	run("config", "user.email", "test@padr.local")

	initFile := filepath.Join(dir, "ROADMAP.md")
	if err := os.WriteFile(initFile, []byte("# Roadmap\n- [ ] Task A\n"), 0644); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}
	run("add", "ROADMAP.md")
	run("commit", "-m", "chore: initial commit")

	return dir
}

func TestOrchestratorSuccessFlow(t *testing.T) {
	tempHome, err := os.MkdirTemp("", "padr-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tempHome)
	t.Setenv("PADR_HOME", tempHome)

	repoDir := setupTestGitProject(t)
	defer os.RemoveAll(repoDir)

	dbPath := filepath.Join(tempHome, "state", "padr.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	cfg := &config.GlobalConfig{
		Limits: config.LimitsConfig{
			MaxRunsPerDay:      5,
			MaxRuntimeMinutes:  5,
			MaxTasksPerRun:     2,
			MaxCommitsPerRun:   5,
		},
		Providers: map[string]config.ProviderConfig{
			"mock-local": {
				ID:           "mock-local",
				Provider:     "ollama",
				Model:        "mock-model",
				MaxDailyRuns: 5,
				Enabled:      true,
			},
		},
		Routing: config.RoutingConfig{
			Strategy: "fallback",
			Models:   []string{"mock-local"},
		},
	}

	proj := config.DefaultProjectConfig("demo-app", repoDir)
	proj.Agent.Engine = "mock"
	proj.Git.AutoPush = false // no remote in test repo

	orch := runner.NewOrchestrator(cfg, s)
	rec, err := orch.RunProject(context.Background(), proj, runner.RunOptions{
		EngineOverride: "mock",
	})

	if err != nil {
		t.Fatalf("RunProject returned error: %v", err)
	}
	if rec.Status != "success" {
		t.Errorf("expected run status 'success', got '%s'", rec.Status)
	}
	if rec.Commits != 1 {
		t.Errorf("expected 1 commit created, got %d", rec.Commits)
	}

	// Verify run recorded in store
	runs, err := s.ListRuns("demo-app", 10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("expected 1 run in store, got %d (err: %v)", len(runs), err)
	}
}

func TestOrchestratorDirtyRepoStops(t *testing.T) {
	tempHome, err := os.MkdirTemp("", "padr-home-*")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tempHome)
	t.Setenv("PADR_HOME", tempHome)

	repoDir := setupTestGitProject(t)
	defer os.RemoveAll(repoDir)

	dbPath := filepath.Join(tempHome, "state", "padr.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	// Make working tree dirty
	uncommitted := filepath.Join(repoDir, "user_wip.txt")
	_ = os.WriteFile(uncommitted, []byte("in progress work"), 0644)

	cfg := &config.GlobalConfig{
		Limits: config.LimitsConfig{MaxRunsPerDay: 5, MaxRuntimeMinutes: 5},
		Providers: map[string]config.ProviderConfig{
			"mock-local": {ID: "mock-local", Provider: "ollama", Enabled: true},
		},
		Routing: config.RoutingConfig{Strategy: "fallback", Models: []string{"mock-local"}},
	}

	proj := config.DefaultProjectConfig("demo-app", repoDir)
	orch := runner.NewOrchestrator(cfg, s)

	rec, err := orch.RunProject(context.Background(), proj, runner.RunOptions{})
	if err == nil {
		t.Fatalf("expected error on dirty working tree")
	}
	if rec.Status != "skipped" {
		t.Errorf("expected status 'skipped', got '%s'", rec.Status)
	}
}
