package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// MockEngine simulates agent activity for testing and dry-runs
type MockEngine struct {
	ShouldFail bool
	ErrorMessage string
}

// NewMockEngine creates a new mock agent engine
func NewMockEngine() *MockEngine {
	return &MockEngine{}
}

func (m *MockEngine) Name() string {
	return "mock"
}

func (m *MockEngine) Run(ctx context.Context, req AgentRunRequest) (*AgentRunResult, error) {
	startTime := time.Now()

	if m.ShouldFail {
		return &AgentRunResult{
			Success:         false,
			Error:           m.ErrorMessage,
			DurationSeconds: 1,
		}, nil
	}

	// Simulate work by appending to or creating a progress file in the target repository
	progressFile := filepath.Join(req.RepoDir, "ROADMAP_PROGRESS.md")
	content := fmt.Sprintf("\n- [x] Task completed autonomously by PADR (%s) at %s\n",
		req.Provider.ID, time.Now().Format(time.RFC3339))

	f, err := os.OpenFile(progressFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write mock progress: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return nil, err
	}

	return &AgentRunResult{
		Success:         true,
		Output:          fmt.Sprintf("Mock task executed successfully using provider %s", req.Provider.ID),
		TasksCompleted:  1,
		DurationSeconds: int(time.Since(startTime).Seconds()),
	}, nil
}
