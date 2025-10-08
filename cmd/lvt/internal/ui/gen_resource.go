package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/livefir/livetemplate/cmd/lvt/internal/generator"
	"github.com/livefir/livetemplate/cmd/lvt/internal/parser"
)

type fieldEntry struct {
	name     string
	typ      string
	inferred bool
}

type genResourceModel struct {
	textInput       textinput.Model
	stage           int // 0: resource name, 1: add fields, 2: CSS framework, 3: confirm, 4: generating, 5: success
	resourceName    string
	fields          []fieldEntry
	cssFramework    string
	cssCursor       int // cursor position for CSS framework selection
	moduleName      string
	basePath        string
	err             error
	validationError string
	validationWarn  string
	showHelp        bool
	termWidth       int
	termHeight      int
}

func (m genResourceModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m genResourceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				if m.stage == 1 && len(m.fields) > 0 {
					// During field entry, Esc moves to confirm
					m.stage = 2
					return m, nil
				}
				return m, tea.Quit
			}

		case tea.KeyEnter:
			if m.stage == 0 {
				// Resource name -> Field entry
				m.resourceName = strings.TrimSpace(m.textInput.Value())

				// Validate resource name
				result := IsValidResourceName(m.resourceName)
				if !result.Valid {
					m.validationError = result.Error
					m.validationWarn = result.Warning
					return m, nil
				}

				m.validationError = ""
				m.validationWarn = ""
				m.stage = 1
				m.textInput.Reset()
				m.textInput.Placeholder = "name, email, age:int, ..."
				return m, nil

			} else if m.stage == 1 {
				// Add field or finish
				input := strings.TrimSpace(m.textInput.Value())
				if input == "" {
					// Empty input means finish
					if len(m.fields) == 0 {
						m.validationError = "at least one field required"
						return m, nil
					}
					m.validationError = ""
					m.validationWarn = ""
					m.stage = 2
					return m, nil
				}

				// Parse field
				name, typ := ParseFieldInput(input)
				inferred := !strings.Contains(input, ":")

				// Validate field name
				result := IsValidFieldName(name)
				if !result.Valid {
					m.validationError = result.Error
					m.validationWarn = result.Warning
					return m, nil
				}

				// Check for duplicates
				for _, f := range m.fields {
					if f.name == name {
						m.validationError = fmt.Sprintf("field '%s' already exists", name)
						return m, nil
					}
				}

				m.fields = append(m.fields, fieldEntry{
					name:     name,
					typ:      typ,
					inferred: inferred,
				})
				m.textInput.Reset()
				m.validationError = ""
				m.validationWarn = result.Warning
				return m, nil

			} else if m.stage == 2 {
				// Confirm -> Generate
				m.stage = 3
				return m, m.generateResource

			} else if m.stage == 4 {
				// Success -> Exit
				return m, tea.Quit
			}

		case tea.KeyCtrlD:
			// Delete last field during field entry
			if m.stage == 1 && len(m.fields) > 0 {
				m.fields = m.fields[:len(m.fields)-1]
				m.err = nil
				return m, nil
			}
		}
	}

	if m.stage == 0 || m.stage == 1 {
		// Update text input and validate on each keystroke
		oldValue := m.textInput.Value()
		m.textInput, cmd = m.textInput.Update(msg)
		newValue := m.textInput.Value()

		// Real-time validation if value changed
		if oldValue != newValue && newValue != "" {
			if m.stage == 0 {
				// Validate resource name
				result := IsValidResourceName(newValue)
				if result.Valid {
					m.validationError = ""
					m.validationWarn = result.Warning
				} else {
					m.validationError = result.Error
					m.validationWarn = result.Warning
				}
			} else if m.stage == 1 {
				// Validate field name (parse first)
				name, _ := ParseFieldInput(newValue)
				result := IsValidFieldName(name)
				if result.Valid {
					m.validationError = ""
					m.validationWarn = result.Warning
				} else {
					m.validationError = result.Error
					m.validationWarn = result.Warning
				}
			}
		} else if newValue == "" {
			m.validationError = ""
			m.validationWarn = ""
		}
	}

	return m, cmd
}

func (m genResourceModel) View() string {
	// Handle help overlay
	if m.showHelp {
		background := m.renderContent()
		return RenderHelpOverlay(background, GetGenResourceHelp(), m.termWidth, m.termHeight)
	}

	return m.renderContent()
}

