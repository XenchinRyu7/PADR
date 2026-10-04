package router_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/router"
	"github.com/padr-runner/padr/pkg/store"
)

func TestRouterFallbackAndQuota(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "padr-router-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "padr.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	t.Setenv("TEST_GROQ_KEY", "dummy-key-groq")

	cfg := &config.GlobalConfig{
		Limits: config.LimitsConfig{
			MaxRunsPerDay: 5,
		},
		Providers: map[string]config.ProviderConfig{
			"groq-fast": {
				ID:           "groq-fast",
				Provider:     "groq",
				Model:        "gpt-oss-120b",
				APIKeyEnv:    "TEST_GROQ_KEY",
				MaxDailyRuns: 1,
				Enabled:      true,
			},
			"ollama-local": {
				ID:           "ollama-local",
				Provider:     "ollama",
				Model:        "qwen2.5-coder",
				MaxDailyRuns: 5,
				Enabled:      true,
			},
		},
		Routing: config.RoutingConfig{
			Strategy: "fallback",
			Models:   []string{"groq-fast", "ollama-local"},
		},
	}

	r := router.NewRouter(cfg, s)

	// First candidate should be groq-fast
	candidate, skips, err := r.ResolveCandidate(context.Background())
	if err != nil {
		t.Fatalf("expected resolution to succeed, got %v (skips: %v)", err, skips)
	}
	if candidate.ID != "groq-fast" {
		t.Errorf("expected groq-fast, got %s", candidate.ID)
	}

	// Consume groq quota
	today := time.Now().Format("2006-01-02")
	if err := s.IncrementProviderUsage("groq-fast", today); err != nil {
		t.Fatalf("IncrementProviderUsage failed: %v", err)
	}

	// Next candidate should fallback to ollama-local
	fallbackCandidate, skips, err := r.ResolveCandidate(context.Background())
	if err != nil {
		t.Fatalf("expected fallback to succeed, got %v", err)
	}
	if fallbackCandidate.ID != "ollama-local" {
		t.Errorf("expected fallback to ollama-local, got %s", fallbackCandidate.ID)
	}
	if len(skips) != 1 || skips[0].ProviderID != "groq-fast" {
		t.Errorf("expected groq-fast in skips list, got %+v", skips)
	}
}

func TestIsRateLimitError(t *testing.T) {
	cases := []struct {
		errText string
		expect  bool
	}{
		{"Error 429: Too Many Requests", true},
		{"OpenAI API rate limit reached, please wait", true},
		{"File not found: ROADMAP.md", false},
	}

	for _, c := range cases {
		got := router.IsRateLimitOrQuotaError(c.errText)
		if got != c.expect {
			t.Errorf("for %q expected %v, got %v", c.errText, c.expect, got)
		}
	}
}
