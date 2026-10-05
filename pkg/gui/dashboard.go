package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/github"
	"github.com/padr-runner/padr/pkg/runner"
	"github.com/padr-runner/padr/pkg/scheduler"
	"github.com/padr-runner/padr/pkg/store"
)

// RunDashboard launches the native desktop dashboard with system tray support
func RunDashboard() {
	a := app.NewWithID("com.padr.runner")
	a.Settings().SetTheme(theme.DarkTheme())

	icon := GetAppIcon()
	a.SetIcon(icon)

	w := a.NewWindow("PADR — Autonomous Development Runner")
	w.Resize(fyne.NewSize(920, 640))
	w.SetIcon(icon)

	// Ensure home and store are loaded
	_ = config.InitPadrHome()
	globalCfg, _ := config.LoadGlobalConfig()

	stateDir, _ := config.GetStateDir()
	dbPath := filepath.Join(stateDir, "padr.db")
	dbStore, err := store.NewStore(dbPath)
	if err != nil {
		dialog.ShowError(err, w)
		return
	}
	defer dbStore.Close()

	orch := runner.NewOrchestrator(globalCfg, dbStore)

	// Build Tabs
	tabSchedule := buildScheduleTab(w, globalCfg, dbStore, orch)
	tabRepos := buildReposTab(w)
	tabProviders := buildProvidersTab(w, globalCfg)
	tabGitHub := buildGitHubTab(w)
	tabLogs := buildLogsTab(w, dbStore)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Schedule & Run", theme.MediaPlayIcon(), tabSchedule),
		container.NewTabItemWithIcon("Repositories", theme.FolderIcon(), tabRepos),
		container.NewTabItemWithIcon("AI Providers", theme.SettingsIcon(), tabProviders),
		container.NewTabItemWithIcon("GitHub Account", theme.AccountIcon(), tabGitHub),
		container.NewTabItemWithIcon("Run Logs", theme.DocumentIcon(), tabLogs),
	)
	tabs.SetTabLocation(container.TabLocationLeading)

	w.SetContent(tabs)

	// Setup System Tray
	if desk, ok := a.(desktop.App); ok {
		menu := fyne.NewMenu("PADR",
			fyne.NewMenuItem("Open Dashboard", func() {
				w.Show()
				w.RequestFocus()
			}),
			fyne.NewMenuItem("Run All Projects Now", func() {
				go func() {
					projects, _ := config.ListProjects()
					for _, p := range projects {
						_, _ = orch.RunProject(context.Background(), p, runner.RunOptions{})
					}
				}()
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Quit", func() {
				a.Quit()
			}),
		)
		desk.SetSystemTrayMenu(menu)
		desk.SetSystemTrayIcon(icon)
	}

	// Close intercepts -> hide to tray instead of quitting
	w.SetCloseIntercept(func() {
		w.Hide()
	})

	w.ShowAndRun()
}

