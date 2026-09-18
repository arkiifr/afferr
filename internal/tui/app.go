package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/arkiifr/afferr/internal/config"
)

type screen int
const ( screenChat screen = iota; screenPicker; screenDiff; screenPalette )

type App struct { cfg config.Config; theme Theme; width, height int; active screen; variant string; input string; messages []string; sidebar, right []string; status string }
func New(cfg config.Config) *App { return &App{cfg: cfg, theme: ThemeFor(cfg.Theme), active: screenChat, variant: "none", status: "ready"} }
func Run(cfg config.Config) error { _, err := tea.NewProgram(New(cfg), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); return err }
func (a *App) Init() tea.Cmd { return nil }
func (a *App) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg: a.width, a.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c": return a, tea.Quit
		case "ctrl+p": a.active = screenPalette
		case "esc": a.active = screenChat
		case "tab": a.variant = "next"; a.status = "variant cycle"
		case "enter": if strings.TrimSpace(a.input) != "" { a.messages = append(a.messages, "> "+a.input); a.input = ""; a.status = "queued" }
		case "backspace": if len(a.input) > 0 { a.input = a.input[:len(a.input)-1] }
		case "alt+1": a.active = screenChat
		case "alt+2": a.active = screenPicker
		case "alt+3": a.active = screenDiff
		default: if len(msg.Runes) > 0 { a.input += string(msg.Runes) }
		}
	}
	return a, nil
}
func (a *App) View() string { if a.width == 0 { return "starting affer…" }; switch a.active { case screenPicker: return renderPicker(a.theme, a.width, a.height); case screenDiff: return renderDiff(a.theme, a.width, a.height); case screenPalette: return renderPalette(a.theme, a.width, a.height) }; return a.chatView() }
func (a *App) chatView() string { left, center, right := paneWidths(a.width); top := lipgloss.JoinHorizontal(lipgloss.Top, renderSidebar(a.theme, left, a.sidebar), renderChat(a.theme, center, a.messages), renderRight(a.theme, right, a.right)); input := a.theme.Panel().Width(a.width-4).Render("▌ "+a.input); status := renderStatus(a.theme, a.width, a.status, a.variant); return lipgloss.JoinVertical(lipgloss.Left, top, input, status) }
func paneWidths(width int) (int, int, int) { usable := width - 6; if usable < 30 { return width, width, width }; left := usable/4; center := usable/2; return left, center, usable-left-center }
