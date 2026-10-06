package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/padr-runner/padr/pkg/config"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// syncOpenCodeProvider registers or updates provider in ~/.config/opencode/opencode.json
func syncOpenCodeProvider(prov config.ProviderConfig, exePath string) (string, error) {
	pType := strings.ToLower(strings.TrimSpace(prov.Provider))
	var cleanID string
	var baseURL string
	var envKeys []string

	if pType == "google" || pType == "gemini" {
		cleanID = "google"
		envKeys = []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}
	} else if pType == "custom" || prov.Endpoint != "" {
		provKey := strings.ToLower(strings.TrimSpace(prov.ID))
		var cleanKey strings.Builder
		for _, r := range provKey {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				cleanKey.WriteRune(r)
			} else if r == ' ' {
				cleanKey.WriteRune('-')
			}
		}
		cleanID = cleanKey.String()
		if cleanID == "" {
			cleanID = "custom"
		}
		ep := strings.TrimSpace(prov.Endpoint)
		ep = strings.TrimRight(ep, "/")
		ep = strings.TrimSuffix(ep, "/chat/completions")
		ep = strings.TrimSuffix(ep, "/completions")
		baseURL = ep
		envKey := strings.ToUpper(strings.ReplaceAll(cleanID, "-", "_")) + "_API_KEY"
		envKeys = []string{envKey}
	} else {
		return pType, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return cleanID, err
	}

	configDir := filepath.Join(homeDir, ".config", "opencode")
	_ = os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "opencode.json")

	var cfg map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}
	if cfg["$schema"] == nil {
		cfg["$schema"] = "https://opencode.ai/config.json"
	}

	var providers map[string]interface{}
	if rawProviders, ok := cfg["providers"].(map[string]interface{}); ok {
		providers = rawProviders
	} else {
		providers = make(map[string]interface{})
	}

	provEntry := map[string]interface{}{
		"name": prov.ID,
		"env":  envKeys,
	}
	if pType == "google" || pType == "gemini" {
		provEntry["package"] = "@ai-sdk/google"
	} else {
		provEntry["package"] = "@ai-sdk/openai-compatible"
		provEntry["settings"] = map[string]interface{}{
			"baseURL": baseURL,
		}
	}
	providers[cleanID] = provEntry
	cfg["providers"] = providers

	marshaled, err := json.MarshalIndent(cfg, "", "  ")
	if err == nil {
		_ = os.WriteFile(configPath, marshaled, 0644)
	}

	// Trigger quick reload so opencode background daemon catches the new provider
	if exePath != "" {
		_ = exec.Command(exePath, "reload").Run()
	}

	return cleanID, nil
}

type progressWriter struct {
	buf        *bytes.Buffer
	onProgress func(string)
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.buf.Write(p)
	if pw.onProgress != nil && len(p) > 0 {
		pw.onProgress(string(p))
	}
	return n, err
}

// OpenCodeEngine drives the OpenCode CLI
type OpenCodeEngine struct {
	ExecutablePath string
}

// NewOpenCodeEngine creates an OpenCode adapter
func NewOpenCodeEngine(binPath string) *OpenCodeEngine {
	if binPath == "" {
		binPath = "opencode"
	}
	return &OpenCodeEngine{ExecutablePath: binPath}
}

func (e *OpenCodeEngine) Name() string {
	return "opencode"
}

