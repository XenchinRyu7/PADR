package cmd

import (
	"context"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/scheduler"
	"github.com/spf13/cobra"
)

var scheduleTime string

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage OS-level background scheduled tasks",
}

var scheduleInstallCmd = &cobra.Command{
	Use:   "install <project>",
	Short: "Install OS scheduled task (e.g. Windows Task Scheduler) for project",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := args[0]
		proj, err := config.FindProjectByName(projectName)
		checkErr(err, "Failed to resolve project")

		targetTime := scheduleTime
		if targetTime == "" {
			if proj.Schedule.DailyAt != "" {
				targetTime = proj.Schedule.DailyAt
			} else if proj.Schedule.Cron != "" {
				targetTime = proj.Schedule.Cron
			} else {
				targetTime = "09:00"
			}
		}

		sched := scheduler.NewScheduler()
		err = sched.Install(context.Background(), projectName, targetTime, "")
		checkErr(err, "Failed to install scheduled task")

		printSuccess("Scheduled task for '%s' installed successfully (Trigger: %s)!", projectName, targetTime)
	},
}

var scheduleRemoveCmd = &cobra.Command{
	Use:   "remove <project>",
	Short: "Uninstall scheduled task for project",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := args[0]
		sched := scheduler.NewScheduler()
		err := sched.Uninstall(context.Background(), projectName)
		checkErr(err, "Failed to remove scheduled task")

		printSuccess("Scheduled task for '%s' removed successfully!", projectName)
	},
}

var scheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active PADR scheduled tasks",
	Run: func(cmd *cobra.Command, args []string) {
		sched := scheduler.NewScheduler()
		tasks, err := sched.List(context.Background())
		checkErr(err, "Failed to query scheduled tasks")

		if len(tasks) == 0 {
			printInfo("No PADR scheduled tasks found.")
			return
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Task Name", "Next Run Time", "Status"})
		table.SetBorder(true)
		table.SetAutoWrapText(false)

		for _, t := range tasks {
			table.Append([]string{
				t.Name,
				t.NextRun,
				t.Status,
			})
		}

		table.Render()
	},
}

func init() {
	scheduleInstallCmd.Flags().StringVarP(&scheduleTime, "time", "t", "", "Execution time (e.g. 09:00 or cron)")

	scheduleCmd.AddCommand(scheduleInstallCmd)
	scheduleCmd.AddCommand(scheduleRemoveCmd)
	scheduleCmd.AddCommand(scheduleListCmd)
}
