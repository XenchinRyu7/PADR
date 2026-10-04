package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/runner"
	"github.com/padr-runner/padr/pkg/store"
	"github.com/spf13/cobra"
)

var (
	runAll         bool
	runDryRun      bool
	engineOverride string
)

var runCmd = &cobra.Command{
	Use:   "run [project]",
	Short: "Execute autonomous development session for a project",
	Run: func(cmd *cobra.Command, args []string) {
		globalCfg, err := config.LoadGlobalConfig()
		checkErr(err, "Failed to load global config")

		stateDir, err := config.GetStateDir()
		checkErr(err, "Failed to get state directory")

		dbPath := filepath.Join(stateDir, "padr.db")
		s, err := store.NewStore(dbPath)
		checkErr(err, "Failed to connect to SQLite store")
		defer s.Close()

		orch := runner.NewOrchestrator(globalCfg, s)
		opts := runner.RunOptions{
			EngineOverride: engineOverride,
			DryRun:         runDryRun,
		}

		if runAll {
			projects, err := config.ListProjects()
			checkErr(err, "Failed to list projects")
			if len(projects) == 0 {
				printWarn("No projects registered to run. Use 'padr project add' first.")
				return
			}

			for _, p := range projects {
				executeProjectRun(orch, p, opts)
			}
			return
		}

		if len(args) == 0 {
			// Try finding project in current directory
			proj, err := config.LoadProjectConfig(".")
			if err != nil {
				color.Red("Please specify a project name or run from a project directory with .padr/project.yaml")
				color.Yellow("Usage: padr run <project> or padr run --all")
				return
			}
			executeProjectRun(orch, proj, opts)
			return
		}

		projectName := args[0]
		proj, err := config.FindProjectByName(projectName)
		checkErr(err, fmt.Sprintf("Project '%s' not found", projectName))

		executeProjectRun(orch, proj, opts)
	},
}

func executeProjectRun(orch *runner.Orchestrator, proj *config.ProjectConfig, opts runner.RunOptions) {
	color.Cyan("\n▶ Starting autonomous run for project: %s (%s)", proj.Name, proj.Repository.Path)
	rec, err := orch.RunProject(context.Background(), proj, opts)

	if err != nil {
		if rec != nil && rec.Status == "skipped" {
			color.Yellow("⏭ Run skipped: %v", err)
			return
		}
		color.Red("✗ Run failed: %v", err)
		return
	}

	color.Green("✓ Autonomous run completed successfully!")
	fmt.Printf("  • Provider:  %s (%s)\n", rec.Provider, rec.Model)
	fmt.Printf("  • Commits:   %d\n", rec.Commits)
	fmt.Printf("  • Tasks:     %d\n", rec.TasksCompleted)
	fmt.Printf("  • Duration:  %ds\n", rec.DurationSeconds)
	if rec.DiffSummary != "" {
		fmt.Printf("  • Changes:\n%s\n", rec.DiffSummary)
	}
}

func init() {
	runCmd.Flags().BoolVar(&runAll, "all", false, "Run autonomous cycle across all registered projects")
	runCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "Simulate execution with mock engine without touching remote")
	runCmd.Flags().StringVar(&engineOverride, "engine", "", "Override agent engine (e.g. opencode, mock)")
}
