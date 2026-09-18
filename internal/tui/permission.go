package tui

import (
	"fmt"
	"strings"

	"github.com/arkiifr/afferr/internal/tools"
)
func PermissionView(theme Theme, width, height int, permission tools.PermissionError) string { lines := []string{"PERMISSION REQUIRED", "", fmt.Sprintf("Tool: %s", permission.Tool), fmt.Sprintf("Reason: %s", permission.Reason), fmt.Sprintf("Args: %s", string(permission.Args)), "", "Approve this command or edit?", "", "y approve   n deny"}; return theme.Panel().Width(max(1, width-4)).Height(max(1, height-4)).Render(strings.Join(lines, "\n")) }
