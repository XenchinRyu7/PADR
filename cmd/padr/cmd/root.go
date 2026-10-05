package cmd

import (
	"os"

	"github.com/fatih/color"
	"github.com/padr-runner/padr/pkg/gui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "padr",
	Short: "PADR — Personal Autonomous Development Runner",
	Long: color.CyanString(`
  ██████╗  █████╗ ██████╗ ██████╗ 
  ██╔══██╗██╔══██╗██╔══██╗██╔══██╗
  ██████╔╝███████║██║  ██║██████╔╝
  ██╔═══╝ ██╔══██║██║  ██║██╔══██╗
  ██║     ██║  ██║██████╔╝██║  ██║
  ╚═╝     ╚═╝  ╚═╝╚═════╝ ╚═╝  ╚═╝
  Personal Autonomous Development Runner
`) + `
Runs autonomous development tasks on local repositories using AI coding agents
with model provider swapping, Git safety guards, and task scheduling.`,
	Run: func(cmd *cobra.Command, args []string) {
		gui.RunDashboard()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(quotaCmd)
	rootCmd.AddCommand(providerCmd)
	rootCmd.AddCommand(scheduleCmd)
}

func checkErr(err error, message string) {
	if err != nil {
		color.Red("✗ %s: %v", message, err)
		os.Exit(1)
	}
}

func printSuccess(format string, a ...interface{}) {
	color.Green("✓ "+format, a...)
}

func printInfo(format string, a ...interface{}) {
	color.Cyan("ℹ "+format, a...)
}

func printWarn(format string, a ...interface{}) {
	color.Yellow("⚠ "+format, a...)
}
