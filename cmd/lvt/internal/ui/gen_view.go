package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/livefir/livetemplate/cmd/lvt/internal/generator"
)

type genViewModel struct {
	textInput       textinput.Model
	stage           int // 0: input, 1: confirm, 2: generating, 3: success
	viewName        string
	moduleName      string
	basePath        string
	err             error
	validationError string
	validationWarn  string
	showHelp        bool
	termWidth       int
	termHeight      int
}

func (m genViewModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m genViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

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
				// Input stage -> Confirm stage
				m.viewName = strings.TrimSpace(m.textInput.Value())

				// Validate
				result := IsValidViewName(m.viewName)
				if !result.Valid {
					m.validationError = result.Error
					m.validationWarn = result.Warning
					return m, nil
				}

				m.validationError = ""
				m.validationWarn = result.Warning
				m.stage = 1
				return m, nil
			} else if m.stage == 1 {
				// Confirm stage -> Generate
				m.stage = 2
				return m, m.generateView
			} else if m.stage == 3 {
				// Success -> Exit
				return m, tea.Quit
			}
		}
	}

	if m.stage == 0 {
		// Update text input and validate on each keystroke
		oldValue := m.textInput.Value()
		m.textInput, cmd = m.textInput.Update(msg)
		newValue := m.textInput.Value()

		// Real-time validation if value changed
		if oldValue != newValue && newValue != "" {
			result := IsValidViewName(newValue)
			if result.Valid {
				m.validationError = ""
				m.validationWarn = result.Warning
			} else {
				m.validationError = result.Error
				m.validationWarn = result.Warning
			}
		} else if newValue == "" {
			m.validationError = ""
			m.validationWarn = ""
		}
	}

	return m, cmd
}

func (m genViewModel) View() string {
	// Handle help overlay
	if m.showHelp {
		background := m.renderContent()
		return RenderHelpOverlay(background, GetGenViewHelp(), m.termWidth, m.termHeight)
	}

	return m.renderContent()
}

func (m genViewModel) renderContent() string {
	var b strings.Builder

	switch m.stage {
	case 0: // Input stage
		b.WriteString(TitleStyle.Render("Generate View"))
		b.WriteString("\n\n")
		b.WriteString(PromptStyle.Render("View name:") + " " + m.textInput.View())

		// Show real-time validation feedback
		if m.validationError != "" {
			b.WriteString("\n")
			b.WriteString(ErrorStyle.Render("✗ " + m.validationError))
		} else if m.validationWarn != "" {
			b.WriteString("\n")
			b.WriteString(WarningStyle.Render("⚠ " + m.validationWarn))
		} else if m.textInput.Value() != "" {
			b.WriteString("\n")
			b.WriteString(ValidationOkStyle.Render("✓ Valid"))
		}

		b.WriteString("\n\n")
		if m.err != nil {
			b.WriteString(ErrorStyle.Render("Error: "+m.err.Error()) + "\n\n")
		}
		b.WriteString(HintStyle.Render("Enter: continue • Esc: cancel • ?: help"))

	case 1: // Confirm stage
		viewNameLower := strings.ToLower(m.viewName)
		b.WriteString(TitleStyle.Render("Review & Generate"))
		b.WriteString("\n\n")
		b.WriteString(BoxStyle.Render(
			fmt.Sprintf("View: %s\n\n", HighlightStyle.Render(viewNameLower)) +
				"Will create:\n" +
				fmt.Sprintf("  • internal/app/%s/%s.go\n", viewNameLower, viewNameLower) +
				fmt.Sprintf("  • internal/app/%s/%s.tmpl\n", viewNameLower, viewNameLower) +
				fmt.Sprintf("  • internal/app/%s/%s_test.go\n", viewNameLower, viewNameLower) +
				fmt.Sprintf("  • internal/app/%s/%s_ws_test.go\n", viewNameLower, viewNameLower) +
				"  • Auto-inject route",
		))
		b.WriteString("\n\n")
		b.WriteString(HintStyle.Render("Enter: generate • Esc: cancel • ?: help"))

	case 2: // Generating
		b.WriteString(TitleStyle.Render("Generating..."))
		b.WriteString("\n\n")
		b.WriteString(HintStyle.Render("Please wait..."))

	case 3: // Success
		viewNameLower := strings.ToLower(m.viewName)
		b.WriteString(SuccessStyle.Render("✅ View generated successfully!"))
		b.WriteString("\n\n")
		b.WriteString(BoxStyle.Render(
			fmt.Sprintf("View '%s' is ready\n\n", HighlightStyle.Render(viewNameLower)) +
				"Registration auto-injected:\n" +
				fmt.Sprintf("  router.Register(\"%s\", %s.NewStore())\n\n", viewNameLower, viewNameLower) +
				"Next steps:\n" +
				fmt.Sprintf("  1. Customize handler: internal/app/%s/%s.go\n", viewNameLower, viewNameLower) +
				fmt.Sprintf("  2. Edit template: internal/app/%s/%s.tmpl\n", viewNameLower, viewNameLower) +
				"  3. Run your app",
		))
		b.WriteString("\n\n")
		b.WriteString(HintStyle.Render("Press Enter to exit"))
	}

	return b.String()
}

func (m genViewModel) generateView() tea.Msg {
	viewNameLower := strings.ToLower(m.viewName)

	// Use default CSS framework for now (TODO: add interactive selection)
	cssFramework := "tailwind"

	if err := generator.GenerateView(m.basePath, m.moduleName, viewNameLower, cssFramework); err != nil {
		m.err = err
		m.stage = 0
		return m
	}

	m.stage = 3
	return m
}

func GenViewInteractive() error {
	// Get module name
	moduleName, err := getModuleName()
	if err != nil {
		return fmt.Errorf("failed to get module name: %w (are you in a Go project?)", err)
	}

	// Get current directory
	basePath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	ti := textinput.New()
	ti.Placeholder = "counter"
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 30

	m := genViewModel{
		textInput:  ti,
		stage:      0,
		moduleName: moduleName,
		basePath:   basePath,
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func getModuleName() (string, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}

	return "", fmt.Errorf("module name not found in go.mod")
}
