package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/padr-runner/padr/pkg/git"
)

func setupTestGitRepo(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "padr-git-test-*")
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

	initFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(initFile, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")

	return dir
}

func TestGitSafetyOperations(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	defer os.RemoveAll(repoDir)

	ctx := context.Background()
	mgr := git.NewManager(repoDir)

	if !mgr.IsGitRepo(ctx) {
		t.Errorf("expected directory to be recognized as git repo")
	}

	clean, _, err := mgr.CheckClean(ctx)
	if err != nil || !clean {
		t.Fatalf("expected initial repo to be clean, got clean=%v, err=%v", clean, err)
	}

	baseCommit, err := mgr.GetHeadCommit(ctx)
	if err != nil || baseCommit == "" {
		t.Fatalf("failed to get head commit: %v", err)
	}

	// Make a dirty change
	dirtyFile := filepath.Join(repoDir, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("dirty content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	clean, diffSummary, err := mgr.CheckClean(ctx)
	if clean || err == nil {
		t.Errorf("expected dirty check to fail, got clean=%v, err=%v", clean, err)
	}
	if diffSummary == "" {
		t.Errorf("expected diff summary for dirty repo")
	}

	hasChanges, err := mgr.HasChanges(ctx)
	if err != nil || !hasChanges {
		t.Errorf("expected HasChanges to be true")
	}

	newCommit, err := mgr.StageAndCommit(ctx, "feat: add dirty.txt")
	if err != nil {
		t.Fatalf("StageAndCommit failed: %v", err)
	}
	if newCommit == baseCommit {
		t.Errorf("expected new commit hash, got same as base")
	}

	count, err := mgr.CountCommitsSince(ctx, baseCommit)
	if err != nil {
		t.Fatalf("CountCommitsSince failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 commit since base, got %d", count)
	}
}

func TestCheckCleanIgnoresPadrFiles(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	defer os.RemoveAll(repoDir)

	ctx := context.Background()
	mgr := git.NewManager(repoDir)

	// Add .padr/project.yaml and PADR_ROADMAP.md
	padrDir := filepath.Join(repoDir, ".padr")
	_ = os.MkdirAll(padrDir, 0755)
	_ = os.WriteFile(filepath.Join(padrDir, "project.yaml"), []byte("name: test\n"), 0644)
	_ = os.WriteFile(filepath.Join(repoDir, "PADR_ROADMAP.md"), []byte("# Roadmap\n- [ ] Task 1\n"), 0644)

	// CheckClean should still consider the repo clean (so PADR can run autonomously)
	clean, _, err := mgr.CheckClean(ctx)
	if err != nil || !clean {
		t.Fatalf("expected CheckClean to ignore .padr and PADR_ROADMAP.md, got clean=%v, err=%v", clean, err)
	}
}

