package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)
func renderStatus(t Theme, width int, status, variant string) string { text := fmt.Sprintf(" %s  ·  variant:%s  ·  context:0%%  ·  cost:$0.00  ·  branch:—  ·  LSP:—", status, variant); return lipgloss.NewStyle().Foreground(t.Muted).Width(width).Render(text) }
