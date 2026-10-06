package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/padr-runner/padr/pkg/config"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List configured LLM models and fallback routing hierarchy",
	Run: func(cmd *cobra.Command, args []string) {
		globalCfg, err := config.LoadGlobalConfig()
		checkErr(err, "Failed to load global config")

		color.Cyan("\nModel Routing Strategy: %s\n", globalCfg.Routing.Strategy)

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Priority", "ID", "Provider", "Model", "Env Key", "Daily Limit", "Status"})
		table.SetBorder(true)
		table.SetAutoWrapText(false)

		for i, modelID := range globalCfg.Routing.Models {
			prov, exists := globalCfg.Providers[modelID]
			if !exists {
				table.Append([]string{
					fmt.Sprintf("#%d", i+1),
					modelID,
					"-",
					"-",
					"-",
					"-",
					color.RedString("missing"),
				})
				continue
			}

			statusStr := color.GreenString("enabled")
			if !prov.Enabled {
				statusStr = color.YellowString("disabled")
			}

			keyStatus := prov.KeyStatus()

			table.Append([]string{
				fmt.Sprintf("#%d", i+1),
				prov.ID,
				prov.Provider,
				prov.Model,
				keyStatus,
				fmt.Sprintf("%d runs/day", prov.MaxDailyRuns),
				statusStr,
			})
		}

		table.Render()
		fmt.Println()
	},
}
