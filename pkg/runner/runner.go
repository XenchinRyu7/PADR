package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/padr-runner/padr/pkg/agent"
	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/git"
	"github.com/padr-runner/padr/pkg/router"
	"github.com/padr-runner/padr/pkg/store"
)

// RunOptions configures the execution of a project
type RunOptions struct {
	EngineOverride   string
	ProviderOverride string
	DryRun           bool
	CustomPrompt     string
	OnProgress       func(string)
}

// Orchestrator coordinates the end-to-end autonomous workflow for projects
type Orchestrator struct {
	GlobalConfig *config.GlobalConfig
	Store        *store.Store
	Router       *router.Router
}

// NewOrchestrator creates a new Orchestrator
func NewOrchestrator(cfg *config.GlobalConfig, s *store.Store) *Orchestrator {
	return &Orchestrator{
		GlobalConfig: cfg,
		Store:        s,
		Router:       router.NewRouter(cfg, s),
	}
}

// RunProject executes the complete pipeline for a single project
func (o *Orchestrator) RunProject(ctx context.Context, proj *config.ProjectConfig, opts RunOptions) (*store.RunRecord, error) {
	startedAt := time.Now()
	repoDir := proj.Repository.Path

	record := &store.RunRecord{
		Project:   proj.Name,
		StartedAt: startedAt,
		Status:    "running",
	}

	// 1. Git Safety Checks
	gitMgr := git.NewManager(repoDir)
	if !gitMgr.IsGitRepo(ctx) {
		record.Status = "failed"
		record.Error = fmt.Sprintf("directory '%s' is not a git repository", repoDir)
		record.FinishedAt = time.Now()
		record.DurationSeconds = int(time.Since(startedAt).Seconds())
		o.persistRun(record)
		return record, fmt.Errorf("%s", record.Error)
	}

	// Clean / Dirty check (PRD Section 8: STOP if dirty)
	clean, dirtyDiff, err := gitMgr.CheckClean(ctx)
	if !clean || err != nil {
		record.Status = "skipped"
		record.Error = "Repository has uncommitted changes (dirty working tree)"
		record.DiffSummary = dirtyDiff
		record.FinishedAt = time.Now()
		record.DurationSeconds = int(time.Since(startedAt).Seconds())
		o.persistRun(record)
		return record, fmt.Errorf("%s", record.Error)
	}

	// 2. Checkout target branch and pull if configured
	if proj.Repository.Branch != "" {
		currentBranch, err := gitMgr.GetCurrentBranch(ctx)
		if err == nil && currentBranch != proj.Repository.Branch {
			_ = gitMgr.Checkout(ctx, proj.Repository.Branch)
		}
	}

	if proj.Git.AutoPull {
		_ = gitMgr.PullFastForward(ctx, proj.Repository.Branch)
	}

	baseCommit, _ := gitMgr.GetHeadCommit(ctx)

	// 3. Resolve Model Candidate with Fallback (or Override)
	var provider *config.ProviderConfig
	// 3. Resolve Provider Candidate
	var candidates []*config.ProviderConfig
	var skips []router.SkipReason
	if opts.ProviderOverride != "" && opts.ProviderOverride != "Auto (Router Fallback)" {
		if p, ok := o.GlobalConfig.Providers[opts.ProviderOverride]; ok {
			candidates = []*config.ProviderConfig{&p}
		} else {
			record.Status = "failed"
			record.Error = fmt.Sprintf("specified provider '%s' not found in configuration", opts.ProviderOverride)
			record.FinishedAt = time.Now()
			record.DurationSeconds = int(time.Since(startedAt).Seconds())
			o.persistRun(record)
			return record, fmt.Errorf("%s", record.Error)
		}
	} else {
		var err error
		candidates, skips, err = o.Router.ResolveAllCandidates(ctx)
		if err != nil {
			record.Status = "skipped"
			record.Error = err.Error()
			record.FinishedAt = time.Now()
			record.DurationSeconds = int(time.Since(startedAt).Seconds())
			o.persistRun(record)
			return record, err
		}
	}

	provider = candidates[0]
	record.Provider = provider.ID
	record.Model = provider.Model

	// 4. Determine Agent Engine
	engineName := proj.Agent.Engine
	if opts.EngineOverride != "" {
		engineName = opts.EngineOverride
	}
	if opts.DryRun {
		engineName = "mock"
	}

	agentEngine, err := agent.GetEngine(engineName)
	if err != nil {
		record.Status = "failed"
		record.Error = err.Error()
		record.FinishedAt = time.Now()
		record.DurationSeconds = int(time.Since(startedAt).Seconds())
		o.persistRun(record)
		return record, err
	}

	// 5. Setup Execution Window (Timeout)
	maxMinutes := o.GlobalConfig.Limits.MaxRuntimeMinutes
	if maxMinutes <= 0 {
		maxMinutes = 45
	}
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(maxMinutes)*time.Minute)
	defer cancel()

	// 6. Execute Agent (with automatic fallback to next candidate if rate limit or engine error occurs)
	var agentRes *agent.AgentRunResult
	var lastErr error

	for candIdx, cand := range candidates {
		provider = cand
		record.Provider = cand.ID
		record.Model = cand.Model

		req := agent.AgentRunRequest{
			ProjectName:        proj.Name,
			RepoDir:            repoDir,
			Provider:           *cand,
			MaxTasks:           proj.Development.MaxTasks,
			RoadmapFile:        proj.Development.Roadmap,
			RulesFile:          proj.Development.RulesFile,
			ArchitectureFile:   proj.Development.ArchitectureFile,
			ValidationCommands: proj.Validation.Commands,
			CustomPrompt:       opts.CustomPrompt,
			DryRun:             opts.DryRun,
			OnProgress:         opts.OnProgress,
		}

		res, aErr := agentEngine.Run(execCtx, req)
		if aErr == nil && res != nil && res.Success {
			agentRes = res
			lastErr = nil
			break
		}

		errMsg := ""
		if aErr != nil {
			errMsg = aErr.Error()
		} else if res != nil {
			errMsg = res.Error
		}
		lastErr = fmt.Errorf("%s", errMsg)
		if res != nil && res.Output != "" {
			record.LogOutput = res.Output
		}

		// If more candidates remain and we are in auto router fallback mode, try next candidate
		if len(candidates) > 1 && candIdx < len(candidates)-1 {
			fmt.Printf("[Runner] Provider '%s' encountered an issue (%s). Auto-falling back to next provider '%s'...\n",
				cand.ID, errMsg, candidates[candIdx+1].ID)
			continue
		}
	}

	if lastErr != nil {
		record.Status = "failed"
		record.Error = lastErr.Error()
		record.FinishedAt = time.Now()
		record.DurationSeconds = int(time.Since(startedAt).Seconds())
		o.persistRun(record)
		return record, fmt.Errorf("agent run failed: %w", lastErr)
	}

	if agentRes != nil {
		record.TasksCompleted = agentRes.TasksCompleted
		record.LogOutput = agentRes.Output
	}

	// 7. Post-execution Validation Commands
	if len(proj.Validation.Commands) > 0 {
		for _, cmdStr := range proj.Validation.Commands {
			valErr := runValidationCommand(execCtx, repoDir, cmdStr)
			if valErr != nil {
				record.Status = "failed"
				record.Error = fmt.Sprintf("validation command failed (%s): %v", cmdStr, valErr)
				record.FinishedAt = time.Now()
				record.DurationSeconds = int(time.Since(startedAt).Seconds())
				o.persistRun(record)
				return record, fmt.Errorf("%s", record.Error)
			}
		}
	}

	// 8. Review Git Diff & Commit
	diffSummary, _ := gitMgr.GetDiffSummary(ctx)
	record.DiffSummary = diffSummary

	hasChanges, _ := gitMgr.HasChanges(ctx)
	if hasChanges && proj.Git.AutoCommit {
		prefix := proj.Git.CommitMessagePrefix
		if prefix == "" {
			prefix = "chore(padr):"
		}
		commitMsg := fmt.Sprintf("%s autonomous task progression for %s", prefix, proj.Name)
		_, commitErr := gitMgr.StageAndCommit(ctx, commitMsg)
		if commitErr != nil {
			record.Status = "failed"
			record.Error = fmt.Sprintf("commit failed: %v", commitErr)
			record.FinishedAt = time.Now()
			record.DurationSeconds = int(time.Since(startedAt).Seconds())
			o.persistRun(record)
			return record, commitErr
		}
	}

	// Count commits made during this session
	commitsCount, _ := gitMgr.CountCommitsSince(ctx, baseCommit)
	record.Commits = commitsCount

	// 9. Git Push (PRD Section 8)
	if proj.Git.AutoPush && record.Commits > 0 {
		_ = gitMgr.Push(ctx, proj.Repository.Branch)
	}

	// 10. Finalize Run Record
	record.Status = "success"
	record.FinishedAt = time.Now()
	record.DurationSeconds = int(time.Since(startedAt).Seconds())

	// Increment provider usage for today
	today := time.Now().Format("2006-01-02")
	_ = o.Store.IncrementProviderUsage(provider.ID, today)

	o.persistRun(record)
	_ = skips // logged if needed
	return record, nil
}

// runValidationCommand executes a shell command in the project directory
func runValidationCommand(ctx context.Context, dir, commandLine string) error {
	var cmd *exec.Cmd
	// Use powershell on windows or sh on unix
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") || os.PathSeparator == '\\' {
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", commandLine)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", commandLine)
	}
	cmd.Dir = dir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// persistRun saves to SQLite and writes a JSON snapshot in ~/.padr/runs
func (o *Orchestrator) persistRun(r *store.RunRecord) {
	_, _ = o.Store.RecordRun(r)

	runsDir, err := config.GetRunsDir()
	if err == nil {
		filename := fmt.Sprintf("%s_%s_%d.json",
			r.StartedAt.Format("20060102_150405"),
			r.Project,
			time.Now().UnixNano()%10000)
		runFilePath := filepath.Join(runsDir, filename)
		data, err := json.MarshalIndent(r, "", "  ")
		if err == nil {
			_ = os.WriteFile(runFilePath, data, 0644)
		}
	}
}
