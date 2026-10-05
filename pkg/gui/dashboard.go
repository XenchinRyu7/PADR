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

	"github.com/padr-runner/padr/pkg/agent"
	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/github"
	"github.com/padr-runner/padr/pkg/router"
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
	w.Resize(fyne.NewSize(1040, 720))
	w.SetIcon(icon)

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

	tabSchedule := buildScheduleTab(w, globalCfg, dbStore, orch)
	tabRepos := buildReposTab(w)
	tabProviders := buildProvidersTab(w, globalCfg)
	tabEngine := buildEngineTab(w)
	tabGitHub := buildGitHubTab(w)
	tabLogs := buildLogsTab(w, dbStore)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Runner", theme.MediaPlayIcon(), tabSchedule),
		container.NewTabItemWithIcon("Repositories", theme.FolderIcon(), tabRepos),
		container.NewTabItemWithIcon("Providers", theme.StorageIcon(), tabProviders),
		container.NewTabItemWithIcon("Engine", theme.SettingsIcon(), tabEngine),
		container.NewTabItemWithIcon("Account", theme.AccountIcon(), tabGitHub),
		container.NewTabItemWithIcon("Logs", theme.DocumentIcon(), tabLogs),
	)
	tabs.SetTabLocation(container.TabLocationLeading)

	w.SetContent(tabs)

	if desk, ok := a.(desktop.App); ok {
		menu := fyne.NewMenu("PADR",
			fyne.NewMenuItem("Open Dashboard", func() {
				w.Show()
				w.RequestFocus()
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Quit PADR", func() {
				a.Quit()
			}),
		)
		desk.SetSystemTrayMenu(menu)
		desk.SetSystemTrayIcon(icon)
	}

	w.SetCloseIntercept(func() {
		w.Hide()
	})

	existingProjs, _ := config.ListProjects()
	if len(existingProjs) == 0 {
		ShowSetupWizard(w, func() {
			w.Show()
		})
	}

	w.ShowAndRun()
}

