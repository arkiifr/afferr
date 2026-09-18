package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct { Name string; Background lipgloss.Color; Foreground lipgloss.Color; Muted lipgloss.Color; Accent lipgloss.Color; Border lipgloss.Color; Success lipgloss.Color; Warning lipgloss.Color }
func ThemeFor(name string) Theme { switch name { case "light": return Theme{"light", "#f7f7f8", "#202124", "#6b7280", "#2563eb", "#cbd5e1", "#15803d", "#b45309"}; case "catppuccin": return Theme{"catppuccin", "#1e1e2e", "#cdd6f4", "#9399b2", "#cba6f7", "#45475a", "#a6e3a1", "#f9e2af"}; case "tokyo-night": return Theme{"tokyo-night", "#16161e", "#c0caf5", "#565f89", "#7aa2f7", "#3b4261", "#9ece6a", "#e0af68"}; default: return Theme{"dark", "#0d1117", "#e6edf3", "#8b949e", "#58a6ff", "#30363d", "#3fb950", "#d29922"} } }
func (t Theme) Panel() lipgloss.Style { return lipgloss.NewStyle().Background(t.Background).Foreground(t.Foreground).Border(lipgloss.RoundedBorder()).BorderForeground(t.Border).Padding(0, 1) }
