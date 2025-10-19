package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("86")  // Cyan
	successColor   = lipgloss.Color("42")  // Green
	errorColor     = lipgloss.Color("196") // Red
	warningColor   = lipgloss.Color("226") // Yellow
	subtleColor    = lipgloss.Color("241") // Gray
	highlightColor = lipgloss.Color("212") // Pink

	// Styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginTop(1).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			MarginBottom(1)

	PromptStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	InputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))

	SuccessStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	HintStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Italic(true)

	HelpStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			MarginTop(1)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true)

	FieldLabelStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	FieldValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))

	ListItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(highlightColor).
				Bold(true).
				PaddingLeft(1)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	ConfirmBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(successColor).
			Padding(1, 2)

	// Validation styles
	WarningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)

	ValidationOkStyle = lipgloss.NewStyle().
				Foreground(successColor)

	// Help overlay styles
	HelpTitleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Underline(true).
			Align(lipgloss.Center)

	HelpSectionStyle = lipgloss.NewStyle().
				Foreground(highlightColor).
				Bold(true).
				MarginTop(1)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Width(16)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	OverlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			BorderBackground(lipgloss.Color("235")).
			Background(lipgloss.Color("235")).
			Padding(2, 3).
			Align(lipgloss.Center)
)
