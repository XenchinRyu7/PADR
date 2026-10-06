package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var (
	ErrDirtyWorkingTree = errors.New("repository has uncommitted changes")
	ErrNotGitRepo       = errors.New("not a git repository")
)

// Manager encapsulates git operations for a specific repository
type Manager struct {
	RepoDir string
}

// NewManager creates a git manager targeting the given directory
func NewManager(repoDir string) *Manager {
	return &Manager{RepoDir: repoDir}
}

// runGit executes a git command in RepoDir
func (m *Manager) runGit(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.RepoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w (stderr: %s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}

// IsGitRepo checks if directory is a git repository
func (m *Manager) IsGitRepo(ctx context.Context) bool {
	out, err := m.runGit(ctx, "rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// CheckClean verifies whether working tree has uncommitted or untracked changes outside of PADR files
func (m *Manager) CheckClean(ctx context.Context) (bool, string, error) {
	out, err := m.runGit(ctx, "status", "--porcelain")
	if err != nil {
		return false, "", err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var dirtyLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Porcelain format: "XY path/to/file" or "?? path/to/file"
		parts := strings.Fields(trimmed)
		if len(parts) >= 2 {
			filePath := parts[len(parts)-1]
			normalized := strings.Trim(strings.ReplaceAll(filePath, "\\", "/"), "\"")
			if strings.HasPrefix(normalized, ".padr") || normalized == "PADR_ROADMAP.md" || strings.HasSuffix(normalized, "/PADR_ROADMAP.md") {
				continue // Ignore PADR internal config and roadmap file modifications
			}
		}
		dirtyLines = append(dirtyLines, trimmed)
	}

	if len(dirtyLines) > 0 {
		return false, strings.Join(dirtyLines, "\n"), ErrDirtyWorkingTree
	}

	return true, "", nil
}

// GetCurrentBranch returns the active branch name
func (m *Manager) GetCurrentBranch(ctx context.Context) (string, error) {
	return m.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
}

// Checkout switches to the specified branch
func (m *Manager) Checkout(ctx context.Context, branch string) error {
	_, err := m.runGit(ctx, "checkout", branch)
	return err
}

// PullFastForward performs a fast-forward only pull
func (m *Manager) PullFastForward(ctx context.Context, branch string) error {
	if branch == "" {
		branch = "main"
	}
	_, err := m.runGit(ctx, "pull", "--ff-only", "origin", branch)
	return err
}

// GetHeadCommit returns the current commit hash
func (m *Manager) GetHeadCommit(ctx context.Context) (string, error) {
	return m.runGit(ctx, "rev-parse", "HEAD")
}

// HasChanges returns true if there are staged, unstaged, or untracked modifications
func (m *Manager) HasChanges(ctx context.Context) (bool, error) {
	out, err := m.runGit(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// GetDiffSummary returns git diff statistics
func (m *Manager) GetDiffSummary(ctx context.Context) (string, error) {
	// First check unstaged/staged diff
	diffStat, err := m.runGit(ctx, "diff", "--stat")
	if err != nil {
		return "", err
	}
	if diffStat != "" {
		return diffStat, nil
	}

	// If no tracked diff, check short status for untracked files
	status, err := m.runGit(ctx, "status", "--short")
	if err != nil {
		return "", err
	}
	return status, nil
}

// StageAndCommit stages all changes and creates a commit
func (m *Manager) StageAndCommit(ctx context.Context, message string) (string, error) {
	hasChanges, err := m.HasChanges(ctx)
	if err != nil {
		return "", err
	}
	if !hasChanges {
		return "", errors.New("no changes to commit")
	}

	if _, err := m.runGit(ctx, "add", "-A"); err != nil {
		return "", fmt.Errorf("git add failed: %w", err)
	}

	if _, err := m.runGit(ctx, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("git commit failed: %w", err)
	}

	return m.GetHeadCommit(ctx)
}

// CountCommitsSince returns number of commits between a base hash and HEAD
func (m *Manager) CountCommitsSince(ctx context.Context, baseHash string) (int, error) {
	if baseHash == "" {
		return 0, nil
	}
	out, err := m.runGit(ctx, "rev-list", "--count", fmt.Sprintf("%s..HEAD", baseHash))
	if err != nil {
		return 0, err
	}
	var count int
	_, err = fmt.Sscanf(out, "%d", &count)
	return count, err
}

// Push pushes current branch to remote
func (m *Manager) Push(ctx context.Context, branch string) error {
	if branch == "" {
		branch = "main"
	}
	_, err := m.runGit(ctx, "push", "origin", branch)
	return err
}
