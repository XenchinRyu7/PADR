package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/store"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display PADR daemon and project execution status",
	Run: func(cmd *cobra.Command, args []string) {
		home, err := config.GetPadrHome()
		checkErr(err, "Failed to resolve PADR home")

		globalCfg, err := config.LoadGlobalConfig()
		checkErr(err, "Failed to load global config")

		projects, _ := config.ListProjects()

		stateDir, err := config.GetStateDir()
		checkErr(err, "Failed to get state directory")

		dbPath := filepath.Join(stateDir, "padr.db")
		s, err := store.NewStore(dbPath)
		checkErr(err, "Failed to connect to SQLite store")
		defer s.Close()

		today := time.Now().Format("2006-01-02")
		dailyUsage, _ := s.GetTotalDailyUsage(today)

		color.Cyan("\nPADR System Status")
		fmt.Printf("  • Home:                %s\n", home)
		fmt.Printf("  • Registered Projects: %d\n", len(projects))
		fmt.Printf("  • Today Runs:          %d / %d (max per day)\n", dailyUsage, globalCfg.Limits.MaxRunsPerDay)
		fmt.Printf("  • Max Runtime Window:  %d minutes\n", globalCfg.Limits.MaxRuntimeMinutes)
		fmt.Printf("  • Fallback Models:     %d configured\n", len(globalCfg.Routing.Models))

		color.Cyan("\nProject Overview:")
		if len(projects) == 0 {
			fmt.Println("  (No registered projects yet)")
		} else {
			for _, p := range projects {
				lastRun, _ := s.GetLastRun(p.Name)
				lastRunStr := "Never executed"
				if lastRun != nil {
					lastRunStr = fmt.Sprintf("%s (%s, %d commits, %s)",
						lastRun.StartedAt.Format("2006-01-02 15:04"),
						lastRun.Status,
						lastRun.Commits,
						lastRun.Provider)
				}
				fmt.Printf("  • [%s] %s -> Last: %s\n", p.Name, p.Repository.Path, lastRunStr)
			}
		}
		fmt.Println()
	},
}
