package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/store"
	"github.com/spf13/cobra"
)

var (
	logsProject string
	logsLimit   int
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show recent autonomous execution logs",
	Run: func(cmd *cobra.Command, args []string) {
		stateDir, err := config.GetStateDir()
		checkErr(err, "Failed to get state directory")

		dbPath := filepath.Join(stateDir, "padr.db")
		s, err := store.NewStore(dbPath)
		checkErr(err, "Failed to connect to SQLite store")
		defer s.Close()

		runs, err := s.ListRuns(logsProject, logsLimit)
		checkErr(err, "Failed to retrieve logs")

		if len(runs) == 0 {
			printInfo("No execution logs found.")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "Project", "Started At", "Status", "Provider", "Model", "Commits", "Tasks", "Duration"})
		table.SetBorder(true)
		table.SetAutoWrapText(false)

		for _, r := range runs {
			statusStr := r.Status
			switch r.Status {
			case "success":
				statusStr = color.GreenString("✓ success")
			case "failed":
				statusStr = color.RedString("✗ failed")
			case "skipped":
				statusStr = color.YellowString("⏭ skipped")
			}

			table.Append([]string{
				fmt.Sprintf("%d", r.ID),
				r.Project,
				r.StartedAt.Format("2006-01-02 15:04:05"),
				statusStr,
				r.Provider,
				r.Model,
				fmt.Sprintf("%d", r.Commits),
				fmt.Sprintf("%d", r.TasksCompleted),
				fmt.Sprintf("%ds", r.DurationSeconds),
			})
		}

		table.Render()
	},
}

func init() {
	logsCmd.Flags().StringVarP(&logsProject, "project", "p", "", "Filter logs by project name")
	logsCmd.Flags().IntVarP(&logsLimit, "limit", "n", 20, "Number of log records to display")
}
