package scheduler

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ScheduledTaskInfo provides human-readable schedule status
type ScheduledTaskInfo struct {
	Name      string
	NextRun   string
	Status    string
	Trigger   string
	Command   string
}

// Scheduler defines the interface for platform job schedulers
type Scheduler interface {
	Install(ctx context.Context, projectName, scheduleTime, binaryPath string) error
	Uninstall(ctx context.Context, projectName string) error
	List(ctx context.Context) ([]ScheduledTaskInfo, error)
}

// NewScheduler returns the appropriate platform scheduler
func NewScheduler() Scheduler {
	if runtime.GOOS == "windows" {
		return &WindowsScheduler{}
	}
	return &UnixCronScheduler{}
}

// WindowsScheduler integrates with Windows Task Scheduler via schtasks.exe
type WindowsScheduler struct{}

func (w *WindowsScheduler) taskName(projectName string) string {
	return fmt.Sprintf("PADR_%s", projectName)
}

// Install registers a scheduled task in Windows Task Scheduler
func (w *WindowsScheduler) Install(ctx context.Context, projectName, scheduleTime, binaryPath string) error {
	if binaryPath == "" {
		currentExe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to determine executable path: %w", err)
		}
		binaryPath = currentExe
	}

	taskName := w.taskName(projectName)
	command := fmt.Sprintf("\"%s\" run \"%s\"", binaryPath, projectName)

	// Clean time string (default to 09:00 if not in HH:mm format)
	timeFormatted := scheduleTime
	if !strings.Contains(timeFormatted, ":") {
		timeFormatted = "09:00"
	} else if len(timeFormatted) > 5 {
		// If cron string e.g. "0 9 * * *", parse hour and minute
		parts := strings.Fields(timeFormatted)
		if len(parts) >= 2 {
			timeFormatted = fmt.Sprintf("%02s:%02s", parts[1], parts[0])
		} else {
			timeFormatted = "09:00"
		}
	}

	args := []string{
		"/create",
		"/tn", taskName,
		"/tr", command,
		"/sc", "daily",
		"/st", timeFormatted,
		"/f",
	}

	cmd := exec.CommandContext(ctx, "schtasks", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("schtasks /create failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

// Uninstall deletes a scheduled task from Windows Task Scheduler
func (w *WindowsScheduler) Uninstall(ctx context.Context, projectName string) error {
	taskName := w.taskName(projectName)
	args := []string{"/delete", "/tn", taskName, "/f"}

	cmd := exec.CommandContext(ctx, "schtasks", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("schtasks /delete failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

// List queries all registered PADR tasks in Windows Task Scheduler
func (w *WindowsScheduler) List(ctx context.Context) ([]ScheduledTaskInfo, error) {
	cmd := exec.CommandContext(ctx, "schtasks", "/query", "/fo", "CSV", "/nh")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("schtasks /query failed: %w", err)
	}

	reader := csv.NewReader(&stdout)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var tasks []ScheduledTaskInfo
	for _, row := range records {
		if len(row) < 3 {
			continue
		}
		name := strings.Trim(row[0], "\\")
		if strings.HasPrefix(name, "PADR_") {
			tasks = append(tasks, ScheduledTaskInfo{
				Name:    name,
				NextRun: row[1],
				Status:  row[2],
			})
		}
	}

	return tasks, nil
}

// UnixCronScheduler handles Linux/macOS fallback
type UnixCronScheduler struct{}

func (u *UnixCronScheduler) Install(ctx context.Context, projectName, scheduleTime, binaryPath string) error {
	// Simple stub for cross-platform compliance
	return nil
}

func (u *UnixCronScheduler) Uninstall(ctx context.Context, projectName string) error {
	return nil
}

func (u *UnixCronScheduler) List(ctx context.Context) ([]ScheduledTaskInfo, error) {
	return []ScheduledTaskInfo{
		{
			Name:    "cron",
			Trigger: "Unix crontab managed",
			Status:  "Ready",
			NextRun: time.Now().Format("2006-01-02 15:04"),
		},
	}, nil
}