// Tab 1: Schedule & Execution Runner
func buildScheduleTab(w fyne.Window, globalCfg *config.GlobalConfig, s *store.Store, orch *runner.Orchestrator) fyne.CanvasObject {
	statusTitle := widget.NewLabelWithStyle("⚡ Autonomous Runner & Scheduler", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	today := time.Now().Format("2006-01-02")
	usageCount, _ := s.GetTotalDailyUsage(today)
	usageLabel := widget.NewLabel(fmt.Sprintf("Today's Runs: %d / %d  |  Max Runtime: %d min",
		usageCount, globalCfg.Limits.MaxRunsPerDay, globalCfg.Limits.MaxRuntimeMinutes))

	// Schedule setup
	timeSelect := widget.NewSelect([]string{"08:00", "09:00", "12:00", "15:00", "18:00", "21:00", "23:00"}, nil)
	timeSelect.SetSelected("09:00")

	schedStatus := widget.NewLabel("Windows Task Scheduler: Ready")

	installBtn := widget.NewButtonWithIcon("Sync to Windows Task Scheduler", theme.ConfirmIcon(), func() {
		projects, err := config.ListProjects()
		if err != nil || len(projects) == 0 {
			dialog.ShowInformation("No Projects", "Please add at least one repository first in the Repositories tab.", w)
			return
		}

		sched := scheduler.NewScheduler()
		exePath, _ := os.Executable()
		successCount := 0

		for _, p := range projects {
			if err := sched.Install(context.Background(), p.Name, timeSelect.Selected, exePath); err == nil {
				successCount++
			}
		}

		schedStatus.SetText(fmt.Sprintf("Synced %d project(s) to Windows Task Scheduler at %s daily", successCount, timeSelect.Selected))
		dialog.ShowInformation("Schedule Synced", fmt.Sprintf("Registered daily autonomous task at %s in Windows Task Scheduler.", timeSelect.Selected), w)
	})

	// Manual Trigger
	projectSelect := widget.NewSelect([]string{"All Projects"}, nil)
	refreshProjects := func() {
		projects, _ := config.ListProjects()
		opts := []string{"All Projects"}
		for _, p := range projects {
			opts = append(opts, p.Name)
		}
		projectSelect.Options = opts
		projectSelect.SetSelected("All Projects")
	}
	refreshProjects()

	dryRunCheck := widget.NewCheck("Dry Run (Simulate with mock engine, no git push)", nil)
	dryRunCheck.SetChecked(false)

	logOutput := widget.NewMultiLineEntry()
	logOutput.SetPlaceHolder("Real-time execution log will appear here...")
	logOutput.Wrapping = fyne.TextWrapWord
	logOutput.Disable()

	runBtn := widget.NewButtonWithIcon("🚀 Run Autonomous Pipeline Now", theme.MediaPlayIcon(), func() {
		selected := projectSelect.Selected
		isDryRun := dryRunCheck.Checked
		logOutput.SetText("Starting execution pipeline...\n")

		go func() {
			opts := runner.RunOptions{DryRun: isDryRun}
			if selected == "All Projects" {
				projects, _ := config.ListProjects()
				if len(projects) == 0 {
					logOutput.SetText("No projects registered! Add a repository in the Repositories tab.")
					return
				}
				for _, p := range projects {
					rec, err := orch.RunProject(context.Background(), p, opts)
					if err != nil {
						logOutput.SetText(logOutput.Text + fmt.Sprintf("\n[%s] FAILED/SKIPPED: %v\n", p.Name, err))
					} else {
						logOutput.SetText(logOutput.Text + fmt.Sprintf("\n[%s] SUCCESS: %d commits, Provider: %s, Time: %ds\n",
							p.Name, rec.Commits, rec.Provider, rec.DurationSeconds))
					}
				}
			} else {
				p, err := config.FindProjectByName(selected)
				if err != nil {
					logOutput.SetText("Project not found: " + selected)
					return
				}
				rec, err := orch.RunProject(context.Background(), p, opts)
				if err != nil {
					logOutput.SetText(logOutput.Text + fmt.Sprintf("[%s] FAILED/SKIPPED: %v\n", p.Name, err))
				} else {
					logOutput.SetText(logOutput.Text + fmt.Sprintf("[%s] SUCCESS: %d commits, Provider: %s (%s), Duration: %ds\nDiff:\n%s\n",
						p.Name, rec.Commits, rec.Provider, rec.Model, rec.DurationSeconds, rec.DiffSummary))
				}
			}

			// Update usage count
			today := time.Now().Format("2006-01-02")
			cnt, _ := s.GetTotalDailyUsage(today)
			usageLabel.SetText(fmt.Sprintf("Today's Runs: %d / %d  |  Max Runtime: %d min",
				cnt, globalCfg.Limits.MaxRunsPerDay, globalCfg.Limits.MaxRuntimeMinutes))
		}()
	})

	logOutput.SetMinRowsVisible(8)
	scrollLog := container.NewScroll(logOutput)

	return container.NewVBox(
		statusTitle,
		usageLabel,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("⏰ Daily Execution Time (Windows Task Scheduler)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(widget.NewLabel("Run Every Day At:"), timeSelect, installBtn),
		schedStatus,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("▶ Manual Execution", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(widget.NewLabel("Target:"), projectSelect, dryRunCheck),
		runBtn,
		widget.NewLabelWithStyle("Execution Output:", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
		scrollLog,
	)
}

// Tab 2: Repositories & PADR_ROADMAP.md
func buildReposTab(w fyne.Window) fyne.CanvasObject {
	repoList := widget.NewList(
		func() int {
			projs, _ := config.ListProjects()
			return len(projs)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Project Item")
		},
		nil,
	)

	detailName := widget.NewLabelWithStyle("Select a repository", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	detailPath := widget.NewLabel("Path: -")
	detailBranch := widget.NewLabel("Branch: -")

	roadmapEditor := widget.NewMultiLineEntry()
	roadmapEditor.Wrapping = fyne.TextWrapWord
	roadmapEditor.SetPlaceHolder("Contents of PADR_ROADMAP.md will be loaded here...")

	var currentSelectedProj *config.ProjectConfig

	refreshRepoDetails := func(p *config.ProjectConfig) {
		currentSelectedProj = p
		detailName.SetText(fmt.Sprintf("Repository: %s", p.Name))
		detailPath.SetText(fmt.Sprintf("Path: %s", p.Repository.Path))
		detailBranch.SetText(fmt.Sprintf("Branch: %s", p.Repository.Branch))

		// Check PADR_ROADMAP.md or ROADMAP.md
		padrRoadmap := filepath.Join(p.Repository.Path, "PADR_ROADMAP.md")
		data, err := os.ReadFile(padrRoadmap)
		if err != nil {
			legacyRoadmap := filepath.Join(p.Repository.Path, "ROADMAP.md")
			data, err = os.ReadFile(legacyRoadmap)
		}

		if err == nil {
			roadmapEditor.SetText(string(data))
		} else {
			roadmapEditor.SetText(fmt.Sprintf("# %s — Autonomous Roadmap\n\n- [ ] Initial autonomous task\n", p.Name))
		}
	}

	repoList.UpdateItem = func(i int, o fyne.CanvasObject) {
		projs, _ := config.ListProjects()
		if i < len(projs) {
			o.(*widget.Label).SetText(fmt.Sprintf("📦 %s (%s)", projs[i].Name, projs[i].Repository.Branch))
		}
	}

	repoList.OnSelected = func(id int) {
		projs, _ := config.ListProjects()
		if id < len(projs) {
			refreshRepoDetails(projs[id])
		}
	}

	newTaskEntry := widget.NewEntry()
	newTaskEntry.SetPlaceHolder("Enter new task for PADR_ROADMAP.md...")

	addTaskBtn := widget.NewButtonWithIcon("Add Task to Roadmap", theme.ContentAddIcon(), func() {
		taskText := strings.TrimSpace(newTaskEntry.Text)
		if taskText == "" || currentSelectedProj == nil {
			return
		}

		padrRoadmap := filepath.Join(currentSelectedProj.Repository.Path, "PADR_ROADMAP.md")
		content := fmt.Sprintf("\n- [ ] %s\n", taskText)

		f, err := os.OpenFile(padrRoadmap, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = f.WriteString(content)
			_ = f.Close()
			newTaskEntry.SetText("")
			refreshRepoDetails(currentSelectedProj)
			dialog.ShowInformation("Task Added", "New task added to PADR_ROADMAP.md!", w)
		}
	})

	saveRoadmapBtn := widget.NewButtonWithIcon("Save Changes to PADR_ROADMAP.md", theme.DocumentSaveIcon(), func() {
		if currentSelectedProj == nil {
			return
		}
		padrRoadmap := filepath.Join(currentSelectedProj.Repository.Path, "PADR_ROADMAP.md")
		err := os.WriteFile(padrRoadmap, []byte(roadmapEditor.Text), 0644)
		if err != nil {
			dialog.ShowError(err, w)
		} else {
			dialog.ShowInformation("Saved", "PADR_ROADMAP.md updated successfully!", w)
		}
	})

	// Add Project Dialog Button
	addRepoBtn := widget.NewButtonWithIcon("➕ Register New Repository", theme.FolderNewIcon(), func() {
		nameEntry := widget.NewEntry()
		nameEntry.SetPlaceHolder("project-name (e.g. food-erp)")

		pathEntry := widget.NewEntry()
		pathEntry.SetPlaceHolder("C:/dev/your-repo")

		branchEntry := widget.NewEntry()
		branchEntry.SetText("main")

		form := dialog.NewForm("Register Repository for PADR", "Register", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Project Name", nameEntry),
			widget.NewFormItem("Folder Path", pathEntry),
			widget.NewFormItem("Branch", branchEntry),
		}, func(ok bool) {
			if !ok || nameEntry.Text == "" || pathEntry.Text == "" {
				return
			}
			absPath, _ := filepath.Abs(pathEntry.Text)
			proj := config.DefaultProjectConfig(nameEntry.Text, absPath)
			proj.Repository.Branch = branchEntry.Text

			_ = config.SaveProjectConfig(absPath, proj)
			_ = config.RegisterProject(proj)
			repoList.Refresh()
			dialog.ShowInformation("Success", fmt.Sprintf("Project '%s' registered!", proj.Name), w)
		}, w)
		form.Resize(fyne.NewSize(500, 300))
		form.Show()
	})

	leftPane := container.NewBorder(addRepoBtn, nil, nil, nil, repoList)

	rightPane := container.NewVBox(
		detailName,
		detailPath,
		detailBranch,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("📋 PADR_ROADMAP.md (Autonomous Plan)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(newTaskEntry, addTaskBtn),
		container.NewScroll(roadmapEditor),
		saveRoadmapBtn,
	)

	return container.NewHSplit(leftPane, rightPane)
}

// Tab 3: Providers & Fallback Models
func buildProvidersTab(w fyne.Window, cfg *config.GlobalConfig) fyne.CanvasObject {
	header := widget.NewLabelWithStyle("🤖 AI Model Providers & Fallback Hierarchy", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	chainLabel := widget.NewLabel(fmt.Sprintf("Active Fallback Chain: %s", strings.Join(cfg.Routing.Models, " ➔ ")))

	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("provider-id (e.g. groq-fast, gemini-flash, ollama-local)")

	typeSelect := widget.NewSelect([]string{"groq", "google", "openrouter", "ollama", "custom"}, nil)
	typeSelect.SetSelected("groq")

	modelEntry := widget.NewEntry()
	modelEntry.SetPlaceHolder("Model Identifier (e.g. openai/gpt-oss-120b or gemini-2.5-flash)")

	keyEnvEntry := widget.NewEntry()
	keyEnvEntry.SetPlaceHolder("API Key Env Variable (e.g. GROQ_API_KEY)")

	endpointEntry := widget.NewEntry()
	endpointEntry.SetPlaceHolder("Custom Endpoint URL (e.g. http://localhost:11434 for Ollama)")

	limitEntry := widget.NewEntry()
	limitEntry.SetText("5")

	providerList := widget.NewMultiLineEntry()
	providerList.Disable()

	refreshProvidersList := func() {
		var sb strings.Builder
		sb.WriteString("Configured Providers:\n")
		for _, mID := range cfg.Routing.Models {
			p, ok := cfg.Providers[mID]
			if !ok {
				continue
			}
			status := "✓ Active"
			if !p.Enabled {
				status = "✗ Disabled"
			}
			sb.WriteString(fmt.Sprintf("• [%s] Type: %s | Model: %s | Limit: %d/day | %s\n",
				p.ID, p.Provider, p.Model, p.MaxDailyRuns, status))
		}
		providerList.SetText(sb.String())
		chainLabel.SetText(fmt.Sprintf("Active Fallback Chain: %s", strings.Join(cfg.Routing.Models, " ➔ ")))
	}
	refreshProvidersList()

	saveBtn := widget.NewButtonWithIcon("Save / Add Provider", theme.DocumentSaveIcon(), func() {
		if idEntry.Text == "" || modelEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("ID and Model name are required"), w)
			return
		}

		limit := 5
		_, _ = fmt.Sscanf(limitEntry.Text, "%d", &limit)

		if cfg.Providers == nil {
			cfg.Providers = make(map[string]config.ProviderConfig)
		}

		cfg.Providers[idEntry.Text] = config.ProviderConfig{
			ID:           idEntry.Text,
			Provider:     typeSelect.Selected,
			Model:        modelEntry.Text,
			APIKeyEnv:    keyEnvEntry.Text,
			Endpoint:     endpointEntry.Text,
			MaxDailyRuns: limit,
			Enabled:      true,
		}

		// Add to routing if not present
		found := false
		for _, m := range cfg.Routing.Models {
			if m == idEntry.Text {
				found = true
				break
			}
		}
		if !found {
			cfg.Routing.Models = append(cfg.Routing.Models, idEntry.Text)
		}

		_ = config.SaveGlobalConfig(cfg)
		refreshProvidersList()
		dialog.ShowInformation("Provider Saved", fmt.Sprintf("Provider '%s' is now configured and active!", idEntry.Text), w)
	})

	form := container.NewVBox(
		widget.NewLabelWithStyle("Configure Provider:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(widget.NewLabel("ID:"), idEntry),
		container.NewHBox(widget.NewLabel("Provider Type:"), typeSelect),
		container.NewHBox(widget.NewLabel("Model:"), modelEntry),
		container.NewHBox(widget.NewLabel("API Key Env:"), keyEnvEntry),
		container.NewHBox(widget.NewLabel("Endpoint:"), endpointEntry),
		container.NewHBox(widget.NewLabel("Daily Limit:"), limitEntry),
		saveBtn,
	)

	return container.NewVBox(
		header,
		chainLabel,
		widget.NewSeparator(),
		providerList,
		widget.NewSeparator(),
		form,
	)
}

// Tab 4: GitHub Account Detection
func buildGitHubTab(w fyne.Window) fyne.CanvasObject {
	header := widget.NewLabelWithStyle("👤 Connected GitHub Account", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	statusBadge := widget.NewLabel("Checking connection...")
	userLabel := widget.NewLabel("Username: -")
	emailLabel := widget.NewLabel("Email: -")
	methodLabel := widget.NewLabel("Detection Method: -")
	scopesLabel := widget.NewLabel("Token Scopes: -")

	refreshAccount := func() {
		acc := github.DetectAccount(context.Background())
		if acc.LoggedIn {
			statusBadge.SetText("🟢 Connected and Active")
			userLabel.SetText(fmt.Sprintf("Username: %s", acc.Username))
			emailLabel.SetText(fmt.Sprintf("Git Committer Email: %s", acc.Email))
			methodLabel.SetText(fmt.Sprintf("Authentication: %s", acc.Method))
			if acc.Scopes != "" {
				scopesLabel.SetText(fmt.Sprintf("Permissions: %s", acc.Scopes))
			} else {
				scopesLabel.SetText("Permissions: Default Git push/pull authorized")
			}
		} else {
			statusBadge.SetText("🔴 Not Connected")
			userLabel.SetText("No GitHub account or Git user configured.")
			emailLabel.SetText("Run 'git config --global user.name' or 'gh auth login'.")
		}
	}
	refreshAccount()

	refreshBtn := widget.NewButtonWithIcon("Refresh Connection", theme.ViewRefreshIcon(), refreshAccount)

	infoCard := widget.NewLabel(
		"PADR uses your native Windows credential store and git configuration.\n" +
			"All commits and PRs generated during autonomous runs will be credited directly\n" +
			"to your personal GitHub profile and count toward your contribution graph.")

	return container.NewVBox(
		header,
		widget.NewSeparator(),
		statusBadge,
		userLabel,
		emailLabel,
		methodLabel,
		scopesLabel,
		widget.NewSeparator(),
		infoCard,
		refreshBtn,
	)
}

// Tab 5: Execution Logs
func buildLogsTab(w fyne.Window, s *store.Store) fyne.CanvasObject {
	header := widget.NewLabelWithStyle("📜 Recent Autonomous Run Logs", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	logDisplay := widget.NewMultiLineEntry()
	logDisplay.Wrapping = fyne.TextWrapWord
	logDisplay.Disable()

	refreshLogs := func() {
		runs, err := s.ListRuns("", 30)
		if err != nil || len(runs) == 0 {
			logDisplay.SetText("No execution records found yet.")
			return
		}

		var sb strings.Builder
		for _, r := range runs {
			icon := "✓"
			if r.Status == "failed" {
				icon = "✗"
			} else if r.Status == "skipped" {
				icon = "⏭"
			}

			sb.WriteString(fmt.Sprintf("[%s] %s Project: %s | Model: %s (%s) | Commits: %d | Time: %ds\n",
				r.StartedAt.Format("2006-01-02 15:04:05"),
				icon,
				r.Project,
				r.Provider,
				r.Model,
				r.Commits,
				r.DurationSeconds,
			))
			if r.Error != "" {
				sb.WriteString(fmt.Sprintf("   Error/Note: %s\n", r.Error))
			}
			if r.DiffSummary != "" {
				sb.WriteString(fmt.Sprintf("   Changes: %s\n", strings.TrimSpace(r.DiffSummary)))
			}
			sb.WriteString("\n")
		}
		logDisplay.SetText(sb.String())
	}
	refreshLogs()

	refreshBtn := widget.NewButtonWithIcon("Refresh Logs", theme.ViewRefreshIcon(), refreshLogs)

	return container.NewBorder(
		header,
		refreshBtn,
		nil,
		nil,
		container.NewScroll(logDisplay),
	)
}
