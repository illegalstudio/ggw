package ui

import "github.com/charmbracelet/lipgloss"

var (
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	Success = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22C55E"))

	Error = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444"))

	Info = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3B82F6"))

	Muted = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	Branch = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B")).
		Bold(true)

	Path = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true)
)