// Tab 1: Schedule & Execution Runner (With Delete Schedule & Responsive Scroll)
func buildScheduleTab(w fyne.Window, globalCfg *config.GlobalConfig, s *store.Store, orch *runner.Orchestrator) fyne.CanvasObject {
	header := widget.NewLabelWithStyle("Autonomous Runner & Scheduler", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	today := time.Now().Format("2006-01-02")
	usageCount, _ := s.GetTotalDailyUsage(today)
	usageLabel := widget.NewLabel(fmt.Sprintf("Today's Runs: %d / %d  |  Max Runtime Window: %d minutes",
		usageCount, globalCfg.Limits.MaxRunsPerDay, globalCfg.Limits.MaxRuntimeMinutes))

	timeSelect := widget.NewSelect([]string{"08:00", "09:00", "12:00", "15:00", "18:00", "21:00", "23:00"}, nil)
	timeSelect.SetSelected("09:00")

	schedStatus := widget.NewLabel("Scheduler Status: Checking Windows Task Scheduler...")

	updateSchedStatus := func() {
		sched := scheduler.NewScheduler()
		tasks, err := sched.List(context.Background())
		if err != nil || len(tasks) == 0 {
			schedStatus.SetText("Scheduler Status: Inactive (No scheduled task registered in Windows)")
		} else {
			schedStatus.SetText(fmt.Sprintf("Scheduler Status: Active (%d scheduled task(s) in Windows Task Scheduler)", len(tasks)))
		}
	}
	updateSchedStatus()

	installBtn := widget.NewButtonWithIcon("Sync / Enable Schedule", theme.ConfirmIcon(), func() {
		projects, err := config.ListProjects()
		if err != nil || len(projects) == 0 {
			dialog.ShowInformation("No Projects", "Register at least one repository first in the Repositories tab.", w)
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

		updateSchedStatus()
		dialog.ShowInformation("Schedule Enabled", fmt.Sprintf("Registered daily task at %s in Windows Task Scheduler (%d project(s)).", timeSelect.Selected, successCount), w)
	})

	deleteSchedBtn := widget.NewButtonWithIcon("Delete / Remove Schedule", theme.DeleteIcon(), func() {
		projects, err := config.ListProjects()
		if err != nil || len(projects) == 0 {
			return
		}

		dialog.ShowConfirm("Remove Schedule", "Are you sure you want to remove all PADR tasks from Windows Task Scheduler?", func(confirmed bool) {
			if confirmed {
				sched := scheduler.NewScheduler()
				for _, p := range projects {
					_ = sched.Uninstall(context.Background(), p.Name)
				}
				updateSchedStatus()
				dialog.ShowInformation("Schedule Removed", "All PADR scheduled tasks have been deleted from Windows Task Scheduler.", w)
			}
		}, w)
	})

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

	dryRunCheck := widget.NewCheck("Dry Run (Simulate changes without git push)", nil)
	dryRunCheck.SetChecked(false)

	logOutput := widget.NewMultiLineEntry()
	logOutput.SetPlaceHolder("Session execution output will appear here...")
	logOutput.Wrapping = fyne.TextWrapWord
	logOutput.SetMinRowsVisible(8)

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

			today := time.Now().Format("2006-01-02")
			cnt, _ := s.GetTotalDailyUsage(today)
			usageLabel.SetText(fmt.Sprintf("Today's Runs: %d / %d  |  Max Runtime Window: %d minutes",
				cnt, globalCfg.Limits.MaxRunsPerDay, globalCfg.Limits.MaxRuntimeMinutes))
		}()
	})

	schedButtons := container.NewHBox(installBtn, deleteSchedBtn)

	scheduleForm := container.NewVBox(
		widget.NewLabelWithStyle("Daily Schedule Trigger", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, widget.NewLabel("Trigger Time: "), schedButtons, timeSelect),
		schedStatus,
	)

	manualForm := container.NewVBox(
		widget.NewLabelWithStyle("Manual Execution Trigger", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, widget.NewLabel("Target: "), dryRunCheck, projectSelect),
		runBtn,
	)

	topSection := container.NewVBox(
		header,
		usageLabel,
		widget.NewSeparator(),
		scheduleForm,
		widget.NewSeparator(),
		manualForm,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Live Output Stream", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
	)

	return container.NewBorder(
		topSection,
		nil, nil, nil,
		container.NewScroll(logOutput),
	)
}

