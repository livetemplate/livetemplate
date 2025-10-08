package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/livefir/livetemplate/cmd/lvt/internal/generator"
)

type newAppModel struct {
	inputs          []textinput.Model
	focusIndex      int
	stage           int // 0: inputs, 1: confirm, 2: generating, 3: success
	appName         string
	modulePath      string
	err             error
	appNameError    string
	appNameWarn     string
	modulePathError string
	modulePathWarn  string
	showHelp        bool
	termWidth       int
	termHeight      int
}

func (m newAppModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m newAppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height

	case tea.KeyMsg:
		// Handle help toggle
		if msg.String() == "?" {
			m.showHelp = !m.showHelp
			return m, nil
		}

		// Close help on Esc when help is showing
		if m.showHelp && msg.Type == tea.KeyEsc {
			m.showHelp = false
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEsc:
			if !m.showHelp {
				return m, tea.Quit
			}

		case tea.KeyEnter:
			if m.stage == 0 {
				// Input stage
				if m.focusIndex == 0 {
					// Move to module input
					m.appName = strings.TrimSpace(m.inputs[0].Value())

					// Validate app name
					result := IsValidAppName(m.appName)
					if !result.Valid {
						m.appNameError = result.Error
						m.appNameWarn = result.Warning
						return m, nil
					}

					m.appNameError = ""
					m.appNameWarn = result.Warning
					m.focusIndex = 1
					m.inputs[0].Blur()
					m.inputs[1].Focus()
					return m, textinput.Blink
				} else {
					// Move to confirm stage
					m.modulePath = strings.TrimSpace(m.inputs[1].Value())

					// Validate module path
					result := IsValidModulePath(m.modulePath)
					if !result.Valid {
						m.modulePathError = result.Error
						m.modulePathWarn = result.Warning
						return m, nil
					}

					m.modulePathError = ""
					m.modulePathWarn = result.Warning
					m.stage = 1
					return m, nil
				}
			} else if m.stage == 1 {
				// Confirm -> Generate
				m.stage = 2
				return m, m.generateApp
			} else if m.stage == 3 {
				// Success -> Exit
				return m, tea.Quit
			}

		case tea.KeyTab, tea.KeyShiftTab:
			if m.stage == 0 {
				// Navigate between inputs
				if msg.Type == tea.KeyTab {
					m.focusIndex++
					if m.focusIndex > 1 {
						m.focusIndex = 0
					}
				} else {
					m.focusIndex--
					if m.focusIndex < 0 {
						m.focusIndex = 1
					}
				}

				for i := 0; i < len(m.inputs); i++ {
					if i == m.focusIndex {
						m.inputs[i].Focus()
					} else {
						m.inputs[i].Blur()
					}
				}

				return m, textinput.Blink
			}
		}
	}

	// Update the focused input
	if m.stage == 0 {
		var cmd tea.Cmd
		oldValue := m.inputs[m.focusIndex].Value()
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
		newValue := m.inputs[m.focusIndex].Value()

		// Real-time validation if value changed
		if oldValue != newValue && newValue != "" {
			if m.focusIndex == 0 {
				// Validate app name
				result := IsValidAppName(newValue)
				if result.Valid {
					m.appNameError = ""
					m.appNameWarn = result.Warning
				} else {
					m.appNameError = result.Error
					m.appNameWarn = result.Warning
				}
			} else {
				// Validate module path
				result := IsValidModulePath(newValue)
				if result.Valid {
					m.modulePathError = ""
					m.modulePathWarn = result.Warning
				} else {
					m.modulePathError = result.Error
					m.modulePathWarn = result.Warning
				}
			}
		} else if newValue == "" {
			if m.focusIndex == 0 {
				m.appNameError = ""
				m.appNameWarn = ""
			} else {
				m.modulePathError = ""
				m.modulePathWarn = ""
			}
		}

		return m, cmd
	}

	return m, nil
}

func (m newAppModel) View() string {
	// Handle help overlay
	if m.showHelp {
		background := m.renderContent()
		return RenderHelpOverlay(background, GetNewAppHelp(), m.termWidth, m.termHeight)
	}

	return m.renderContent()
}

