package cmd

import (
	"fmt"

	"github.com/padr-runner/padr/pkg/config"
	"github.com/spf13/cobra"
)

var (
	provType       string
	provModel      string
	provKeyEnv     string
	provDailyLimit int
	provEndpoint   string
)

var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "Configure AI model providers and budget limits",
}

var providerAddCmd = &cobra.Command{
	Use:   "add <id>",
	Short: "Add or update a provider in PADR configuration",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]

		globalCfg, err := config.LoadGlobalConfig()
		checkErr(err, "Failed to load global config")

		if provType == "" {
			provType = id
		}
		if provModel == "" {
			checkErr(fmt.Errorf("model is required"), "Specify --model")
		}

		if globalCfg.Providers == nil {
			globalCfg.Providers = make(map[string]config.ProviderConfig)
		}

		globalCfg.Providers[id] = config.ProviderConfig{
			ID:           id,
			Provider:     provType,
			Model:        provModel,
			APIKeyEnv:    provKeyEnv,
			Endpoint:     provEndpoint,
			MaxDailyRuns: provDailyLimit,
			Enabled:      true,
		}

		// Ensure it's in the routing list if not already
		found := false
		for _, m := range globalCfg.Routing.Models {
			if m == id {
				found = true
				break
			}
		}
		if !found {
			globalCfg.Routing.Models = append(globalCfg.Routing.Models, id)
		}

		err = config.SaveGlobalConfig(globalCfg)
		checkErr(err, "Failed to save updated config")

		printSuccess("Provider '%s' (%s, model: %s) configured successfully!", id, provType, provModel)
	},
}

func init() {
	providerAddCmd.Flags().StringVarP(&provType, "type", "t", "groq", "Provider type (groq, google, openrouter, ollama)")
	providerAddCmd.Flags().StringVarP(&provModel, "model", "m", "", "Model name (e.g. openai/gpt-oss-120b)")
	providerAddCmd.Flags().StringVar(&provKeyEnv, "key-env", "", "Environment variable name containing the API key")
	providerAddCmd.Flags().IntVar(&provDailyLimit, "daily-limit", 5, "Max runs allowed per day for this provider")
	providerAddCmd.Flags().StringVar(&provEndpoint, "endpoint", "", "Custom endpoint (for ollama or custom API)")

	providerCmd.AddCommand(providerAddCmd)
}
