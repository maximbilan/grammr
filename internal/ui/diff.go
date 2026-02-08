package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sergi/go-diff/diffmatchpatch"
)

func renderDiff(original, corrected string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(original, corrected, false)
	// Clean up the diff to make it more semantic (word-level rather than character-level)
	diffs = dmp.DiffCleanupSemantic(diffs)

	var styled strings.Builder
	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffDelete:
			styled.WriteString(
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("9")).
					Strikethrough(true).
					Render(diff.Text),
			)
		case diffmatchpatch.DiffInsert:
			styled.WriteString(
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("10")).
					Render(diff.Text),
			)
		case diffmatchpatch.DiffEqual:
			styled.WriteString(
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("8")).
					Render(diff.Text),
			)
		}
	}
	return styled.String()
}
