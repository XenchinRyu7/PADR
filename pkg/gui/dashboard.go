package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

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

	var refreshRunnerProviders func()
	tabSchedule := buildScheduleTab(w, globalCfg, dbStore, orch, func(ref func()) {
		refreshRunnerProviders = ref
	})
	tabRepos := buildReposTab(w)
	tabProviders := buildProvidersTab(w, globalCfg, func() {
		if refreshRunnerProviders != nil {
			refreshRunnerProviders()
		}
	})
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
func buildScheduleTab(w fyne.Window, globalCfg *config.GlobalConfig, s *store.Store, orch *runner.Orchestrator, registerRefresh func(func())) fyne.CanvasObject {
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

	providerSelect := widget.NewSelect([]string{"Auto (Router Fallback)"}, nil)
	refreshRunnerProviders := func() {
		opts := []string{"Auto (Router Fallback)"}
		for _, m := range globalCfg.Routing.Models {
			opts = append(opts, m)
		}
		for k := range globalCfg.Providers {
			found := false
			for _, o := range opts {
				if o == k {
					found = true
					break
				}
			}
			if !found {
				opts = append(opts, k)
			}
		}
		providerSelect.Options = opts
		if providerSelect.Selected == "" {
			providerSelect.SetSelected("Auto (Router Fallback)")
		} else {
			providerSelect.Refresh()
		}
	}
	refreshRunnerProviders()
	if registerRefresh != nil {
		registerRefresh(refreshRunnerProviders)
	}

	dryRunCheck := widget.NewCheck("Dry Run (Simulate changes without git push)", nil)
	dryRunCheck.SetChecked(false)

	logOutput := widget.NewMultiLineEntry()
	logOutput.SetPlaceHolder("Session execution output will appear here...")
	logOutput.Wrapping = fyne.TextWrapWord
	logOutput.SetMinRowsVisible(8)

	runBtn := widget.NewButtonWithIcon("Run Autonomous Pipeline Now", theme.MediaPlayIcon(), func() {
		selected := projectSelect.Selected
		isDryRun := dryRunCheck.Checked
		activeProv := providerSelect.Selected
		logOutput.SetText("Starting autonomous execution session...\n")

		go func() {
			opts := runner.RunOptions{
				DryRun:           isDryRun,
				ProviderOverride: activeProv,
				OnProgress: func(chunk string) {
					clean := stripANSI(chunk)
					if clean != "" {
						logOutput.SetText(logOutput.Text + clean)
					}
				},
			}
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
		container.NewBorder(nil, nil, widget.NewLabel("Provider: "), nil, providerSelect),
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

type providerListItem struct {
	widget.BaseWidget
	title   *widget.Label
	sub1    *widget.Label
	sub2    *widget.Label
	content *fyne.Container
	onTap   func()
}

func newProviderListItem() *providerListItem {
	item := &providerListItem{}
	item.title = widget.NewLabelWithStyle("Provider Title", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	item.sub1 = widget.NewLabel("Model: Placeholder")
	item.sub2 = widget.NewLabel("Status: Active | Limit: 5/day")
	item.content = container.NewVBox(item.title, item.sub1, item.sub2)
	item.ExtendBaseWidget(item)
	return item
}

func (p *providerListItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.content)
}

func (p *providerListItem) Tapped(_ *fyne.PointEvent) {
	if p.onTap != nil {
		p.onTap()
	}
}

// Tab 3: Providers (Two-Column Layout: Left List, Right Form Editor + Ping)
func buildProvidersTab(w fyne.Window, cfg *config.GlobalConfig, onProvidersChanged func()) fyne.CanvasObject {
	header := widget.NewLabelWithStyle("AI Providers & Fallback Priority", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	chainLabel := widget.NewLabel(fmt.Sprintf("Routing Chain: %s", strings.Join(cfg.Routing.Models, " -> ")))

	// Form fields
	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("e.g. groq-fast, zai-custom, gemini-flash, ollama-local")

	typeSelect := widget.NewSelect([]string{"groq", "google", "openrouter", "ollama", "custom"}, nil)
	typeSelect.SetSelected("groq")

	endpointEntry := widget.NewEntry()
	endpointEntry.SetPlaceHolder("Optional custom endpoint (leave empty for default cloud API)")

	modelEntry := widget.NewEntry()
	modelEntry.SetPlaceHolder("Enter model or click 'Test Connection / Ping' to fetch from API")

	detectedModelsSelect := widget.NewSelect([]string{"(Click 'Test Connection / Ping' to fetch models from API)"}, func(selected string) {
		if selected != "" && !strings.HasPrefix(selected, "(") {
			modelEntry.SetText(selected)
		}
	})

	typeSelect.OnChanged = func(selected string) {
		switch selected {
		case "google":
			endpointEntry.SetPlaceHolder("Optional: defaults to Google Generative AI endpoint")
			modelEntry.SetPlaceHolder("e.g. gemini-3.8-flash (or click Ping to fetch live models)")
		case "groq":
			endpointEntry.SetPlaceHolder("Optional: defaults to Groq Cloud endpoint")
			modelEntry.SetPlaceHolder("e.g. qwen/qwen3.8-27b (or click Ping to fetch live models)")
		case "openrouter":
			endpointEntry.SetPlaceHolder("Optional: defaults to OpenRouter endpoint")
			modelEntry.SetPlaceHolder("e.g. deepseek/deepseek-chat:free (or click Ping to fetch live models)")
		case "custom":
			endpointEntry.SetPlaceHolder("Required: e.g. https://api.z.ai/api/paas/v4 or https://api.deepseek.com")
			modelEntry.SetPlaceHolder("Enter model name (or click Ping to fetch live models)")
		case "ollama":
			endpointEntry.SetPlaceHolder("Optional: default is http://localhost:11434")
			modelEntry.SetPlaceHolder("Enter model name (or click Ping to fetch local models)")
		default:
			endpointEntry.SetPlaceHolder("Optional custom endpoint (leave empty for default cloud API)")
		}
		go func(prov string) {
			ocModels := router.FetchOpenCodeModels(context.Background(), prov)
			if len(ocModels) > 0 {
				detectedModelsSelect.Options = ocModels
				if modelEntry.Text == "" {
					modelEntry.SetText(ocModels[0])
					detectedModelsSelect.SetSelected(ocModels[0])
				}
			} else {
				detectedModelsSelect.Options = []string{"(Click 'Test Connection / Ping' to fetch models from API)"}
				detectedModelsSelect.SetSelected("(Click 'Test Connection / Ping' to fetch models from API)")
			}
			detectedModelsSelect.Refresh()
		}(selected)
	}
	typeSelect.OnChanged(typeSelect.Selected)

	keyEnvEntry := widget.NewEntry()
	keyEnvEntry.SetPlaceHolder("API Key or env var name (e.g. GROQ_API_KEY)")

	limitEntry := widget.NewEntry()
	limitEntry.SetText("5")

	pingResultLabel := widget.NewLabel("Status: Ready to test")

	var selectedModelID string

	var providerList *widget.List

	selectProvider := func(id int) {
		if id < len(cfg.Routing.Models) {
			selectedModelID = cfg.Routing.Models[id]
			if p, ok := cfg.Providers[selectedModelID]; ok {
				idEntry.SetText(p.ID)
				typeSelect.SetSelected(p.Provider)
				modelEntry.SetText(p.Model)
				displayKey := p.APIKeyEnv
				if p.APIKey != "" {
					displayKey = p.APIKey
				}
				keyEnvEntry.SetText(displayKey)
				endpointEntry.SetText(p.Endpoint)
				limitEntry.SetText(fmt.Sprintf("%d", p.MaxDailyRuns))
				pingResultLabel.SetText("Status: Selected - " + p.KeyStatus())
			}
		}
	}

	providerList = widget.NewList(
		func() int {
			return len(cfg.Routing.Models)
		},
		func() fyne.CanvasObject {
			return newProviderListItem()
		},
		func(i int, o fyne.CanvasObject) {
			item := o.(*providerListItem)
			item.onTap = func() {
				selectProvider(i)
				providerList.Select(i)
			}
			if i < len(cfg.Routing.Models) {
				mID := cfg.Routing.Models[i]
				p, ok := cfg.Providers[mID]
				if ok {
					status := "Active"
					if !p.Enabled {
						status = "Disabled"
					}
					item.title.SetText(fmt.Sprintf("#%d [%s] (%s)", i+1, p.ID, p.Provider))
					item.sub1.SetText(fmt.Sprintf("Model: %s", p.Model))
					item.sub2.SetText(fmt.Sprintf("%s | Limit: %d/day | %s", p.KeyStatus(), p.MaxDailyRuns, status))
				} else {
					item.title.SetText(fmt.Sprintf("#%d [%s]", i+1, mID))
					item.sub1.SetText("Not configured")
					item.sub2.SetText("")
				}
			}
		},
	)

	clearForm := func() {
		providerList.UnselectAll()
		selectedModelID = ""
		idEntry.SetText("")
		modelEntry.SetText("")
		detectedModelsSelect.Options = []string{"(Click 'Test Connection / Ping' to fetch models)"}
		detectedModelsSelect.SetSelected("(Click 'Test Connection / Ping' to fetch models)")
		keyEnvEntry.SetText("")
		endpointEntry.SetText("")
		limitEntry.SetText("5")
		pingResultLabel.SetText("Status: Ready")
	}

	providerList.OnSelected = func(id int) {
		selectProvider(id)
	}

	pingBtn := widget.NewButtonWithIcon("Test Connection / Ping", theme.MediaPlayIcon(), func() {
		keyVal := strings.TrimSpace(keyEnvEntry.Text)
		directKey := ""
		envName := ""
		if config.IsEnvVarName(keyVal) {
			envName = keyVal
		} else {
			directKey = keyVal
			envName = keyVal
		}

		p := config.ProviderConfig{
			ID:        idEntry.Text,
			Provider:  typeSelect.Selected,
			Model:     modelEntry.Text,
			APIKeyEnv: envName,
			APIKey:    directKey,
			Endpoint:  endpointEntry.Text,
		}
		pingResultLabel.SetText("Testing connection & fetching models...")
		go func() {
			res := router.PingProvider(context.Background(), p)
			if res.Success {
				pingResultLabel.SetText(fmt.Sprintf("Success: %s", res.Message))
				if len(res.AvailableModels) > 0 {
					detectedModelsSelect.Options = res.AvailableModels
					detectedModelsSelect.Refresh()
				}
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

		keyVal := strings.TrimSpace(keyEnvEntry.Text)
		directKey := ""
		envName := ""
		if config.IsEnvVarName(keyVal) {
			envName = keyVal
		} else {
			directKey = keyVal
			envName = keyVal
		}

		// If editing an existing provider and renamed the identifier:
		if selectedModelID != "" && selectedModelID != idEntry.Text {
			delete(cfg.Providers, selectedModelID)
			for idx, m := range cfg.Routing.Models {
				if m == selectedModelID {
					cfg.Routing.Models[idx] = idEntry.Text
					break
				}
			}
		}

		cfg.Providers[idEntry.Text] = config.ProviderConfig{
			ID:           idEntry.Text,
			Provider:     typeSelect.Selected,
			Model:        modelEntry.Text,
			APIKeyEnv:    envName,
			APIKey:       directKey,
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

		selectedModelID = idEntry.Text
		_ = config.SaveGlobalConfig(cfg)
		chainLabel.SetText(fmt.Sprintf("Routing Chain: %s", strings.Join(cfg.Routing.Models, " -> ")))
		providerList.Refresh()
		if onProvidersChanged != nil {
			onProvidersChanged()
		}
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
		if onProvidersChanged != nil {
			onProvidersChanged()
		}
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
				if onProvidersChanged != nil {
					onProvidersChanged()
				}
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
		container.NewBorder(nil, nil, widget.NewLabel("Identifier:    "), nil, idEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Provider Type: "), nil, typeSelect),
		container.NewBorder(nil, nil, widget.NewLabel("Endpoint URL:  "), nil, endpointEntry),
		container.NewBorder(nil, nil, widget.NewLabel("API Key / Env: "), nil, keyEnvEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Model ID:      "), nil, modelEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Fetch Model:   "), nil, detectedModelsSelect),
		container.NewBorder(nil, nil, widget.NewLabel("Daily Limit:   "), nil, limitEntry),
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
		data, err := s.ListRuns("", 50)
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
			title := widget.NewLabelWithStyle("Run Title Placeholder", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			sub := widget.NewLabel("Provider & Model Placeholder")
			return container.NewVBox(title, sub)
		},
		func(i int, o fyne.CanvasObject) {
			if i < len(runs) {
				r := runs[i]
				box := o.(*fyne.Container)
				title := box.Objects[0].(*widget.Label)
				sub := box.Objects[1].(*widget.Label)

				projName := r.Project
				if len(projName) > 24 {
					projName = projName[:21] + "..."
				}

				title.SetText(fmt.Sprintf("[%s] %s  •  %s (%ds)",
					r.StartedAt.Format("15:04:05"),
					projName,
					strings.ToUpper(r.Status),
					r.DurationSeconds,
				))

				provShort := r.Provider
				if len(provShort) > 18 {
					provShort = provShort[:15] + "..."
				}
				modelShort := r.Model
				if len(modelShort) > 28 {
					modelShort = modelShort[:25] + "..."
				}
				sub.SetText(fmt.Sprintf("Provider: %s  |  Model: %s  |  Commits: %d",
					provShort, modelShort, r.Commits))
			}
		},
	)

	logList.OnSelected = func(id int) {
		if id >= len(runs) {
			return
		}
		r := runs[id]

		detailVBox := container.NewVBox(
			widget.NewLabelWithStyle(fmt.Sprintf("Run #%d — %s", r.ID, r.Project), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabel(fmt.Sprintf("Started: %s  |  Finished: %s  |  Duration: %ds",
				r.StartedAt.Format("2006-01-02 15:04:05"),
				r.FinishedAt.Format("15:04:05"),
				r.DurationSeconds)),
			widget.NewLabel(fmt.Sprintf("Status: %s  |  Commits: %d  |  Tasks Completed: %d",
				strings.ToUpper(r.Status), r.Commits, r.TasksCompleted)),
			widget.NewLabel(fmt.Sprintf("Provider: %s  |  Model: %s", r.Provider, r.Model)),
			widget.NewSeparator(),
		)

		if r.Error != "" {
			errBox := widget.NewMultiLineEntry()
			errBox.Wrapping = fyne.TextWrapWord
			errBox.SetText(r.Error)
			detailVBox.Add(widget.NewLabelWithStyle("Error / Reason:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
			detailVBox.Add(errBox)
		}

		if r.LogOutput != "" {
			outBox := widget.NewMultiLineEntry()
			outBox.Wrapping = fyne.TextWrapWord
			outBox.SetText(r.LogOutput)
			detailVBox.Add(widget.NewLabelWithStyle("Agent Output / Logs:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
			detailVBox.Add(outBox)
		}

		scrollContent := container.NewVScroll(detailVBox)
		d := dialog.NewCustom("Execution Run Details", "Close", scrollContent, w)
		d.Resize(fyne.NewSize(750, 520))
		d.Show()
		logList.UnselectAll()
	}

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