// Tab 2: Repositories, Rules, Architecture & PADR_ROADMAP.md
func buildReposTab(w fyne.Window) fyne.CanvasObject {
	repoList := widget.NewList(
		func() int {
			projs, _ := config.ListProjects()
			return len(projs)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Project Item Placeholder")
		},
		nil,
	)

	detailName := widget.NewLabelWithStyle("Select a repository", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	detailPath := widget.NewLabel("Path: -")

	branchEntry := widget.NewEntry()
	rulesFileEntry := widget.NewEntry()
	archFileEntry := widget.NewEntry()
	maxTasksEntry := widget.NewEntry()
	maxTasksEntry.SetText("1")

	docSelector := widget.NewSelect([]string{"PADR_ROADMAP.md (Roadmap)", "PROJECT.md (Rules)", "ARCHITECTURE.md (Architecture)"}, nil)
	docSelector.SetSelected("PADR_ROADMAP.md (Roadmap)")

	docEditor := widget.NewMultiLineEntry()
	docEditor.Wrapping = fyne.TextWrapWord
	docEditor.SetPlaceHolder("Document contents will be loaded here...")

	var currentSelectedProj *config.ProjectConfig

	loadDocument := func() {
		if currentSelectedProj == nil {
			return
		}
		targetFilename := "PADR_ROADMAP.md"
		switch docSelector.Selected {
		case "PROJECT.md (Rules)":
			targetFilename = currentSelectedProj.Development.RulesFile
			if targetFilename == "" {
				targetFilename = "PROJECT.md"
			}
		case "ARCHITECTURE.md (Architecture)":
			targetFilename = currentSelectedProj.Development.ArchitectureFile
			if targetFilename == "" {
				targetFilename = "ARCHITECTURE.md"
			}
		default:
			targetFilename = currentSelectedProj.Development.Roadmap
			if targetFilename == "" {
				targetFilename = "PADR_ROADMAP.md"
			}
		}

		fullPath := filepath.Join(currentSelectedProj.Repository.Path, targetFilename)
		data, err := os.ReadFile(fullPath)
		if err == nil {
			docEditor.SetText(string(data))
		} else {
			docEditor.SetText(fmt.Sprintf("# %s\n\n(File not found at %s. Edit here and click 'Save Document' to create it.)\n", targetFilename, fullPath))
		}
	}

	docSelector.OnChanged = func(s string) {
		loadDocument()
	}

	refreshRepoDetails := func(p *config.ProjectConfig) {
		currentSelectedProj = p
		detailName.SetText(fmt.Sprintf("Repository: %s", p.Name))
		detailPath.SetText(fmt.Sprintf("Path: %s", p.Repository.Path))
		branchEntry.SetText(p.Repository.Branch)

		rulesName := p.Development.RulesFile
		if rulesName == "" {
			rulesName = "PROJECT.md"
		}
		rulesFileEntry.SetText(rulesName)

		archName := p.Development.ArchitectureFile
		if archName == "" {
			archName = "ARCHITECTURE.md"
		}
		archFileEntry.SetText(archName)

		maxTasks := p.Development.MaxTasks
		if maxTasks <= 0 {
			maxTasks = 1
		}
		maxTasksEntry.SetText(fmt.Sprintf("%d", maxTasks))

		loadDocument()
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

	saveSettingsBtn := widget.NewButtonWithIcon("Save Project Settings", theme.DocumentSaveIcon(), func() {
		if currentSelectedProj == nil {
			return
		}
		currentSelectedProj.Repository.Branch = branchEntry.Text
		currentSelectedProj.Development.RulesFile = rulesFileEntry.Text
		currentSelectedProj.Development.ArchitectureFile = archFileEntry.Text
		mt := 1
		_, _ = fmt.Sscanf(maxTasksEntry.Text, "%d", &mt)
		currentSelectedProj.Development.MaxTasks = mt

		_ = config.SaveProjectConfig(currentSelectedProj.Repository.Path, currentSelectedProj)
		_ = config.RegisterProject(currentSelectedProj)
		dialog.ShowInformation("Saved", "Project configuration saved.", w)
	})

	newTaskEntry := widget.NewEntry()
	newTaskEntry.SetPlaceHolder("Type task to append to PADR_ROADMAP.md...")

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
			loadDocument()
			dialog.ShowInformation("Task Appended", "New task added to PADR_ROADMAP.md", w)
		}
	})

	saveDocBtn := widget.NewButtonWithIcon("Save Document File", theme.DocumentSaveIcon(), func() {
		if currentSelectedProj == nil {
			return
		}
		targetFilename := "PADR_ROADMAP.md"
		switch docSelector.Selected {
		case "PROJECT.md (Rules)":
			targetFilename = rulesFileEntry.Text
		case "ARCHITECTURE.md (Architecture)":
			targetFilename = archFileEntry.Text
		}
		fullPath := filepath.Join(currentSelectedProj.Repository.Path, targetFilename)
		err := os.WriteFile(fullPath, []byte(docEditor.Text), 0644)
		if err != nil {
			dialog.ShowError(err, w)
		} else {
			dialog.ShowInformation("Saved", fmt.Sprintf("%s saved successfully.", targetFilename), w)
		}
	})

	addRepoBtn := widget.NewButtonWithIcon("Register Repo", theme.FolderNewIcon(), func() {
		nameEntry := widget.NewEntry()
		nameEntry.SetPlaceHolder("e.g. restaurant-erp")

		pathEntry := widget.NewEntry()
		pathEntry.SetPlaceHolder("C:/dev/your-repo")

		branchEntryForm := widget.NewEntry()
		branchEntryForm.SetText("main")

		form := dialog.NewForm("Register Repository", "Register", "Cancel", []*widget.FormItem{
			widget.NewFormItem("Project Name", nameEntry),
			widget.NewFormItem("Folder Path", pathEntry),
			widget.NewFormItem("Target Branch", branchEntryForm),
		}, func(ok bool) {
			if !ok || nameEntry.Text == "" || pathEntry.Text == "" {
				return
			}
			absPath, _ := filepath.Abs(pathEntry.Text)
			proj := config.DefaultProjectConfig(nameEntry.Text, absPath)
			proj.Repository.Branch = branchEntryForm.Text

			_ = config.SaveProjectConfig(absPath, proj)
			_ = config.RegisterProject(proj)
			repoList.Refresh()
			dialog.ShowInformation("Success", fmt.Sprintf("Repository '%s' registered.", proj.Name), w)
		}, w)
		form.Resize(fyne.NewSize(520, 300))
		form.Show()
	})

	deleteRepoBtn := widget.NewButtonWithIcon("Unregister", theme.DeleteIcon(), func() {
		if currentSelectedProj == nil {
			return
		}
		regDir, _ := config.GetProjectsRegistryDir()
		regFile := filepath.Join(regDir, fmt.Sprintf("%s.yaml", currentSelectedProj.Name))
		_ = os.Remove(regFile)
		currentSelectedProj = nil
		repoList.Refresh()
		dialog.ShowInformation("Unregistered", "Repository removed from PADR registry.", w)
	})

	leftControls := container.NewHBox(addRepoBtn, deleteRepoBtn)
	leftPane := container.NewBorder(leftControls, nil, nil, nil, repoList)

	settingsCard := container.NewVBox(
		detailName,
		detailPath,
		container.NewBorder(nil, nil, widget.NewLabel("Branch:       "), nil, branchEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Rules File:   "), nil, rulesFileEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Architecture: "), nil, archFileEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Max Tasks/Run:"), nil, maxTasksEntry),
		saveSettingsBtn,
		widget.NewSeparator(),
	)

	editorBar := container.NewBorder(nil, nil, widget.NewLabel("Document: "), saveDocBtn, docSelector)
	taskInputBar := container.NewBorder(nil, nil, nil, addTaskBtn, newTaskEntry)

	rightPane := container.NewBorder(
		container.NewVBox(settingsCard, editorBar, taskInputBar),
		nil, nil, nil,
		container.NewScroll(docEditor),
	)

	return container.NewHSplit(leftPane, rightPane)
}