func (m genResourceModel) renderContent() string {
	var b strings.Builder

	switch m.stage {
	case 0: // Resource name input
		b.WriteString(TitleStyle.Render("Generate CRUD Resource"))
		b.WriteString("\n\n")
		b.WriteString(PromptStyle.Render("Resource name:") + " " + m.textInput.View())

		// Show real-time validation feedback
		if m.validationError != "" {
			b.WriteString("\n")
			b.WriteString(ErrorStyle.Render("✗ " + m.validationError))
			if m.validationWarn != "" {
				b.WriteString("\n")
				b.WriteString(WarningStyle.Render("  " + m.validationWarn))
			}
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

	case 1: // Field builder
		resourceNameLower := strings.ToLower(m.resourceName)
		b.WriteString(TitleStyle.Render(fmt.Sprintf("Fields for '%s' resource", resourceNameLower)))
		b.WriteString("\n\n")

		// Show existing fields
		if len(m.fields) > 0 {
			for i, f := range m.fields {
				marker := "✓"
				typeDisplay := f.typ
				if f.inferred {
					typeDisplay = HintStyle.Render(f.typ + " (inferred)")
				}
				b.WriteString(fmt.Sprintf("  %d. %-15s [%s] %s\n", i+1, f.name, typeDisplay, marker))
			}
			b.WriteString("\n")
		}

		// Input for next field
		b.WriteString(PromptStyle.Render("Add field:") + " " + m.textInput.View())

		// Show real-time validation feedback
		if m.validationError != "" {
			b.WriteString("\n")
			b.WriteString(ErrorStyle.Render("✗ " + m.validationError))
			if m.validationWarn != "" {
				b.WriteString("\n")
				b.WriteString(WarningStyle.Render("  " + m.validationWarn))
			}
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

		b.WriteString(HintStyle.Render("💡 fieldname (type inferred) or name:type"))
		b.WriteString("\n")
		b.WriteString(HintStyle.Render("   Enter on empty: finish • Ctrl+D: delete last • ?: help"))

	case 2: // Confirmation
		resourceNameLower := strings.ToLower(m.resourceName)
		b.WriteString(TitleStyle.Render("Review & Generate"))
		b.WriteString("\n\n")

		content := fmt.Sprintf("Resource: %s\n\n", HighlightStyle.Render(resourceNameLower))
		content += "Fields:\n"
		for _, f := range m.fields {
			if f.inferred {
				content += fmt.Sprintf("  • %s: %s %s\n", f.name, f.typ, HintStyle.Render("(inferred)"))
			} else {
				content += fmt.Sprintf("  • %s: %s\n", f.name, f.typ)
			}
		}
		content += "\nAuto-added:\n"
		content += "  • id: INTEGER PRIMARY KEY\n"
		content += "  • created_at: DATETIME\n\n"
		content += "Will create:\n"
		content += fmt.Sprintf("  • internal/app/%s/%s.go\n", resourceNameLower, resourceNameLower)
		content += fmt.Sprintf("  • internal/app/%s/%s.tmpl\n", resourceNameLower, resourceNameLower)
		content += "  • Database schema and queries\n"
		content += "  • Auto-inject route"

		b.WriteString(BoxStyle.Render(content))
		b.WriteString("\n\n")
		b.WriteString(HintStyle.Render("Enter: generate • Esc: cancel • ?: help"))

	case 3: // Generating
		b.WriteString(TitleStyle.Render("Generating..."))
		b.WriteString("\n\n")
		b.WriteString(HintStyle.Render("Please wait..."))

	case 4: // Success
		resourceNameLower := strings.ToLower(m.resourceName)
		b.WriteString(SuccessStyle.Render("✅ Resource generated successfully!"))
		b.WriteString("\n\n")

		content := fmt.Sprintf("Resource '%s' is ready\n\n", HighlightStyle.Render(resourceNameLower))
		content += "Registration auto-injected:\n"
		content += fmt.Sprintf("  router.Register(\"%s\", %s.NewStore(queries))\n\n", resourceNameLower, resourceNameLower)
		content += "Next steps:\n"
		content += "  1. Run migration:\n"
		content += "     lvt migration up\n"
		content += "  2. Run your app"

		b.WriteString(BoxStyle.Render(content))
		b.WriteString("\n\n")
		b.WriteString(HintStyle.Render("Press Enter to exit"))
	}

	return b.String()
}

func (m genResourceModel) generateResource() tea.Msg {
	resourceNameLower := strings.ToLower(m.resourceName)

	// Convert fieldEntry to parser.Field
	fields := make([]parser.Field, len(m.fields))
	for i, f := range m.fields {
		fields[i] = parser.Field{
			Name: f.name,
			Type: f.typ,
		}
	}

	// Use default CSS framework for now (TODO: add interactive selection)
	cssFramework := "tailwind"
	if m.cssFramework != "" {
		cssFramework = m.cssFramework
	}

	// Use default app mode (multi-page)
	appMode := "multi"

	if err := generator.GenerateResource(m.basePath, m.moduleName, resourceNameLower, fields, cssFramework, appMode); err != nil {
		m.err = err
		m.stage = 1
		return m
	}

	m.stage = 4
	return m
}

func GenResourceInteractive() error {
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
	ti.Placeholder = "users, posts, products, ..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40

	m := genResourceModel{
		textInput:  ti,
		stage:      0,
		fields:     []fieldEntry{},
		moduleName: moduleName,
		basePath:   basePath,
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
