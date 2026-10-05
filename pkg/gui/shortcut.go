package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CreateDesktopShortcut creates a Windows Desktop .lnk shortcut for PADR
func CreateDesktopShortcut() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	desktopDir := filepath.Join(homeDir, "Desktop")
	shortcutPath := filepath.Join(desktopDir, "PADR Autonomous Runner.lnk")

	script := fmt.Sprintf(`
$ws = New-Object -ComObject WScript.Shell
$s = $ws.CreateShortcut('%s')
$s.TargetPath = '%s'
$s.Arguments = 'gui'
$s.WorkingDirectory = '%s'
$s.IconLocation = '%s,0'
$s.Description = 'PADR — Personal Autonomous Development Runner'
$s.Save()
`, strings.ReplaceAll(shortcutPath, "'", "''"),
		strings.ReplaceAll(exePath, "'", "''"),
		strings.ReplaceAll(filepath.Dir(exePath), "'", "''"),
		strings.ReplaceAll(exePath, "'", "''"))

	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	return cmd.Run()
}

// CreateStartupShortcut registers PADR in Windows Startup folder for automatic tray launch
func CreateStartupShortcut() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	appData := os.Getenv("APPDATA")
	if appData == "" {
		return fmt.Errorf("APPDATA environment variable not set")
	}

	startupDir := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	shortcutPath := filepath.Join(startupDir, "PADR.lnk")

	script := fmt.Sprintf(`
$ws = New-Object -ComObject WScript.Shell
$s = $ws.CreateShortcut('%s')
$s.TargetPath = '%s'
$s.Arguments = 'gui'
$s.WorkingDirectory = '%s'
$s.IconLocation = '%s,0'
$s.Description = 'PADR — Personal Autonomous Development Runner'
$s.Save()
`, strings.ReplaceAll(shortcutPath, "'", "''"),
		strings.ReplaceAll(exePath, "'", "''"),
		strings.ReplaceAll(filepath.Dir(exePath), "'", "''"),
		strings.ReplaceAll(exePath, "'", "''"))

	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	return cmd.Run()
}