// Run executes autonomous task session through OpenCode CLI
func (e *OpenCodeEngine) Run(ctx context.Context, req AgentRunRequest) (*AgentRunResult, error) {
	prompt := BuildAutonomousPrompt(req)
	startTime := time.Now()

	// Check if opencode executable exists
	exePath, err := exec.LookPath(e.ExecutablePath)
	if err != nil {
		return &AgentRunResult{
			Success:         false,
			DurationSeconds: int(time.Since(startTime).Seconds()),
			Error:           fmt.Sprintf("OpenCode CLI executable '%s' not found in PATH: %v", e.ExecutablePath, err),
		}, nil
	}

	// Prepare arguments for non-interactive autonomous run
	args := []string{"run", "--auto"}
	var customEnvKey string
	if req.Provider.Model != "" {
		modelArg := strings.TrimSpace(req.Provider.Model)
		pType := strings.ToLower(strings.TrimSpace(req.Provider.Provider))

		switch pType {
		case "google", "gemini":
			pType = "google"
			_, _ = syncOpenCodeProvider(req.Provider, exePath)
			modelArg = strings.TrimPrefix(modelArg, "google/")
		case "custom":
			cleanID, _ := syncOpenCodeProvider(req.Provider, exePath)
			pType = cleanID
			customEnvKey = strings.ToUpper(strings.ReplaceAll(cleanID, "-", "_")) + "_API_KEY"
			// Strip any leftover openai prefix
			modelArg = strings.TrimPrefix(modelArg, "openai/")
			modelArg = strings.TrimPrefix(modelArg, "openai\\")
		}

		if pType != "" {
			prefix := pType + "/"
			if !strings.HasPrefix(strings.ToLower(modelArg), prefix) {
				modelArg = prefix + modelArg
			}
		}
		args = append(args, "--model", modelArg)
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, exePath, args...)
	cmd.Dir = req.RepoDir

	// Setup environment variables for provider without cross-polluting keys
	var env []string
	for _, e := range os.Environ() {
		// Strip any ambient provider keys that might override our target provider
		if !strings.HasPrefix(e, "OPENAI_API_KEY=") &&
			!strings.HasPrefix(e, "GROQ_API_KEY=") &&
			!strings.HasPrefix(e, "GEMINI_API_KEY=") &&
			!strings.HasPrefix(e, "GOOGLE_API_KEY=") &&
			!strings.HasPrefix(e, "OPENROUTER_API_KEY=") {
			env = append(env, e)
		}
	}

	apiKey := req.Provider.ResolveAPIKey()
	if apiKey != "" {
		if customEnvKey != "" {
			env = append(env, fmt.Sprintf("%s=%s", customEnvKey, apiKey))
		}
		if req.Provider.APIKeyEnv != "" && config.IsEnvVarName(req.Provider.APIKeyEnv) {
			env = append(env, fmt.Sprintf("%s=%s", req.Provider.APIKeyEnv, apiKey))
		}
		switch strings.ToLower(req.Provider.Provider) {
		case "groq":
			env = append(env, fmt.Sprintf("GROQ_API_KEY=%s", apiKey))
		case "google", "gemini":
			env = append(env, fmt.Sprintf("GEMINI_API_KEY=%s", apiKey))
			env = append(env, fmt.Sprintf("GOOGLE_API_KEY=%s", apiKey))
			env = append(env, fmt.Sprintf("GOOGLE_GENERATIVE_AI_API_KEY=%s", apiKey))
		case "openrouter":
			env = append(env, fmt.Sprintf("OPENROUTER_API_KEY=%s", apiKey))
		case "openai":
			env = append(env, fmt.Sprintf("OPENAI_API_KEY=%s", apiKey))
		}
	}
	if req.Provider.Endpoint != "" {
		ep := strings.TrimSpace(req.Provider.Endpoint)
		ep = strings.TrimRight(ep, "/")
		ep = strings.TrimSuffix(ep, "/chat/completions")
		ep = strings.TrimSuffix(ep, "/completions")
		env = append(env, fmt.Sprintf("OLLAMA_HOST=%s", ep))
		env = append(env, fmt.Sprintf("OPENAI_BASE_URL=%s", ep))
		env = append(env, fmt.Sprintf("OPENAI_API_BASE=%s", ep))
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	var outWriter io.Writer = &stdout
	var errWriter io.Writer = &stderr

	if req.OnProgress != nil {
		outWriter = &progressWriter{buf: &stdout, onProgress: req.OnProgress}
		errWriter = &progressWriter{buf: &stderr, onProgress: req.OnProgress}
	}

	cmd.Stdout = outWriter
	cmd.Stderr = errWriter

	runErr := cmd.Run()
	duration := int(time.Since(startTime).Seconds())
	output := stripANSI(stdout.String())
	cleanStderr := strings.TrimSpace(stripANSI(stderr.String()))
	if cleanStderr != "" {
		output += "\n[STDERR]\n" + cleanStderr
	}

	if runErr != nil {
		errMsg := fmt.Sprintf("opencode run failed: %v", runErr)
		if cleanStderr != "" {
			errMsg = fmt.Sprintf("opencode error: %s", cleanStderr)
		}
		return &AgentRunResult{
			Success:         false,
			Output:          output,
			DurationSeconds: duration,
			Error:           errMsg,
		}, nil
	}

	return &AgentRunResult{
		Success:         true,
		Output:          output,
		TasksCompleted:  1,
		DurationSeconds: duration,
	}, nil
}
