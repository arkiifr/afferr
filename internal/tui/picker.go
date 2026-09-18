package tui

import (
	"fmt"
	"strings"

	"github.com/arkiifr/afferr/internal/provider"
)
func renderPicker(t Theme, width, height int) string { models := provider.BuiltinModels(); lines := []string{"MODEL PICKER", "", "Type to filter · Enter selects · Tab cycles variant", ""}; for _, model := range models { badge := ""; if model.Free() { badge = "  FREE" }; lines = append(lines, fmt.Sprintf("%-30s %6dk ctx  $%.2f/$%.2f%s", model.FullID(), model.Context/1000, model.InputPer1M, model.OutputPer1M, badge)) }; return t.Panel().Width(max(1, width-4)).Height(max(1, height-4)).Render(strings.Join(lines, "\n")) }
