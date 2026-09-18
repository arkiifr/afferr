package tui

import "strings"
func renderSidebar(t Theme, width int, values []string) string { body := []string{"SESSIONS", "  current", "", "FILES"}; body = append(body, values...); return t.Panel().Width(max(1, width-2)).Height(20).Render(strings.Join(body, "\n")) }
func renderChat(t Theme, width int, values []string) string { if len(values) == 0 { values = []string{"Affer", "", "Describe the change you want to make.", "Enter sends · Ctrl+P command palette"} }; return t.Panel().Width(max(1, width-2)).Height(20).Render(strings.Join(values, "\n")) }
func renderRight(t Theme, width int, values []string) string { if len(values) == 0 { values = []string{"DIFF", "No changes yet", "", "TODO", "  ready"} }; return t.Panel().Width(max(1, width-2)).Height(20).Render(strings.Join(values, "\n")) }
func max(a, b int) int { if a > b { return a }; return b }
