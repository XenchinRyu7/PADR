package config_test

import (
	"os"
	"testing"

	"github.com/padr-runner/padr/pkg/config"
)

func TestConfigInitAndLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "padr-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Setenv("PADR_HOME", tempDir)

	if err := config.InitPadrHome(); err != nil {
		t.Fatalf("InitPadrHome failed: %v", err)
	}

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}

	if cfg.Limits.MaxRunsPerDay != config.DefaultMaxRunsPerDay {
		t.Errorf("expected MaxRunsPerDay=%d, got %d", config.DefaultMaxRunsPerDay, cfg.Limits.MaxRunsPerDay)
	}

	if len(cfg.Providers) == 0 {
		t.Errorf("expected providers configured by default")
	}

	if len(cfg.Routing.Models) == 0 {
		t.Errorf("expected routing models configured by default")
	}
}

func TestProjectConfigSaveLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "padr-project-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	proj := config.DefaultProjectConfig("demo-app", tempDir)
	proj.Development.MaxTasks = 3

	if err := config.SaveProjectConfig(tempDir, proj); err != nil {
		t.Fatalf("SaveProjectConfig failed: %v", err)
	}

	loaded, err := config.LoadProjectConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	if loaded.Name != "demo-app" {
		t.Errorf("expected project name demo-app, got %s", loaded.Name)
	}
	if loaded.Development.MaxTasks != 3 {
		t.Errorf("expected max tasks 3, got %d", loaded.Development.MaxTasks)
	}
}
