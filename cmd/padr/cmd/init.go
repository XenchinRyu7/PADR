package cmd

import (
	"path/filepath"

	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/store"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize PADR home directory, config, and state store",
	Run: func(cmd *cobra.Command, args []string) {
		home, err := config.GetPadrHome()
		checkErr(err, "Failed to resolve PADR home")

		err = config.InitPadrHome()
		checkErr(err, "Failed to initialize directories")

		stateDir, err := config.GetStateDir()
		checkErr(err, "Failed to resolve state directory")

		dbPath := filepath.Join(stateDir, "padr.db")
		s, err := store.NewStore(dbPath)
		checkErr(err, "Failed to initialize SQLite state store")
		_ = s.Close()

		printSuccess("PADR initialized successfully at: %s", home)
		printInfo("Default config: %s", filepath.Join(home, "config", config.GlobalConfigFileName))
		printInfo("SQLite state: %s", dbPath)
	},
}
