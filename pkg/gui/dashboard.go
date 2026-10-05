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
	a.Settings().SetTheme(&PadrTheme{})

	icon := GetAppIcon()
	a.SetIcon(icon)

	w := a.NewWindow("PADR — Autonomous Development Runner")
	w.Resize(fyne.NewSize(1000, 700))
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
		container.NewTabItemWithIcon("Runner", theme.MediaPlayIcon(), tabSchedule),
		container.NewTabItemWithIcon("Repositories", theme.FolderIcon(), tabRepos),
		container.NewTabItemWithIcon("Providers", theme.SettingsIcon(), tabProviders),
		container.NewTabItemWithIcon("Account", theme.AccountIcon(), tabGitHub),
		container.NewTabItemWithIcon("Logs", theme.DocumentIcon(), tabLogs),
	)
	tabs.SetTabLocation(container.TabLocationLeading)

	w.SetContent(tabs)

	// Setup System Tray with clean, professional menu
	if desk, ok := a.(desktop.App); ok {
		menu := fyne.NewMenu("PADR",
			fyne.NewMenuItem("Open Dashboard", func() {
				w.Show()
				w.RequestFocus()
			}),
			fyne.NewMenuItem("Run Autonomous Tasks", func() {
				go func() {
					projects, _ := config.ListProjects()
					for _, p := range projects {
						_, _ = orch.RunProject(context.Background(), p, runner.RunOptions{})
					}
				}()
			}),
			fyne.NewMenuItem("Settings & Providers", func() {
				w.Show()
				w.RequestFocus()
				tabs.SelectIndex(2)
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Setup Wizard...", func() {
				ShowSetupWizard(w, func() {
					w.Show()
				})
			}),
			fyne.NewMenuItem("Create Desktop Shortcut", func() {
				if err := CreateDesktopShortcut(); err == nil {
					dialog.ShowInformation("Shortcut Created", "PADR shortcut placed on your Desktop.", w)
				}
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Quit PADR", func() {
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

	// If no projects registered yet, launch the Onboarding Wizard automatically
	existingProjs, _ := config.ListProjects()
	if len(existingProjs) == 0 {
		ShowSetupWizard(w, func() {
			w.Show()
		})
	}

	w.ShowAndRun()
}

// Tab 1: Schedule & Execution Runner
func buildScheduleTab(w fyne.Window, globalCfg *config.GlobalConfig, s *store.Store, orch *runner.Orchestrator) fyne.CanvasObject {
	header := widget.NewLabelWithStyle("Autonomous Runner & Scheduler", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	today := time.Now().Format("2006-01-02")
	usageCount, _ := s.GetTotalDailyUsage(today)
	usageLabel := widget.NewLabel(fmt.Sprintf("Today's Runs: %d / %d  |  Max Runtime Window: %d minutes  |  Status: Ready",
		usageCount, globalCfg.Limits.MaxRunsPerDay, globalCfg.Limits.MaxRuntimeMinutes))

	// Schedule setup
	timeSelect := widget.NewSelect([]string{"08:00", "09:00", "12:00", "15:00", "18:00", "21:00", "23:00"}, nil)
	timeSelect.SetSelected("09:00")

	schedStatus := widget.NewLabel("Scheduler: Windows Task Scheduler active")

	installBtn := widget.NewButtonWithIcon("Sync to Windows Task Scheduler", theme.ConfirmIcon(), func() {
		projects, err := config.ListProjects()
		if err != nil || len(projects) == 0 {
			dialog.ShowInformation("No Projects", "Register at least one repository first.", w)
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

		schedStatus.SetText(fmt.Sprintf("Active: Synced %d project(s) to run daily at %s", successCount, timeSelect.Selected))
		dialog.ShowInformation("Schedule Synchronized", fmt.Sprintf("Registered daily autonomous task at %s in Windows Task Scheduler.", timeSelect.Selected), w)
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

	dryRunCheck := widget.NewCheck("Dry Run (Simulate changes without pushing to git remote)", nil)
	dryRunCheck.SetChecked(false)

	logOutput := widget.NewMultiLineEntry()
	logOutput.SetPlaceHolder("Session execution output will appear here...")
	logOutput.Wrapping = fyne.TextWrapWord
	logOutput.SetMinRowsVisible(10)

	runBtn := widget.NewButtonWithIcon("Run Autonomous Pipeline Now", theme.MediaPlayIcon(), func() {
		selected := projectSelect.Selected
		isDryRun := dryRunCheck.Checked
		logOutput.SetText("Starting autonomous execution session...\n")

		go func() {
			opts := runner.RunOptions{DryRun: isDryRun}
			if selected == "All Projects" {
				projects, _ := config.ListProjects()
				if len(projects) == 0 {
					logOutput.SetText("No projects registered. Please add a repository first.")
					return
				}
				for _, p := range projects {
					rec, err := orch.RunProject(context.Background(), p, opts)
					if err != nil {
						logOutput.SetText(logOutput.Text + fmt.Sprintf("\n[%s] HALTED: %v\n", p.Name, err))
					} else {
						logOutput.SetText(logOutput.Text + fmt.Sprintf("\n[%s] SUCCESS: %d commit(s) created | Model: %s | Duration: %ds\n",
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
					logOutput.SetText(logOutput.Text + fmt.Sprintf("[%s] HALTED: %v\n", p.Name, err))
				} else {
					logOutput.SetText(logOutput.Text + fmt.Sprintf("[%s] SUCCESS: %d commit(s) | Model: %s (%s) | Duration: %ds\nDiff summary:\n%s\n",
						p.Name, rec.Commits, rec.Provider, rec.Model, rec.DurationSeconds, rec.DiffSummary))
				}
			}

			// Update usage count
			today := time.Now().Format("2006-01-02")
			cnt, _ := s.GetTotalDailyUsage(today)
			usageLabel.SetText(fmt.Sprintf("Today's Runs: %d / %d  |  Max Runtime Window: %d minutes  |  Status: Ready",
				cnt, globalCfg.Limits.MaxRunsPerDay, globalCfg.Limits.MaxRuntimeMinutes))
		}()
	})

	scrollLog := container.NewScroll(logOutput)

	scheduleForm := container.NewVBox(
		widget.NewLabelWithStyle("Daily Schedule Trigger", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, widget.NewLabel("Execution Time: "), installBtn, timeSelect),
		schedStatus,
	)

	manualForm := container.NewVBox(
		widget.NewLabelWithStyle("Manual Run Control", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, widget.NewLabel("Target Project: "), dryRunCheck, projectSelect),
		runBtn,
	)

	return container.NewBorder(
		container.NewVBox(header, usageLabel, widget.NewSeparator(), scheduleForm, widget.NewSeparator(), manualForm, widget.NewSeparator()),
		nil, nil, nil,
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

		padrRoadmap := filepath.Join(p.Repository.Path, "PADR_ROADMAP.md")
		data, err := os.ReadFile(padrRoadmap)
		if err != nil {
			legacyRoadmap := filepath.Join(p.Repository.Path, "ROADMAP.md")
			data, err = os.ReadFile(legacyRoadmap)
		}

		if err == nil {
			roadmapEditor.SetText(string(data))
		} else {
			roadmapEditor.SetText(fmt.Sprintf("# %s — Autonomous Development Roadmap\n\n- [ ] Initial project setup\n", p.Name))
		}
	}

	repoList.UpdateItem = func(i int, o fyne.CanvasObject) {
		projs, _ := config.ListProjects()
		if i < len(projs) {
			o.(*widget.Label).SetText(fmt.Sprintf("%s (%s)", projs[i].Name, projs[i].Repository.Branch))
		}
	}

	repoList.OnSelected = func(id int) {
		projs, _ := config.ListProjects()
		if id < len(projs) {
			refreshRepoDetails(projs[id])
		}
	}

	newTaskEntry := widget.NewEntry()
	newTaskEntry.SetPlaceHolder("Type new task to append to PADR_ROADMAP.md...")

	addTaskBtn := widget.NewButtonWithIcon("Add Task", theme.ContentAddIcon(), func() {
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
			dialog.ShowInformation("Task Added", "Task appended to PADR_ROADMAP.md", w)
		}
	})

	saveRoadmapBtn := widget.NewButtonWithIcon("Save PADR_ROADMAP.md", theme.DocumentSaveIcon(), func() {
		if currentSelectedProj == nil {
			return
		}
		padrRoadmap := filepath.Join(currentSelectedProj.Repository.Path, "PADR_ROADMAP.md")
		err := os.WriteFile(padrRoadmap, []byte(roadmapEditor.Text), 0644)
		if err != nil {
			dialog.ShowError(err, w)
		} else {
			dialog.ShowInformation("Saved", "PADR_ROADMAP.md updated successfully.", w)
		}
	})

	addRepoBtn := widget.NewButtonWithIcon("Register Repository", theme.FolderNewIcon(), func() {
		nameEntry := widget.NewEntry()
		nameEntry.SetPlaceHolder("e.g. food-erp")

		pathEntry := widget.NewEntry()
		pathEntry.SetPlaceHolder("C:/dev/your-repo")

		branchEntry := widget.NewEntry()
		branchEntry.SetText("main")

		form := dialog.NewForm("Register Repository", "Register", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Project Name", nameEntry),
			widget.NewFormItem("Folder Path", pathEntry),
			widget.NewFormItem("Target Branch", branchEntry),
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
			dialog.ShowInformation("Success", fmt.Sprintf("Repository '%s' registered.", proj.Name), w)
		}, w)
		form.Resize(fyne.NewSize(500, 300))
		form.Show()
	})

	leftPane := container.NewBorder(addRepoBtn, nil, nil, nil, repoList)

	taskInputBar := container.NewBorder(nil, nil, nil, addTaskBtn, newTaskEntry)

	rightPane := container.NewBorder(
		container.NewVBox(
			detailName,
			detailPath,
			detailBranch,
			widget.NewSeparator(),
			widget.NewLabelWithStyle("PADR_ROADMAP.md (Autonomous Task Backlog)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			taskInputBar,
		),
		saveRoadmapBtn,
		nil,
		nil,
		container.NewScroll(roadmapEditor),
	)

	return container.NewHSplit(leftPane, rightPane)
}

// Tab 3: Providers & Fallback Models (Clean structured list, NO disabled text area!)
func buildProvidersTab(w fyne.Window, cfg *config.GlobalConfig) fyne.CanvasObject {
	header := widget.NewLabelWithStyle("AI Providers & Fallback Hierarchy", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	chainLabel := widget.NewLabel(fmt.Sprintf("Fallback Priority: %s", strings.Join(cfg.Routing.Models, " -> ")))

	// Form fields
	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("e.g. groq-fast, gemini-flash, ollama-local")

	typeSelect := widget.NewSelect([]string{"groq", "google", "openrouter", "ollama", "custom"}, nil)
	typeSelect.SetSelected("groq")

	modelEntry := widget.NewEntry()
	modelEntry.SetPlaceHolder("e.g. openai/gpt-oss-120b, gemini-2.5-flash")

	keyEnvEntry := widget.NewEntry()
	keyEnvEntry.SetPlaceHolder("e.g. GROQ_API_KEY, GEMINI_API_KEY")

	endpointEntry := widget.NewEntry()
	endpointEntry.SetPlaceHolder("e.g. http://localhost:11434 for Ollama")

	limitEntry := widget.NewEntry()
	limitEntry.SetText("5")

	// Structured List for configured providers
	var providerList *widget.List
	providerList = widget.NewList(
		func() int {
			return len(cfg.Routing.Models)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Provider Detail Item")
		},
		func(i int, o fyne.CanvasObject) {
			if i < len(cfg.Routing.Models) {
				mID := cfg.Routing.Models[i]
				p, ok := cfg.Providers[mID]
				if ok {
					status := "Active"
					if !p.Enabled {
						status = "Disabled"
					}
					o.(*widget.Label).SetText(fmt.Sprintf("#%d  [%s]  Type: %s  |  Model: %s  |  Limit: %d runs/day  |  %s",
						i+1, p.ID, p.Provider, p.Model, p.MaxDailyRuns, status))
				} else {
					o.(*widget.Label).SetText(fmt.Sprintf("#%d  [%s]  (Not configured)", i+1, mID))
				}
			}
		},
	)

	saveBtn := widget.NewButtonWithIcon("Save Provider Configuration", theme.DocumentSaveIcon(), func() {
		if idEntry.Text == "" || modelEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("Identifier and Model name are required"), w)
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
		chainLabel.SetText(fmt.Sprintf("Fallback Priority: %s", strings.Join(cfg.Routing.Models, " -> ")))
		providerList.Refresh()
		dialog.ShowInformation("Saved", fmt.Sprintf("Provider '%s' saved and active.", idEntry.Text), w)
	})

	formLayout := container.NewVBox(
		widget.NewLabelWithStyle("Add or Configure Provider", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, widget.NewLabel("Provider ID:    "), nil, idEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Provider Type:  "), nil, typeSelect),
		container.NewBorder(nil, nil, widget.NewLabel("Model ID:       "), nil, modelEntry),
		container.NewBorder(nil, nil, widget.NewLabel("API Key Env:    "), nil, keyEnvEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Endpoint URL:   "), nil, endpointEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Daily Limit:    "), nil, limitEntry),
		saveBtn,
	)

	topContent := container.NewVBox(
		header,
		chainLabel,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Active Provider Fallback Hierarchy", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	return container.NewBorder(
		topContent,
		formLayout,
		nil,
		nil,
		providerList,
	)
}

// Tab 4: GitHub Account Detection
func buildGitHubTab(w fyne.Window) fyne.CanvasObject {
	header := widget.NewLabelWithStyle("Connected GitHub Identity", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	statusBadge := widget.NewLabel("Checking connection...")
	userLabel := widget.NewLabel("Username: -")
	emailLabel := widget.NewLabel("Email: -")
	methodLabel := widget.NewLabel("Detection Method: -")
	scopesLabel := widget.NewLabel("Token Scopes: -")

	refreshAccount := func() {
		acc := github.DetectAccount(context.Background())
		if acc.LoggedIn {
			statusBadge.SetText("Status: Connected and Active")
			userLabel.SetText(fmt.Sprintf("GitHub Username: %s", acc.Username))
			emailLabel.SetText(fmt.Sprintf("Git Committer Email: %s", acc.Email))
			methodLabel.SetText(fmt.Sprintf("Authentication Method: %s", acc.Method))
			if acc.Scopes != "" {
				scopesLabel.SetText(fmt.Sprintf("Token Scopes: %s", acc.Scopes))
			} else {
				scopesLabel.SetText("Permissions: Default Git operations authorized")
			}
		} else {
			statusBadge.SetText("Status: Not Connected")
			userLabel.SetText("No GitHub credentials detected on this device.")
			emailLabel.SetText("Run 'git config --global user.name' or 'gh auth login'.")
		}
	}
	refreshAccount()

	refreshBtn := widget.NewButtonWithIcon("Refresh Connection Status", theme.ViewRefreshIcon(), refreshAccount)

	infoCard := widget.NewLabel(
		"PADR integrates with your local Windows Git credentials and credential store.\n" +
			"Autonomous commits and code pushes will be attributed directly to your personal\n" +
			"GitHub account and will count toward your contribution history.")

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

// Tab 5: Execution Logs (Clean structured table/list, NO disabled text box!)
func buildLogsTab(w fyne.Window, s *store.Store) fyne.CanvasObject {
	header := widget.NewLabelWithStyle("Autonomous Execution History", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	var runs []*store.RunRecord

	refreshRuns := func() {
		data, err := s.ListRuns("", 40)
		if err == nil {
			runs = data
		}
	}
	refreshRuns()

	logList := widget.NewList(
		func() int {
			return len(runs)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Log Item Placeholder")
		},
		func(i int, o fyne.CanvasObject) {
			if i < len(runs) {
				r := runs[i]
				detail := fmt.Sprintf("[%s]  Status: %-8s  |  Project: %-15s  |  Model: %s (%s)  |  Commits: %d  |  Duration: %ds",
					r.StartedAt.Format("2006-01-02 15:04"),
					strings.ToUpper(r.Status),
					r.Project,
					r.Provider,
					r.Model,
					r.Commits,
					r.DurationSeconds,
				)
				if r.Error != "" {
					detail += fmt.Sprintf("  |  Note: %s", r.Error)
				}
				o.(*widget.Label).SetText(detail)
			}
		},
	)

	refreshBtn := widget.NewButtonWithIcon("Refresh Run History", theme.ViewRefreshIcon(), func() {
		refreshRuns()
		logList.Refresh()
	})

	return container.NewBorder(
		header,
		refreshBtn,
		nil,
		nil,
		logList,
	)
}
