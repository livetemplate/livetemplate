package ui

import (
	"strings"
)

// HelpItem represents a single help entry
type HelpItem struct {
	Key         string
	Description string
}

// HelpSection represents a group of help items
type HelpSection struct {
	Title string
	Items []HelpItem
}

// RenderHelp renders a help overlay with the given sections
func RenderHelp(sections []HelpSection, width int) string {
	var b strings.Builder

	// Calculate content width (leave room for border and padding)
	contentWidth := width - 6
	if contentWidth < 40 {
		contentWidth = 40
	}

	b.WriteString(HelpTitleStyle.Render("Help"))
	b.WriteString("\n\n")

	for i, section := range sections {
		if section.Title != "" {
			b.WriteString(HelpSectionStyle.Render(section.Title))
			b.WriteString("\n")
		}

		for _, item := range section.Items {
			key := HelpKeyStyle.Render(item.Key)
			desc := HelpDescStyle.Render(item.Description)
			b.WriteString("  " + key + "  " + desc + "\n")
		}

		// Add spacing between sections
		if i < len(sections)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(HintStyle.Render("Press ? or Esc to close"))

	// Wrap in a box
	content := b.String()
	return OverlayStyle.Width(contentWidth).Render(content)
}

// GetNewAppHelp returns help content for the new app wizard
func GetNewAppHelp() []HelpSection {
	return []HelpSection{
		{
			Title: "Keyboard Shortcuts",
			Items: []HelpItem{
				{"Tab/Shift+Tab", "Navigate between fields"},
				{"Enter", "Continue to next step"},
				{"Ctrl+C", "Cancel and exit"},
				{"?", "Toggle this help"},
			},
		},
		{
			Title: "App Name",
			Items: []HelpItem{
				{"Valid:", "Letters, digits, hyphens, underscores"},
				{"Examples:", "myapp, my-app, my_app"},
				{"Note:", "Hyphens → underscores in package names"},
			},
		},
		{
			Title: "Module Path",
			Items: []HelpItem{
				{"Format:", "domain.com/user/repository"},
				{"GitHub:", "github.com/username/appname"},
				{"GitLab:", "gitlab.com/username/appname"},
			},
		},
	}
}

// GetGenResourceHelp returns help content for the resource generator wizard
func GetGenResourceHelp() []HelpSection {
	return []HelpSection{
		{
			Title: "Keyboard Shortcuts",
			Items: []HelpItem{
				{"Enter", "Add field / Continue"},
				{"Ctrl+D", "Delete last field"},
				{"Esc", "Finish field entry"},
				{"Ctrl+C", "Cancel and exit"},
				{"?", "Toggle this help"},
			},
		},
		{
			Title: "Type Inference Patterns",
			Items: []HelpItem{
				{"string:", "name, email, title, url, username, ..."},
				{"int:", "age, count, quantity, year, *_id, ..."},
				{"float:", "price, amount, total, latitude, ..."},
				{"bool:", "enabled, active, is_*, has_*, can_*"},
				{"time:", "created_at, updated_at, *_at, *_date"},
			},
		},
		{
			Title: "Field Format",
			Items: []HelpItem{
				{"fieldname", "Type inferred from name"},
				{"fieldname:type", "Explicit type (string/int/bool/float/time)"},
				{"Examples:", "email (→string), age:int, price (→float)"},
			},
		},
		{
			Title: "Auto-Added Fields",
			Items: []HelpItem{
				{"id", "TEXT PRIMARY KEY (auto)"},
				{"created_at", "DATETIME (auto)"},
			},
		},
	}
}

// GetGenViewHelp returns help content for the view generator wizard
func GetGenViewHelp() []HelpSection {
	return []HelpSection{
		{
			Title: "Keyboard Shortcuts",
			Items: []HelpItem{
				{"Enter", "Continue to next step"},
				{"Ctrl+C", "Cancel and exit"},
				{"?", "Toggle this help"},
			},
		},
		{
			Title: "View Name",
			Items: []HelpItem{
				{"Valid:", "Letters, digits, underscores"},
				{"Examples:", "dashboard, user_profile, admin_panel"},
				{"Note:", "Creates view-only handler (no database)"},
			},
		},
		{
			Title: "What Gets Created",
			Items: []HelpItem{
				{"Handler:", "internal/app/{view}/{view}.go"},
				{"Template:", "internal/app/{view}/{view}.tmpl"},
				{"Tests:", "WebSocket and E2E test files"},
				{"Route:", "Auto-injected into main.go"},
			},
		},
	}
}

// RenderHelpOverlay renders the help overlay on top of the current screen
func RenderHelpOverlay(background string, helpSections []HelpSection, termWidth, termHeight int) string {
	// Render help content
	helpContent := RenderHelp(helpSections, termWidth)

	// Get help box dimensions
	helpLines := strings.Split(helpContent, "\n")
	helpHeight := len(helpLines)
	helpWidth := 0
	for _, line := range helpLines {
		// Strip ANSI codes for width calculation
		plainLine := stripAnsi(line)
		if len(plainLine) > helpWidth {
			helpWidth = len(plainLine)
		}
	}

	// Calculate centering
	backgroundLines := strings.Split(background, "\n")
	bgHeight := len(backgroundLines)

	// Vertical centering
	topPadding := (bgHeight - helpHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}

	// Horizontal centering (not yet implemented)
	// leftPadding := (termWidth - helpWidth) / 2

	// Build overlay
	var result strings.Builder

	// Add background lines with overlay
	for i := 0; i < bgHeight; i++ {
		if i < len(backgroundLines) {
			result.WriteString(backgroundLines[i])
		}
		result.WriteString("\n")
	}

	// Position help overlay (simple approach - just append at bottom with spacing)
	// For true overlay, we'd need to overwrite specific line positions
	result.WriteString("\n")
	for i := 0; i < topPadding && i < 3; i++ {
		result.WriteString("\n")
	}

	// Add help content
	result.WriteString(helpContent)

	return result.String()
}

// stripAnsi removes ANSI escape codes for length calculation
func stripAnsi(s string) string {
	// Simple ANSI stripper - looks for ESC [ ... m sequences
	var result strings.Builder
	inEscape := false
	escapeStart := false

	for _, ch := range s {
		if ch == '\x1b' {
			escapeStart = true
			continue
		}
		if escapeStart && ch == '[' {
			inEscape = true
			escapeStart = false
			continue
		}
		if inEscape {
			if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(ch)
	}

	return result.String()
}