// Tab 3: Providers (Two-Column Layout: Left List, Right Form Editor + Ping)
func buildProvidersTab(w fyne.Window, cfg *config.GlobalConfig) fyne.CanvasObject {
	header := widget.NewLabelWithStyle("AI Providers & Fallback Priority", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	chainLabel := widget.NewLabel(fmt.Sprintf("Routing Chain: %s", strings.Join(cfg.Routing.Models, " -> ")))

	// Form fields
	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("e.g. groq-fast, zai-custom, gemini-flash, ollama-local")

	typeSelect := widget.NewSelect([]string{"groq", "google", "openrouter", "ollama", "custom"}, nil)
	typeSelect.SetSelected("groq")

	modelEntry := widget.NewEntry()
	modelEntry.SetPlaceHolder("e.g. openai/gpt-oss-120b, GLM-4.5-Flash, gemini-2.5-flash")

	keyEnvEntry := widget.NewEntry()
	keyEnvEntry.SetPlaceHolder("API Key Environment Variable (or literal key)")

	endpointEntry := widget.NewEntry()
	endpointEntry.SetPlaceHolder("Custom URL e.g. https://api.z.ai/v1 or http://localhost:11434")

	limitEntry := widget.NewEntry()
	limitEntry.SetText("5")

	pingResultLabel := widget.NewLabel("Status: Ready to test")

	var selectedModelID string

	var providerList *widget.List
	providerList = widget.NewList(
		func() int {
			return len(cfg.Routing.Models)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Provider Placeholder")
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
					o.(*widget.Label).SetText(fmt.Sprintf("#%d [%s]\n%s (%s)\nLimit: %d/day | %s",
						i+1, p.ID, p.Model, p.Provider, p.MaxDailyRuns, status))
				} else {
					o.(*widget.Label).SetText(fmt.Sprintf("#%d [%s] (Not configured)", i+1, mID))
				}
			}
		},
	)

	clearForm := func() {
		selectedModelID = ""
		idEntry.SetText("")
		modelEntry.SetText("")
		keyEnvEntry.SetText("")
		endpointEntry.SetText("")
		limitEntry.SetText("5")
		pingResultLabel.SetText("Status: Ready")
	}

	providerList.OnSelected = func(id int) {
		if id < len(cfg.Routing.Models) {
			selectedModelID = cfg.Routing.Models[id]
			if p, ok := cfg.Providers[selectedModelID]; ok {
				idEntry.SetText(p.ID)
				typeSelect.SetSelected(p.Provider)
				modelEntry.SetText(p.Model)
				keyEnvEntry.SetText(p.APIKeyEnv)
				endpointEntry.SetText(p.Endpoint)
				limitEntry.SetText(fmt.Sprintf("%d", p.MaxDailyRuns))
				pingResultLabel.SetText("Status: Selected")
			}
		}
	}

	pingBtn := widget.NewButtonWithIcon("Test Connection / Ping", theme.MediaPlayIcon(), func() {
		p := config.ProviderConfig{
			ID:        idEntry.Text,
			Provider:  typeSelect.Selected,
			Model:     modelEntry.Text,
			APIKeyEnv: keyEnvEntry.Text,
			Endpoint:  endpointEntry.Text,
		}
		pingResultLabel.SetText("Testing connection...")
		go func() {
			res := router.PingProvider(context.Background(), p)
			if res.Success {
				pingResultLabel.SetText(fmt.Sprintf("Success: %s", res.Message))
			} else {
				pingResultLabel.SetText(fmt.Sprintf("Failed: %s", res.Message))
			}
		}()
	})

	saveBtn := widget.NewButtonWithIcon("Save Provider", theme.DocumentSaveIcon(), func() {
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
		chainLabel.SetText(fmt.Sprintf("Routing Chain: %s", strings.Join(cfg.Routing.Models, " -> ")))
		providerList.Refresh()
		dialog.ShowInformation("Saved", fmt.Sprintf("Provider '%s' saved.", idEntry.Text), w)
	})

	deleteBtn := widget.NewButtonWithIcon("Delete Provider", theme.DeleteIcon(), func() {
		targetID := idEntry.Text
		if targetID == "" {
			targetID = selectedModelID
		}
		if targetID == "" {
			return
		}

		delete(cfg.Providers, targetID)
		var newModels []string
		for _, m := range cfg.Routing.Models {
			if m != targetID {
				newModels = append(newModels, m)
			}
		}
		cfg.Routing.Models = newModels
		_ = config.SaveGlobalConfig(cfg)

		clearForm()
		chainLabel.SetText(fmt.Sprintf("Routing Chain: %s", strings.Join(cfg.Routing.Models, " -> ")))
		providerList.Refresh()
		dialog.ShowInformation("Deleted", fmt.Sprintf("Provider '%s' removed.", targetID), w)
	})

	newBtn := widget.NewButtonWithIcon("New Provider", theme.ContentAddIcon(), func() {
		clearForm()
	})

	clearAllBtn := widget.NewButtonWithIcon("Clear All", theme.ContentClearIcon(), func() {
		dialog.ShowConfirm("Clear All Providers", "Do you want to clear all configured providers and start fresh?", func(ok bool) {
			if ok {
				cfg.Providers = make(map[string]config.ProviderConfig)
				cfg.Routing.Models = []string{}
				_ = config.SaveGlobalConfig(cfg)
				clearForm()
				chainLabel.SetText("Routing Chain: None configured")
				providerList.Refresh()
			}
		}, w)
	})

	leftControls := container.NewHBox(newBtn, clearAllBtn)
	leftPane := container.NewBorder(
		container.NewVBox(widget.NewLabelWithStyle("Configured Providers", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), leftControls),
		nil, nil, nil,
		providerList,
	)

	actionButtons := container.NewHBox(saveBtn, pingBtn, deleteBtn)

	formContent := container.NewVBox(
		widget.NewLabelWithStyle("Provider Configuration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, widget.NewLabel("Identifier:   "), nil, idEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Provider Type:"), nil, typeSelect),
		container.NewBorder(nil, nil, widget.NewLabel("Model ID:     "), nil, modelEntry),
		container.NewBorder(nil, nil, widget.NewLabel("API Key Env:  "), nil, keyEnvEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Endpoint URL: "), nil, endpointEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Daily Limit:  "), nil, limitEntry),
		pingResultLabel,
		actionButtons,
	)

	rightPane := container.NewScroll(formContent)

	topHeader := container.NewVBox(header, chainLabel, widget.NewSeparator())

	splitLayout := container.NewHSplit(leftPane, rightPane)
	splitLayout.SetOffset(0.35)

	return container.NewBorder(topHeader, nil, nil, nil, splitLayout)
}

