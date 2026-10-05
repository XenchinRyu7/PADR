package cmd

import (
	"github.com/padr-runner/padr/pkg/gui"
	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:     "gui",
	Aliases: []string{"tray", "dashboard"},
	Short:   "Launch the native desktop dashboard and system tray manager",
	Run: func(cmd *cobra.Command, args []string) {
		gui.RunDashboard()
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
