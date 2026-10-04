package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/store"
	"github.com/spf13/cobra"
)

var quotaCmd = &cobra.Command{
	Use:   "quota",
	Short: "Check daily budget limits, provider consumption, and remaining quota",
	Run: func(cmd *cobra.Command, args []string) {
		globalCfg, err := config.LoadGlobalConfig()
		checkErr(err, "Failed to load global config")

		stateDir, err := config.GetStateDir()
		checkErr(err, "Failed to get state directory")

		dbPath := filepath.Join(stateDir, "padr.db")
		s, err := store.NewStore(dbPath)
		checkErr(err, "Failed to connect to SQLite store")
		defer s.Close()

		today := time.Now().Format("2006-01-02")
		totalUsed, _ := s.GetTotalDailyUsage(today)

		color.Cyan("\nDaily Budget Summary (%s):", today)
		fmt.Printf("  • Total Runs Today: %d / %d\n", totalUsed, globalCfg.Limits.MaxRunsPerDay)
		fmt.Printf("  • Max Commits/Run:  %d\n", globalCfg.Limits.MaxCommitsPerRun)
		fmt.Printf("  • Max Tasks/Run:    %d\n\n", globalCfg.Limits.MaxTasksPerRun)

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Provider ID", "Provider Type", "Model", "Used Today", "Daily Limit", "Remaining", "Status"})
		table.SetBorder(true)
		table.SetAutoWrapText(false)

		for _, modelID := range globalCfg.Routing.Models {
			prov, exists := globalCfg.Providers[modelID]
			if !exists {
				continue
			}

			used, _ := s.GetProviderDailyUsage(prov.ID, today)
			limit := prov.MaxDailyRuns
			remaining := limit - used
			if remaining < 0 {
				remaining = 0
			}

			status := color.GreenString("available")
			if !prov.Enabled {
				status = color.YellowString("disabled")
			} else if limit > 0 && used >= limit {
				status = color.RedString("exhausted")
			}

			table.Append([]string{
				prov.ID,
				prov.Provider,
				prov.Model,
				fmt.Sprintf("%d", used),
				fmt.Sprintf("%d", limit),
				fmt.Sprintf("%d", remaining),
				status,
			})
		}

		table.Render()
		fmt.Println()
	},
}
