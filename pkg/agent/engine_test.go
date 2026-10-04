package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/padr-runner/padr/pkg/agent"
	"github.com/padr-runner/padr/pkg/config"
)

func TestBuildAutonomousPrompt(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "padr-agent-prompt-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	roadmapPath := filepath.Join(tempDir, "ROADMAP.md")
	if err := os.WriteFile(roadmapPath, []byte("# Roadmap\n- [ ] Task 1\n"), 0644); err != nil {
		t.Fatalf("failed to write roadmap: %v", err)
	}

	req := agent.AgentRunRequest{
		ProjectName:        "super-app",
		RepoDir:            tempDir,
		MaxTasks:           2,
		RoadmapFile:        "ROADMAP.md",
		ValidationCommands: []string{"go test ./..."},
	}

	prompt := agent.BuildAutonomousPrompt(req)
	if !strings.Contains(prompt, "super-app") {
		t.Errorf("prompt missing project name")
	}
	if !strings.Contains(prompt, "go test ./...") {
		t.Errorf("prompt missing validation command")
	}
	if !strings.Contains(prompt, "Complete at most 2 task(s)") {
		t.Errorf("prompt missing max tasks constraint")
	}
}

func TestMockEngineExecution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "padr-mock-run-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mockEngine := agent.NewMockEngine()
	req := agent.AgentRunRequest{
		ProjectName: "demo",
		RepoDir:     tempDir,
		Provider: config.ProviderConfig{
			ID:    "groq-fast",
			Model: "gpt-oss-120b",
		},
	}

	res, err := mockEngine.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("mock run failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected mock run to succeed")
	}

	progressFile := filepath.Join(tempDir, "ROADMAP_PROGRESS.md")
	if _, err := os.Stat(progressFile); os.IsNotExist(err) {
		t.Errorf("expected progress file to be created")
	}
}
