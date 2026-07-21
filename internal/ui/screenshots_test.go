package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/maximbilan/grammr/internal/config"
	"github.com/maximbilan/grammr/internal/provider"
	"github.com/maximbilan/grammr/internal/translator"
	"github.com/muesli/termenv"
)

// TestGenerateScreenshots renders the real TUI views to text files (with ANSI
// color codes) so they can be turned into screenshots for the docs. It never
// touches the network: it populates the model directly and calls View().
//
// It is skipped during normal test runs. To generate:
//
//	GRAMMR_SHOT_DIR=./out go test ./internal/ui -run TestGenerateScreenshots
func TestGenerateScreenshots(t *testing.T) {
	dir := os.Getenv("GRAMMR_SHOT_DIR")
	if dir == "" {
		t.Skip("set GRAMMR_SHOT_DIR to generate screenshot source files")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Force color output even though we are writing to a file, not a TTY.
	lipgloss.SetColorProfile(termenv.ANSI)

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	const (
		original  = "i beleive the API dont handle edge cases very good, we should of tested it more before we shiped it yesterday."
		corrected = "I believe the API doesn't handle edge cases very well; we should have tested it more before we shipped it yesterday."
	)

	// 1. Main correction view with the diff.
	main := Model{
		mode:          ModeGlobal,
		originalText:  original,
		correctedText: corrected,
		showDiff:      true,
		config:        &config.Config{Style: "casual", ShowDiff: true},
		status:        "✓ Done",
		width:         100,
		height:        30,
	}
	write("main", main.View())

	// 2. Review mode, sitting on the second change.
	rev := main
	rev.mode = ModeReviewDiff
	rev.diffChanges = parseDiffIntoChanges(original, corrected)
	rev.currentChange = 1
	rev.reviewedText = buildReviewedTextFromDiffs(original, corrected, rev.diffChanges)
	rev.status = "Reviewing changes (2/6) - Tab: Apply, Space: Skip, Esc: Exit"
	write("review", rev.View())

	// 3. Help overlay.
	help := main
	help.mode = ModeHelp
	help.height = 44
	write("help", help.View())

	// 4. Translation view (translator only needs to be non-nil to render the box).
	trans, err := translator.NewWithRateLimit(provider.NewMockProvider(), "mock", "spanish", nil)
	if err != nil {
		t.Fatal(err)
	}
	tv := Model{
		mode:           ModeGlobal,
		originalText:   "i cant make it to the metting tomorrow, can we reshedule for Friday?",
		correctedText:  "I can't make it to the meeting tomorrow; can we reschedule for Friday?",
		translatedText: "No podré asistir a la reunión de mañana. ¿Podríamos reprogramarla para el viernes?",
		showDiff:       true,
		translator:     trans,
		config:         &config.Config{Style: "formal", ShowDiff: true},
		status:         "✓ Done ✓ Translated",
		width:          100,
		height:         34,
	}
	write("translation", tv.View())
}
