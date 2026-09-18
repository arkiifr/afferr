package tui

import "strings"
func renderPalette(t Theme, width, height int) string { lines := []string{"COMMAND PALETTE", "", "/models   choose a model", "/diff     review pending changes", "/stats    usage dashboard", "/connect  configure provider", "/compact  summarize context", "/undo     restore last snapshot", "", "Esc closes"}; return t.Panel().Width(max(1, width-4)).Height(max(1, height-4)).Render(strings.Join(lines, "\n")) }