func (m newAppModel) renderContent() string {
	var b strings.Builder

	switch m.stage {
	case 0: // Input stage
		b.WriteString(TitleStyle.Render("Create New LiveTemplate App"))
		b.WriteString("\n\n")

		// App name input
		if m.focusIndex == 0 {
			b.WriteString(PromptStyle.Render("App name:") + " " + m.inputs[0].View())
		} else {
			b.WriteString(HintStyle.Render("App name:") + " " + m.inputs[0].View())
		}

		// Show validation feedback for app name
		if m.appNameError != "" {
			b.WriteString("\n")
			b.WriteString(ErrorStyle.Render("✗ " + m.appNameError))
			if m.appNameWarn != "" {
				b.WriteString("\n")
				b.WriteString(WarningStyle.Render("  " + m.appNameWarn))
			}
		} else if m.appNameWarn != "" {
			b.WriteString("\n")
			b.WriteString(WarningStyle.Render("⚠ " + m.appNameWarn))
		} else if m.inputs[0].Value() != "" && m.focusIndex == 0 {
			b.WriteString("\n")
			b.WriteString(ValidationOkStyle.Render("✓ Valid"))
		}

		b.WriteString("\n")
		b.WriteString(HintStyle.Render("  ↑ Current directory name") + "\n\n")

		// Module path input
		if m.focusIndex == 1 {
			b.WriteString(PromptStyle.Render("Module path:") + " " + m.inputs[1].View())
		} else {
			b.WriteString(HintStyle.Render("Module path:") + " " + m.inputs[1].View())
		}

		// Show validation feedback for module path
		if m.modulePathError != "" {
			b.WriteString("\n")
			b.WriteString(ErrorStyle.Render("✗ " + m.modulePathError))
			if m.modulePathWarn != "" {
				b.WriteString("\n")
				b.WriteString(WarningStyle.Render("  " + m.modulePathWarn))
			}
		} else if m.modulePathWarn != "" {
			b.WriteString("\n")
			b.WriteString(WarningStyle.Render("⚠ " + m.modulePathWarn))
		} else if m.inputs[1].Value() != "" && m.focusIndex == 1 {
			b.WriteString("\n")
			b.WriteString(ValidationOkStyle.Render("✓ Valid"))
		}

		b.WriteString("\n")
		b.WriteString(HintStyle.Render("  ↑ Go module import path") + "\n\n")

		if m.err != nil {
			b.WriteString(ErrorStyle.Render("Error: "+m.err.Error()) + "\n\n")
		}

		b.WriteString(HintStyle.Render("Tab: next • Enter: continue • Esc: cancel • ?: help"))

	case 1: // Confirm stage
		b.WriteString(TitleStyle.Render("Review & Create"))
		b.WriteString("\n\n")
		b.WriteString(BoxStyle.Render(
			fmt.Sprintf("App name: %s\n", HighlightStyle.Render(m.appName)) +
				fmt.Sprintf("Module: %s\n\n", HighlightStyle.Render(m.modulePath)) +
				"Will create:\n" +
				fmt.Sprintf("  • %s/ (project directory)\n", m.appName) +
				"  • Complete LiveTemplate app structure\n" +
				"  • Ready-to-run example",
		))
		b.WriteString("\n\n")
		b.WriteString(HintStyle.Render("Enter: create • Esc: cancel • ?: help"))

	case 2: // Generating
		b.WriteString(TitleStyle.Render("Creating App..."))
		b.WriteString("\n\n")
		b.WriteString(HintStyle.Render("Please wait..."))

	case 3: // Success
		b.WriteString(SuccessStyle.Render("✅ App created successfully!"))
		b.WriteString("\n\n")
		b.WriteString(BoxStyle.Render(
			fmt.Sprintf("App '%s' is ready\n\n", HighlightStyle.Render(m.appName)) +
				"Next steps:\n" +
				fmt.Sprintf("  1. cd %s\n", m.appName) +
				"  2. lvt gen users name:string email:string\n" +
				"  3. lvt migration up\n" +
				fmt.Sprintf("  4. go run cmd/%s/main.go", m.appName),
		))
		b.WriteString("\n\n")
		b.WriteString(HintStyle.Render("Press Enter to exit"))
	}

	return b.String()
}

func (m newAppModel) generateApp() tea.Msg {
	if err := generator.GenerateApp(m.appName, m.appName, false); err != nil { // false = production mode (use CDN)
		m.err = err
		m.stage = 0
		m.focusIndex = 0
		m.inputs[0].Focus()
		m.inputs[1].Blur()
		return m
	}

	// Install dependencies
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = m.appName
	_ = cmd.Run() // Silently install, errors are not critical

	m.stage = 3
	return m
}

func NewAppInteractive() error {
	// Detect current directory name as default app name
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defaultAppName := filepath.Base(currentDir)

	// Suggest module path from git config or use default
	defaultModule := suggestModulePath(defaultAppName)

	// Create inputs
	appNameInput := textinput.New()
	appNameInput.Placeholder = defaultAppName
	appNameInput.Focus()
	appNameInput.CharLimit = 50
	appNameInput.Width = 40

	moduleInput := textinput.New()
	moduleInput.Placeholder = defaultModule
	moduleInput.CharLimit = 100
	moduleInput.Width = 40

	m := newAppModel{
		inputs:     []textinput.Model{appNameInput, moduleInput},
		focusIndex: 0,
		stage:      0,
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func suggestModulePath(appName string) string {
	// Try to get git remote URL
	if gitRemote := getGitRemote(); gitRemote != "" {
		return gitRemote + "/" + appName
	}

	// Fallback to github.com/user/appname
	if username := os.Getenv("USER"); username != "" {
		return "github.com/" + username + "/" + appName
	}

	return "example.com/" + appName
}

func getGitRemote() string {
	// Try to read .git/config
	data, err := os.ReadFile(".git/config")
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "url = ") {
			url := strings.TrimPrefix(line, "url = ")
			// Parse github.com:user/repo.git or https://github.com/user/repo.git
			if strings.Contains(url, "github.com") {
				url = strings.TrimSuffix(url, ".git")
				if strings.HasPrefix(url, "git@") {
					// git@github.com:user/repo -> github.com/user/repo
					url = strings.Replace(url, "git@", "", 1)
					url = strings.Replace(url, ":", "/", 1)
				} else if strings.HasPrefix(url, "https://") {
					// https://github.com/user/repo -> github.com/user/repo
					url = strings.TrimPrefix(url, "https://")
				}
				return url
			}
		}
	}

	return ""
}
