package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/padr-runner/padr/pkg/config"
)

// AgentRunRequest contains all execution parameters passed to an agent engine
type AgentRunRequest struct {
	ProjectName        string
	RepoDir            string
	Provider           config.ProviderConfig
	MaxTasks           int
	RoadmapFile        string
	RulesFile          string
	ArchitectureFile   string
	ValidationCommands []string
	CustomPrompt       string
	DryRun             bool
}

// AgentRunResult captures execution telemetry and output
type AgentRunResult struct {
	Success         bool
	Output          string
	TasksCompleted  int
	DurationSeconds int
	Error           string
}

// AgentEngine defines standard interface for AI agent orchestrators (OpenCode, Cline, Aider)
type AgentEngine interface {
	Name() string
	Run(ctx context.Context, req AgentRunRequest) (*AgentRunResult, error)
}

// BuildAutonomousPrompt constructs the system instructions for autonomous development
func BuildAutonomousPrompt(req AgentRunRequest) string {
	if req.CustomPrompt != "" {
		return req.CustomPrompt
	}

	maxTasks := req.MaxTasks
	if maxTasks <= 0 {
		maxTasks = 2
	}

	roadmap := req.RoadmapFile
	if roadmap == "" || roadmap == "ROADMAP.md" {
		if fileExists(filepath.Join(req.RepoDir, "PADR_ROADMAP.md")) {
			roadmap = "PADR_ROADMAP.md"
		} else {
			roadmap = "ROADMAP.md"
		}
	}

	var readDocs []string
	if req.RulesFile != "" && fileExists(filepath.Join(req.RepoDir, req.RulesFile)) {
		readDocs = append(readDocs, fmt.Sprintf("- %s", req.RulesFile))
	} else if fileExists(filepath.Join(req.RepoDir, "PROJECT.md")) {
		readDocs = append(readDocs, "- PROJECT.md")
	}

	readDocs = append(readDocs, fmt.Sprintf("- %s", roadmap))

	if req.ArchitectureFile != "" && fileExists(filepath.Join(req.RepoDir, req.ArchitectureFile)) {
		readDocs = append(readDocs, fmt.Sprintf("- %s", req.ArchitectureFile))
	} else if fileExists(filepath.Join(req.RepoDir, "ARCHITECTURE.md")) {
		readDocs = append(readDocs, "- ARCHITECTURE.md")
	}

	var validationSection string
	if len(req.ValidationCommands) > 0 {
		validationSection = fmt.Sprintf("\nValidation commands to verify after changes:\n%s\n",
			strings.Join(req.ValidationCommands, "\n"))
	}

	return fmt.Sprintf(`You are the autonomous developer for this repository (%s).

Read and understand the context from:
%s
%s
Rules:
1. Pick the next incomplete task from %s.
2. Do not work on unrelated features or speculative code.
3. Keep changes minimal, clean, and focused on the selected task.
4. Run project validation commands and fix any failures caused by your changes.
5. Do not modify or leak credentials, tokens, or .env secrets.
6. Do not rewrite existing architecture or abstractions without clear justification.
7. Ensure all code compiles and unit tests pass before completing.

Complete at most %d task(s).`,
		req.ProjectName,
		strings.Join(readDocs, "\n"),
		validationSection,
		roadmap,
		maxTasks,
	)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// GetEngine returns registered agent engine by name
func GetEngine(name string) (AgentEngine, error) {
	switch strings.ToLower(name) {
	case "opencode", "":
		return NewOpenCodeEngine(""), nil
	case "mock":
		return NewMockEngine(), nil
	default:
		return nil, fmt.Errorf("unsupported agent engine: %s (supported: opencode, mock)", name)
	}
}
