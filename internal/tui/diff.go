package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)
func renderDiff(t Theme, width, height int) string { added := lipgloss.NewStyle().Foreground(t.Success); removed := lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149")); lines := []string{"DIFF REVIEW", "", removed.Render("- previous content"), added.Render("+ proposed content"), "", "y approve   n reject   Esc close"}; return t.Panel().Width(max(1, width-4)).Height(max(1, height-4)).Render(strings.Join(lines, "\n")) }
