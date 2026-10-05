package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/padr-runner/padr/pkg/config"
	"github.com/padr-runner/padr/pkg/github"
	"github.com/padr-runner/padr/pkg/scheduler"
)

// ShowSetupWizard opens a step-by-step onboarding wizard
func ShowSetupWizard(parent fyne.Window, onComplete func()) {
	wizardWin := fyne.CurrentApp().NewWindow("PADR — Quick Setup Wizard")
	wizardWin.Resize(fyne.NewSize(620, 500))
	wizardWin.SetIcon(GetAppIcon())

	currentStep := 1
	const totalSteps = 5

	stepTitle := widget.NewLabelWithStyle("Step 1/5: Welcome to PADR", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	contentBox := container.NewStack()

	// Form inputs preserved across steps
	// Step 2: GitHub
	githubInfoLabel := widget.NewLabel("Checking GitHub credentials...")

	// Step 3: Provider
	providerSelect := widget.NewSelect([]string{"groq", "google (gemini)", "openrouter", "ollama (local)"}, nil)
	providerSelect.SetSelected("groq")
	apiKeyEntry := widget.NewPasswordEntry()
	apiKeyEntry.SetPlaceHolder("Enter API Key (optional for Ollama)")

	// Step 4: Repo
	repoNameEntry := widget.NewEntry()
	repoNameEntry.SetText("my-project")
	repoPathEntry := widget.NewEntry()
	if cwd, err := os.Getwd(); err == nil {
		repoPathEntry.SetText(cwd)
	}

	// Step 5: Schedule & Shortcuts
	timeSelect := widget.NewSelect([]string{"09:00", "13:00", "18:00", "22:00"}, nil)
	timeSelect.SetSelected("09:00")
	createShortcutCheck := widget.NewCheck("Create Desktop Shortcut (PADR.lnk)", nil)
	createShortcutCheck.SetChecked(true)
	startupCheck := widget.NewCheck("Launch in System Tray on Windows Startup", nil)
	startupCheck.SetChecked(true)

	// Buttons
	backBtn := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), nil)
	nextBtn := widget.NewButtonWithIcon("Next", theme.NavigateNextIcon(), nil)

	var updateStep func()

	updateStep = func() {
		stepTitle.SetText(fmt.Sprintf("Step %d of %d", currentStep, totalSteps))
		backBtn.Enable()
		if currentStep == 1 {
			backBtn.Disable()
		}

		if currentStep == totalSteps {
			nextBtn.SetText("Finish & Launch")
			nextBtn.SetIcon(theme.ConfirmIcon())
		} else {
			nextBtn.SetText("Next")
			nextBtn.SetIcon(theme.NavigateNextIcon())
		}

		switch currentStep {
		case 1:
			// Welcome Screen
			welcomeLabel := widget.NewLabelWithStyle("Welcome to PADR!", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
			descLabel := widget.NewLabel(
				"Personal Autonomous Development Runner (PADR) turns your computer\n" +
					"into a tireless autonomous worker that pushes roadmap features to your repos\n" +
					"on schedule while you sleep.\n\n" +
					"This wizard will help you configure in 60 seconds:\n" +
					"  ✓ Your verified GitHub identity\n" +
					"  ✓ AI model provider and credentials\n" +
					"  ✓ Target repository and PADR_ROADMAP.md\n" +
					"  ✓ Execution time and desktop shortcut\n",
			)
			contentBox.Objects = []fyne.CanvasObject{container.NewVBox(welcomeLabel, descLabel)}

		case 2:
			// GitHub Detection
			acc := github.DetectAccount(context.Background())
			statusText := "🔴 No GitHub connection detected."
			if acc.LoggedIn {
				statusText = fmt.Sprintf("🟢 Connected as: %s\nEmail: %s\nMethod: %s",
					acc.Username, acc.Email, acc.Method)
			}
			githubInfoLabel.SetText(statusText)

			contentBox.Objects = []fyne.CanvasObject{
				container.NewVBox(
					widget.NewLabelWithStyle("Verify Your GitHub Account", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabel("PADR uses your native Windows Git credentials so contributions appear on your GitHub profile:"),
					widget.NewSeparator(),
					githubInfoLabel,
					widget.NewSeparator(),
					widget.NewLabel("All autonomous commits will be authored by this account."),
				),
			}

		case 3:
			// Model Provider
			contentBox.Objects = []fyne.CanvasObject{
				container.NewVBox(
					widget.NewLabelWithStyle("Configure Default AI Provider", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabel("Choose your primary model provider. You can configure fallback models later:"),
					container.NewHBox(widget.NewLabel("Provider:"), providerSelect),
					container.NewHBox(widget.NewLabel("API Key: "), apiKeyEntry),
					widget.NewSeparator(),
					widget.NewLabel("Tip: If using Ollama, ensure Ollama is running at http://localhost:11434 (API key not required)."),
				),
			}

		case 4:
			// Repository Selection
			contentBox.Objects = []fyne.CanvasObject{
				container.NewVBox(
					widget.NewLabelWithStyle("Select Target Repository", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabel("Enter the project name and local folder path:"),
					container.NewHBox(widget.NewLabel("Project Name:"), repoNameEntry),
					container.NewHBox(widget.NewLabel("Folder Path: "), repoPathEntry),
					widget.NewSeparator(),
					widget.NewLabel("PADR will automatically create 'PADR_ROADMAP.md' in this repository\n" +
						"as the isolated task list for autonomous development."),
				),
			}

		case 5:
			// Schedule and Shortcuts
			contentBox.Objects = []fyne.CanvasObject{
				container.NewVBox(
					widget.NewLabelWithStyle("Execution Schedule & System Integration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabel("When should PADR run autonomous tasks?"),
					container.NewHBox(widget.NewLabel("Daily Run Time:"), timeSelect),
					widget.NewSeparator(),
					widget.NewLabelWithStyle("Desktop & Tray Shortcuts:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					createShortcutCheck,
					startupCheck,
				),
			}
		}

		contentBox.Refresh()
	}

	backBtn.OnTapped = func() {
		if currentStep > 1 {
			currentStep--
			updateStep()
		}
	}

	nextBtn.OnTapped = func() {
		if currentStep < totalSteps {
			currentStep++
			updateStep()
			return
		}

		// FINISH ACTION
		_ = config.InitPadrHome()
		globalCfg, _ := config.LoadGlobalConfig()

		// Save provider if API key entered
		if apiKeyEntry.Text != "" {
			switch providerSelect.Selected {
			case "groq":
				os.Setenv("GROQ_API_KEY", apiKeyEntry.Text)
			case "google (gemini)":
				os.Setenv("GEMINI_API_KEY", apiKeyEntry.Text)
			case "openrouter":
				os.Setenv("OPENROUTER_API_KEY", apiKeyEntry.Text)
			}
		}

		// Save Repository
		if repoPathEntry.Text != "" && repoNameEntry.Text != "" {
			absPath, _ := filepath.Abs(repoPathEntry.Text)
			proj := config.DefaultProjectConfig(repoNameEntry.Text, absPath)
			proj.Schedule.DailyAt = timeSelect.Selected
			proj.Schedule.Cron = fmt.Sprintf("0 %s * * *", timeSelect.Selected[:2])

			_ = config.SaveProjectConfig(absPath, proj)
			_ = config.RegisterProject(proj)

			// Create initial PADR_ROADMAP.md if not exists
			padrRoadmap := filepath.Join(absPath, "PADR_ROADMAP.md")
			if _, err := os.Stat(padrRoadmap); os.IsNotExist(err) {
				initialRoadmap := fmt.Sprintf("# %s — Autonomous Development Roadmap\n\n"+
					"This file is dedicated to PADR autonomous development tasks.\n\n"+
					"## Backlog\n"+
					"- [ ] Initial project optimization and cleanup\n"+
					"- [ ] Add test coverage for core components\n", proj.Name)
				_ = os.WriteFile(padrRoadmap, []byte(initialRoadmap), 0644)
			}

			// Install to Windows Task Scheduler
			sched := scheduler.NewScheduler()
			exePath, _ := os.Executable()
			_ = sched.Install(context.Background(), proj.Name, timeSelect.Selected, exePath)
		}

		_ = config.SaveGlobalConfig(globalCfg)

		// Create Desktop Shortcut if checked
		if createShortcutCheck.Checked {
			_ = CreateDesktopShortcut()
		}

		// Create Windows Startup Shortcut if checked
		if startupCheck.Checked {
			_ = CreateStartupShortcut()
		}

		dialog.ShowInformation("Setup Complete!", "PADR has been successfully configured and desktop shortcuts created!", parent)
		wizardWin.Close()
		if onComplete != nil {
			onComplete()
		}
	}

	updateStep()

	bottomNav := container.NewHBox(
		backBtn,
		layout.NewSpacer(),
		nextBtn,
	)

	wizardLayout := container.NewBorder(
		stepTitle,
		bottomNav,
		nil,
		nil,
		contentBox,
	)

	wizardWin.SetContent(wizardLayout)
	wizardWin.Show()
}
