package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/olekukonko/tablewriter"
	"github.com/padr-runner/padr/pkg/config"
	"github.com/spf13/cobra"
)

var (
	projectPath   string
	projectBranch string
	projectCron   string
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage monitored projects and repositories",
}

var projectAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add or configure a project for autonomous development",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		repoPath := projectPath
		if repoPath == "" {
			cwd, err := os.Getwd()
			checkErr(err, "Failed to determine current directory")
			repoPath = cwd
		}

		absPath, err := filepath.Abs(repoPath)
		checkErr(err, "Invalid repository path")

		proj := config.DefaultProjectConfig(name, absPath)
		if projectBranch != "" {
			proj.Repository.Branch = projectBranch
		}
		if projectCron != "" {
			proj.Schedule.Cron = projectCron
		}

		// Save in repository .padr/project.yaml
		err = config.SaveProjectConfig(absPath, proj)
		checkErr(err, "Failed to save project.yaml in repository")

		// Register in ~/.padr/projects/
		err = config.RegisterProject(proj)
		checkErr(err, "Failed to register project in PADR registry")

		printSuccess("Project '%s' registered successfully!", name)
		printInfo("Repository path: %s", absPath)
		printInfo("Target branch:   %s", proj.Repository.Branch)
		printInfo("Config created:  %s", filepath.Join(absPath, config.ProjectConfigDirName, config.ProjectConfigFileName))
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered projects",
	Run: func(cmd *cobra.Command, args []string) {
		projects, err := config.ListProjects()
		checkErr(err, "Failed to load project registry")

		if len(projects) == 0 {
			printWarn("No projects registered yet. Use 'padr project add <name>' to register one.")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Name", "Branch", "Engine", "Max Tasks", "Schedule", "Repository Path"})
		table.SetBorder(true)
		table.SetAutoWrapText(false)

		for _, p := range projects {
			sched := p.Schedule.Cron
			if sched == "" {
				sched = p.Schedule.DailyAt
			}
			table.Append([]string{
				p.Name,
				p.Repository.Branch,
				p.Agent.Engine,
				fmt.Sprintf("%d", p.Development.MaxTasks),
				sched,
				p.Repository.Path,
			})
		}

		table.Render()
	},
}

func init() {
	projectAddCmd.Flags().StringVarP(&projectPath, "path", "p", "", "Path to repository (default: current directory)")
	projectAddCmd.Flags().StringVarP(&projectBranch, "branch", "b", "main", "Target branch for automation")
	projectAddCmd.Flags().StringVar(&projectCron, "cron", "0 9 * * *", "Cron schedule trigger")

	projectCmd.AddCommand(projectAddCmd)
	projectCmd.AddCommand(projectListCmd)
}
