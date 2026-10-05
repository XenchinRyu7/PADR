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

// ShowSetupWizard opens a clean step-by-step onboarding wizard
func ShowSetupWizard(parent fyne.Window, onComplete func()) {
	wizardWin := fyne.CurrentApp().NewWindow("PADR — Quick Setup Wizard")
	wizardWin.Resize(fyne.NewSize(620, 500))
	wizardWin.SetIcon(GetAppIcon())

	currentStep := 1
	const totalSteps = 5

	stepTitle := widget.NewLabelWithStyle("Step 1 of 5: Welcome", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	contentBox := container.NewStack()

	// Form state
	githubInfoLabel := widget.NewLabel("Verifying GitHub credentials...")

	providerSelect := widget.NewSelect([]string{"groq", "google (gemini)", "openrouter", "ollama (local)"}, nil)
	providerSelect.SetSelected("groq")
	apiKeyEntry := widget.NewPasswordEntry()
	apiKeyEntry.SetPlaceHolder("API Key (leave blank for local Ollama)")

	repoNameEntry := widget.NewEntry()
	repoNameEntry.SetText("my-project")
	repoPathEntry := widget.NewEntry()
	if cwd, err := os.Getwd(); err == nil {
		repoPathEntry.SetText(cwd)
	}

	timeSelect := widget.NewSelect([]string{"08:00", "09:00", "12:00", "15:00", "18:00", "21:00"}, nil)
	timeSelect.SetSelected("09:00")
	createShortcutCheck := widget.NewCheck("Create Desktop Shortcut (PADR.lnk)", nil)
	createShortcutCheck.SetChecked(true)
	startupCheck := widget.NewCheck("Launch in System Tray on Windows Startup", nil)
	startupCheck.SetChecked(true)

	backBtn := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), nil)
	nextBtn := widget.NewButtonWithIcon("Continue", theme.NavigateNextIcon(), nil)

	var updateStep func()

	updateStep = func() {
		stepTitle.SetText(fmt.Sprintf("Step %d of %d", currentStep, totalSteps))
		backBtn.Enable()
		if currentStep == 1 {
			backBtn.Disable()
		}

		if currentStep == totalSteps {
			nextBtn.SetText("Finish Setup")
			nextBtn.SetIcon(theme.ConfirmIcon())
		} else {
			nextBtn.SetText("Continue")
			nextBtn.SetIcon(theme.NavigateNextIcon())
		}

		switch currentStep {
		case 1:
			welcomeLabel := widget.NewLabelWithStyle("Personal Autonomous Development Runner", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
			descLabel := widget.NewLabel(
				"PADR operates as a background development worker on your machine,\n" +
					"autonomously completing roadmap tasks from your repositories\n" +
					"on a scheduled window with automated verification and git commits.\n\n" +
					"This setup will guide you through:\n" +
					"  1. Git & GitHub identity verification\n" +
					"  2. Primary AI provider configuration\n" +
					"  3. First repository registration & PADR_ROADMAP.md creation\n" +
					"  4. Daily schedule and desktop integration\n",
			)
			contentBox.Objects = []fyne.CanvasObject{container.NewVBox(welcomeLabel, descLabel)}

		case 2:
			acc := github.DetectAccount(context.Background())
			statusText := "No GitHub connection detected."
			if acc.LoggedIn {
				statusText = fmt.Sprintf("Account: %s\nEmail: %s\nAuth Method: %s",
					acc.Username, acc.Email, acc.Method)
			}
			githubInfoLabel.SetText(statusText)

			contentBox.Objects = []fyne.CanvasObject{
				container.NewVBox(
					widget.NewLabelWithStyle("GitHub Identity", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabel("PADR automatically identifies your local Git and GitHub CLI credentials:"),
					widget.NewSeparator(),
					githubInfoLabel,
					widget.NewSeparator(),
					widget.NewLabel("All autonomous commits will be attributed to this verified profile."),
				),
			}

		case 3:
			contentBox.Objects = []fyne.CanvasObject{
				container.NewVBox(
					widget.NewLabelWithStyle("Primary AI Provider", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabel("Select your primary LLM provider. Fallback providers can be added later:"),
					container.NewHBox(widget.NewLabel("Provider:"), providerSelect),
					container.NewHBox(widget.NewLabel("API Key: "), apiKeyEntry),
					widget.NewSeparator(),
					widget.NewLabel("Note: For Ollama, ensure localhost:11434 is active (no API key needed)."),
				),
			}

		case 4:
			contentBox.Objects = []fyne.CanvasObject{
				container.NewVBox(
					widget.NewLabelWithStyle("Target Repository", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabel("Specify your project repository name and local folder path:"),
					container.NewHBox(widget.NewLabel("Project Name:"), repoNameEntry),
					container.NewHBox(widget.NewLabel("Folder Path: "), repoPathEntry),
					widget.NewSeparator(),
					widget.NewLabel("PADR will generate 'PADR_ROADMAP.md' in this folder as the isolated task list."),
				),
			}

		case 5:
			contentBox.Objects = []fyne.CanvasObject{
				container.NewVBox(
					widget.NewLabelWithStyle("Execution Window & Shortcuts", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
					widget.NewLabel("Select the daily schedule for autonomous development runs:"),
					container.NewHBox(widget.NewLabel("Daily Trigger:"), timeSelect),
					widget.NewSeparator(),
					widget.NewLabelWithStyle("System Integration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
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

		if repoPathEntry.Text != "" && repoNameEntry.Text != "" {
			absPath, _ := filepath.Abs(repoPathEntry.Text)
			proj := config.DefaultProjectConfig(repoNameEntry.Text, absPath)
			proj.Schedule.DailyAt = timeSelect.Selected
			proj.Schedule.Cron = fmt.Sprintf("0 %s * * *", timeSelect.Selected[:2])

			_ = config.SaveProjectConfig(absPath, proj)
			_ = config.RegisterProject(proj)

			padrRoadmap := filepath.Join(absPath, "PADR_ROADMAP.md")
			if _, err := os.Stat(padrRoadmap); os.IsNotExist(err) {
				initialRoadmap := fmt.Sprintf("# %s — Autonomous Development Roadmap\n\n"+
					"This file defines the isolated backlog for PADR autonomous runner.\n\n"+
					"## Tasks\n"+
					"- [ ] Initial project setup and verification\n"+
					"- [ ] Add test suite validation\n", proj.Name)
				_ = os.WriteFile(padrRoadmap, []byte(initialRoadmap), 0644)
			}

			sched := scheduler.NewScheduler()
			exePath, _ := os.Executable()
			_ = sched.Install(context.Background(), proj.Name, timeSelect.Selected, exePath)
		}

		_ = config.SaveGlobalConfig(globalCfg)

		if createShortcutCheck.Checked {
			_ = CreateDesktopShortcut()
		}

		if startupCheck.Checked {
			_ = CreateStartupShortcut()
		}

		dialog.ShowInformation("Setup Complete", "PADR has been configured successfully.", parent)
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
