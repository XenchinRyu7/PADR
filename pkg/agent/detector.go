package agent

import (
	"context"
	"os/exec"
	"strings"
)

// EngineStatus reports detection and availability of agent CLI tools
type EngineStatus struct {
	Name        string
	Installed   bool
	Path        string
	Version     string
	InstallHelp string
}

// DetectEngine checks if the requested coding agent CLI is installed on this machine
func DetectEngine(ctx context.Context, engineName, customPath string) EngineStatus {
	exe := customPath
	if exe == "" {
		switch strings.ToLower(engineName) {
		case "cline":
			exe = "cline"
		case "aider":
			exe = "aider"
		case "mock":
			return EngineStatus{
				Name:        "Mock Engine",
				Installed:   true,
				Version:     "Built-in simulation",
				InstallHelp: "Ready to use without external dependencies.",
			}
		default:
			exe = "opencode"
		}
	}

	status := EngineStatus{
		Name: strings.Title(engineName),
	}

	lookPath, err := exec.LookPath(exe)
	if err != nil {
		status.Installed = false
		switch strings.ToLower(engineName) {
		case "opencode", "":
			status.InstallHelp = "Install with: npm install -g opencode-ai  (or visit https://opencode.ai)"
		case "cline":
			status.InstallHelp = "Install with: npm install -g cline  (or visit https://github.com/cline/cline)"
		case "aider":
			status.InstallHelp = "Install with: pip install aider-chat  (or visit https://aider.chat)"
		}
		return status
	}

	status.Installed = true
	status.Path = lookPath

	// Try checking version
	verCmd := exec.CommandContext(ctx, lookPath, "--version")
	if out, err := verCmd.Output(); err == nil {
		status.Version = strings.TrimSpace(string(out))
	} else {
		status.Version = "Detected in PATH"
	}

	return status
}
