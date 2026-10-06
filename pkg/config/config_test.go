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

func TestResolveAPIKey(t *testing.T) {
	// Case 1: Direct APIKey field
	p1 := config.ProviderConfig{
		Provider: "groq",
		APIKey:   "literal-secret-key-123",
	}
	if p1.ResolveAPIKey() != "literal-secret-key-123" {
		t.Errorf("expected direct APIKey, got %s", p1.ResolveAPIKey())
	}

	// Case 2: Literal API key pasted into APIKeyEnv (e.g. hex token or prefixed key)
	p2 := config.ProviderConfig{
		Provider:  "custom",
		APIKeyEnv: "cbaa0557c4544f83b6329e46a78c9096",
	}
	if p2.ResolveAPIKey() != "cbaa0557c4544f83b6329e46a78c9096" {
		t.Errorf("expected literal key resolved, got %s", p2.ResolveAPIKey())
	}

	// Case 3: Env var name that IS set
	t.Setenv("TEST_PADR_KEY", "env-secret-val-999")
	p3 := config.ProviderConfig{
		Provider:  "google",
		APIKeyEnv: "TEST_PADR_KEY",
	}
	if p3.ResolveAPIKey() != "env-secret-val-999" {
		t.Errorf("expected env val, got %s", p3.ResolveAPIKey())
	}

	// Case 4: Env var name that is NOT set
	p4 := config.ProviderConfig{
		Provider:  "groq",
		APIKeyEnv: "UNSET_GROQ_KEY_XYZ",
	}
	if p4.ResolveAPIKey() != "" {
		t.Errorf("expected empty string for unset env var name, got %s", p4.ResolveAPIKey())
	}
}

func TestIsEnvVarName(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"GROQ_API_KEY", true},
		{"OPENROUTER_KEY_1", true},
		{"cbaa0557c4544f83b6329e46a78c9096", false}, // hex key with lowercase
		{"sk-proj-12345678", false},                 // dashed token
		{"gsk_abc123xyz", false},                    // lowercase prefix
		{"", false},
	}

	for _, tc := range tests {
		got := config.IsEnvVarName(tc.input)
		if got != tc.expected {
			t.Errorf("IsEnvVarName(%q): expected %v, got %v", tc.input, tc.expected, got)
		}
	}
}

