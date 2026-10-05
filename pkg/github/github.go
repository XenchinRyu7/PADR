package github

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
)

// AccountInfo holds detected GitHub credentials and git configuration
type AccountInfo struct {
	Username string
	Email    string
	LoggedIn bool
	Method   string // e.g. "gh cli (keyring)" or "git config"
	Scopes   string
}

// DetectAccount checks local GitHub CLI and Git config for connected accounts
func DetectAccount(ctx context.Context) *AccountInfo {
	info := &AccountInfo{
		LoggedIn: false,
		Method:   "not connected",
	}

	// 1. Try reading global git user info
	gitNameCmd := exec.CommandContext(ctx, "git", "config", "--global", "user.name")
	if out, err := gitNameCmd.Output(); err == nil {
		info.Username = strings.TrimSpace(string(out))
	}

	gitEmailCmd := exec.CommandContext(ctx, "git", "config", "--global", "user.email")
	if out, err := gitEmailCmd.Output(); err == nil {
		info.Email = strings.TrimSpace(string(out))
	}

	// 2. Try querying gh CLI for richer account status
	ghCmd := exec.CommandContext(ctx, "gh", "auth", "status")
	var stdout, stderr bytes.Buffer
	ghCmd.Stdout = &stdout
	ghCmd.Stderr = &stderr

	_ = ghCmd.Run()
	output := stdout.String() + "\n" + stderr.String()

	if strings.Contains(output, "Logged in to github.com account") {
		info.LoggedIn = true
		info.Method = "GitHub CLI (Windows Keyring)"

		// Parse account name if available
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "account ") {
				parts := strings.Split(line, "account ")
				if len(parts) > 1 {
					userPart := strings.Fields(parts[1])
					if len(userPart) > 0 {
						info.Username = userPart[0]
					}
				}
			}
			if strings.HasPrefix(line, "- Token scopes:") {
				info.Scopes = strings.TrimPrefix(line, "- Token scopes:")
			}
		}
	} else if info.Username != "" || info.Email != "" {
		info.LoggedIn = true
		info.Method = "Git Global Configuration"
	}

	return info
}
