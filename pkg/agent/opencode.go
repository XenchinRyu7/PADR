package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// OpenCodeEngine drives the OpenCode CLI
type OpenCodeEngine struct {
	ExecutablePath string
}

// NewOpenCodeEngine creates an OpenCode adapter
func NewOpenCodeEngine(binPath string) *OpenCodeEngine {
	if binPath == "" {
		binPath = "opencode"
	}
	return &OpenCodeEngine{ExecutablePath: binPath}
}

func (e *OpenCodeEngine) Name() string {
	return "opencode"
}

// Run executes autonomous task session through OpenCode CLI
func (e *OpenCodeEngine) Run(ctx context.Context, req AgentRunRequest) (*AgentRunResult, error) {
	prompt := BuildAutonomousPrompt(req)
	startTime := time.Now()

	// Check if opencode executable exists
	exePath, err := exec.LookPath(e.ExecutablePath)
	if err != nil {
		return &AgentRunResult{
			Success:         false,
			DurationSeconds: int(time.Since(startTime).Seconds()),
			Error:           fmt.Sprintf("OpenCode CLI executable '%s' not found in PATH: %v", e.ExecutablePath, err),
		}, nil
	}

	// Prepare arguments for non-interactive autonomous run
	args := []string{"run"}
	if req.Provider.Model != "" {
		args = append(args, "--model", req.Provider.Model)
	}
	if req.Provider.Endpoint != "" {
		args = append(args, "--base-url", req.Provider.Endpoint)
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, exePath, args...)
	cmd.Dir = req.RepoDir

	// Setup environment variables for provider
	env := os.Environ()
	if req.Provider.APIKeyEnv != "" {
		if val := os.Getenv(req.Provider.APIKeyEnv); val != "" {
			env = append(env, fmt.Sprintf("%s=%s", req.Provider.APIKeyEnv, val))
			env = append(env, fmt.Sprintf("OPENAI_API_KEY=%s", val))
		}
	}
	if req.Provider.Endpoint != "" {
		env = append(env, fmt.Sprintf("OLLAMA_HOST=%s", req.Provider.Endpoint))
		env = append(env, fmt.Sprintf("OPENAI_BASE_URL=%s", req.Provider.Endpoint))
		env = append(env, fmt.Sprintf("OPENAI_API_BASE=%s", req.Provider.Endpoint))
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	duration := int(time.Since(startTime).Seconds())
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n[STDERR]\n" + stderr.String()
	}

	if runErr != nil {
		return &AgentRunResult{
			Success:         false,
			Output:          output,
			DurationSeconds: duration,
			Error:           fmt.Sprintf("opencode run failed: %v", runErr),
		}, nil
	}

	return &AgentRunResult{
		Success:         true,
		Output:          output,
		TasksCompleted:  1,
		DurationSeconds: duration,
	}, nil
}