// Tab 4: Agent Engine Configuration (Clean Dedicated Tab)
func buildEngineTab(w fyne.Window) fyne.CanvasObject {
	header := widget.NewLabelWithStyle("Agent Engine Configuration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	engineSelect := widget.NewSelect([]string{"opencode", "cline", "aider", "mock"}, nil)
	engineSelect.SetSelected("opencode")

	customEnginePathEntry := widget.NewEntry()
	customEnginePathEntry.SetPlaceHolder("Leave empty to use PATH, or specify full executable path")

	engineStatusLabel := widget.NewLabel("Checking engine...")
	engineHelpLabel := widget.NewLabel("")

	refreshEngineStatus := func() {
		status := agent.DetectEngine(context.Background(), engineSelect.Selected, customEnginePathEntry.Text)
		if status.Installed {
			engineStatusLabel.SetText(fmt.Sprintf("Status: Installed and Ready (%s)", status.Path))
			engineHelpLabel.SetText("Version / Detection: " + status.Version)
		} else {
			engineStatusLabel.SetText(fmt.Sprintf("Status: NOT FOUND — %s not detected in PATH", status.Name))
			engineHelpLabel.SetText("How to install: " + status.InstallHelp)
		}
	}
	refreshEngineStatus()

	engineSelect.OnChanged = func(s string) {
		refreshEngineStatus()
	}

	checkBtn := widget.NewButtonWithIcon("Re-check Engine Status", theme.ViewRefreshIcon(), refreshEngineStatus)

	infoCard := widget.NewLabel(
		"PADR acts as an autonomous orchestrator that delegates coding sessions to local CLI tools.\n" +
			"• OpenCode: Fast, non-interactive mode natively supported.\n" +
			"• Mock Engine: Built-in offline simulator for safe local dry-runs.\n" +
			"• Cline / Aider: Supported via CLI adapter.")

	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		container.NewBorder(nil, nil, widget.NewLabel("Coding Agent Engine: "), nil, engineSelect),
		container.NewBorder(nil, nil, widget.NewLabel("Custom Binary Path:  "), nil, customEnginePathEntry),
		widget.NewSeparator(),
		engineStatusLabel,
		engineHelpLabel,
		checkBtn,
		widget.NewSeparator(),
		infoCard,
	)

	return container.NewScroll(content)
}

// Tab 5: GitHub Account Detection
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
			emailLabel.SetText("Configure using 'git config --global user.name' or 'gh auth login'.")
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

// Tab 6: Execution Logs
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
