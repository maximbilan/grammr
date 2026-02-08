package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/maximbilan/grammr/internal/cache"
	"github.com/maximbilan/grammr/internal/clipboard"
	"github.com/maximbilan/grammr/internal/config"
	"github.com/maximbilan/grammr/internal/corrector"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// trimTrailingWhitespace removes trailing whitespace from text
func trimTrailingWhitespace(text string) string {
	return strings.TrimRight(text, " \t\n\r")
}

type Mode int

const (
	ModeGlobal Mode = iota
	ModeEditOriginal
	ModeEditCorrected
	ModeHelp
	ModeReviewDiff
)

// DiffChange represents a single change in the diff
type DiffChange struct {
	Type       diffmatchpatch.Operation
	Text       string
	Applied    bool // true if user applied this change
	Skipped    bool // true if user skipped this change
}

type Model struct {
	// State
	mode          Mode
	originalText  string
	correctedText string

	// UI Components
	originalEditor  textarea.Model
	correctedEditor textarea.Model
	viewport        viewport.Model

	// State flags
	isLoading bool
	showDiff  bool
	error     string
	status    string

	// Diff review state
	diffChanges   []DiffChange // All changes from the diff
	currentChange int          // Index of current change being reviewed
	reviewedText  string       // Final text built from applied changes

	// Services
	corrector *corrector.Corrector
	cache     *cache.Cache
	config    *config.Config

	// Dimensions
	width  int
	height int
}

// Messages
type textPastedMsg struct {
	text string
}

type correctionDoneMsg struct {
	original  string
	corrected string
}

type streamChunkMsg struct {
	chunk string
}

type errMsg struct {
	err error
}

func (e errMsg) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return "unknown error"
}

type statusMsg string

// parseDiffIntoChanges parses the diff and returns a list of changes to review
// It pairs delete+insert sequences as single changes for better UX
func parseDiffIntoChanges(original, corrected string) []DiffChange {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(original, corrected, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	changes := make([]DiffChange, 0)
	i := 0
	for i < len(diffs) {
		diff := diffs[i]

		if diff.Type == diffmatchpatch.DiffEqual {
			i++
			continue
		}

		// Check if this is a delete followed by an insert (common pattern)
		if diff.Type == diffmatchpatch.DiffDelete && i+1 < len(diffs) && diffs[i+1].Type == diffmatchpatch.DiffInsert {
			// Pair them as a single change
			changes = append(changes, DiffChange{
				Type:    diffmatchpatch.DiffDelete, // Use delete as primary type
				Text:    diff.Text + " → " + diffs[i+1].Text, // Show both
				Applied: false,
				Skipped: false,
			})
			i += 2
		} else {
			// Single change
			changes = append(changes, DiffChange{
				Type:    diff.Type,
				Text:    diff.Text,
				Applied: false,
				Skipped: false,
			})
			i++
		}
	}
	return changes
}

// buildReviewedText builds the final text based on applied changes
func buildReviewedText(original string, changes []DiffChange) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(original, original, false) // Start with original
	diffs = dmp.DiffCleanupSemantic(diffs)

	// Rebuild diffs with applied changes
	changeIdx := 0
	var result strings.Builder
	diffIdx := 0

	for diffIdx < len(diffs) {
		diff := diffs[diffIdx]

		if diff.Type == diffmatchpatch.DiffEqual {
			result.WriteString(diff.Text)
			diffIdx++
		} else if diff.Type == diffmatchpatch.DiffDelete {
			// Check if this change was applied
			if changeIdx < len(changes) && changes[changeIdx].Type == diffmatchpatch.DiffDelete {
				if changes[changeIdx].Applied {
					// Skip the delete (don't write it)
				} else if changes[changeIdx].Skipped {
					// Keep the original text
					result.WriteString(diff.Text)
				}
				changeIdx++
			} else {
				// No decision made, skip by default
				result.WriteString(diff.Text)
			}
			diffIdx++
		} else if diff.Type == diffmatchpatch.DiffInsert {
			// Check if this change was applied
			if changeIdx < len(changes) && changes[changeIdx].Type == diffmatchpatch.DiffInsert {
				if changes[changeIdx].Applied {
					// Apply the insert
					result.WriteString(diff.Text)
				}
				changeIdx++
			}
			diffIdx++
		}
	}

	return result.String()
}

// buildReviewedTextFromDiffs builds text from original and corrected using change decisions
func buildReviewedTextFromDiffs(original, corrected string, changes []DiffChange) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(original, corrected, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	var result strings.Builder
	changeIdx := 0

	for i := 0; i < len(diffs); i++ {
		diff := diffs[i]

		if diff.Type == diffmatchpatch.DiffEqual {
			result.WriteString(diff.Text)
		} else if diff.Type == diffmatchpatch.DiffDelete {
			// Check if there's a following insert (paired change)
			if i+1 < len(diffs) && diffs[i+1].Type == diffmatchpatch.DiffInsert {
				// This is a paired delete+insert
				if changeIdx < len(changes) {
					change := changes[changeIdx]
					// Verify this is a paired change (should contain " → ")
					if strings.Contains(change.Text, " → ") {
						if change.Applied {
							// Apply the change: skip delete, add insert
							result.WriteString(diffs[i+1].Text)
						} else if change.Skipped {
							// Skip the change: keep original (delete text)
							result.WriteString(diff.Text)
						} else {
							// No decision yet - keep original
							result.WriteString(diff.Text)
						}
						changeIdx++
						i++ // Skip the insert as we've handled it
					} else {
						// Change doesn't match - this shouldn't happen
						// Keep original and don't increment changeIdx
						result.WriteString(diff.Text)
						i++ // Skip the insert
					}
				} else {
					// No more changes - keep original
					result.WriteString(diff.Text)
					i++ // Skip the insert
				}
			} else {
				// Single delete (not paired with insert)
				if changeIdx < len(changes) {
					change := changes[changeIdx]
					// This should be a single delete change (no " → " in text)
					if change.Type == diffmatchpatch.DiffDelete && !strings.Contains(change.Text, " → ") {
						if change.Skipped {
							result.WriteString(diff.Text)
						}
						// If applied, we don't write it (it's deleted)
						changeIdx++
					} else {
						// Mismatch - keep original
						result.WriteString(diff.Text)
					}
				} else {
					result.WriteString(diff.Text)
				}
			}
		} else if diff.Type == diffmatchpatch.DiffInsert {
			// Single insert (not paired with delete)
			// This should only happen if we didn't pair it with a delete above
			if changeIdx < len(changes) {
				change := changes[changeIdx]
				// This should be a single insert change
				if change.Type == diffmatchpatch.DiffInsert && !strings.Contains(change.Text, " → ") {
					if change.Applied {
						result.WriteString(diff.Text)
					}
					// If skipped, we don't write it
					changeIdx++
				} else {
					// Mismatch - don't write the insert
				}
			}
		}
	}

	return result.String()
}

func NewModel(cfg *config.Config) (*Model, error) {
	var c *cache.Cache
	if cfg.CacheEnabled {
		var err error
		c, err = cache.New(cfg.CacheTTLDays)
		if err != nil {
			return nil, fmt.Errorf("failed to create cache: %w", err)
		}
	}

	cor, err := corrector.New(cfg.APIKey, cfg.Model, cfg.Mode)
	if err != nil {
		return nil, fmt.Errorf("failed to create corrector: %w", err)
	}

	originalEditor := textarea.New()
	originalEditor.Placeholder = "Original text will appear here..."
	originalEditor.CharLimit = 0
	originalEditor.SetWidth(80)
	originalEditor.SetHeight(10)

	correctedEditor := textarea.New()
	correctedEditor.Placeholder = "Corrected text will appear here..."
	correctedEditor.CharLimit = 0
	correctedEditor.SetWidth(80)
	correctedEditor.SetHeight(10)

	vp := viewport.New(80, 20)

	return &Model{
		mode:            ModeGlobal,
		originalEditor:  originalEditor,
		correctedEditor: correctedEditor,
		viewport:        vp,
		showDiff:        cfg.ShowDiff,
		corrector:       cor,
		cache:           c,
		config:          cfg,
		status:          "Ready. Press V to paste, C to copy, ? for help",
	}, nil
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		editorWidth := msg.Width - 4
		if editorWidth < 20 {
			editorWidth = 20
		}
		editorHeight := (msg.Height - 10) / 2
		if editorHeight < 5 {
			editorHeight = 5
		}
		m.originalEditor.SetWidth(editorWidth)
		m.originalEditor.SetHeight(editorHeight)
		m.correctedEditor.SetWidth(editorWidth)
		m.correctedEditor.SetHeight(editorHeight)
		m.viewport.Width = editorWidth
		m.viewport.Height = msg.Height - 10
		return m, nil

	case tea.KeyMsg:
		if m.mode == ModeHelp {
			if msg.Type == tea.KeyEsc || msg.String() == "?" || msg.String() == "q" {
				m.mode = ModeGlobal
				return m, nil
			}
			return m, nil
		}

		if m.mode == ModeReviewDiff {
			return m.handleReviewMode(msg)
		}

		if m.mode == ModeEditOriginal || m.mode == ModeEditCorrected {
			return m.handleEditMode(msg)
		}

		return m.handleGlobalMode(msg)

	case textPastedMsg:
		// Show pasted text immediately (trim trailing whitespace)
		trimmedText := trimTrailingWhitespace(msg.text)
		m.originalText = trimmedText
		m.originalEditor.SetValue(trimmedText)
		m.correctedText = ""
		m.correctedEditor.SetValue("")
		m.isLoading = true
		m.status = "[●] Correcting..."
		// Start async correction
		return m, m.streamCorrection(trimmedText)

	case startStreamingMsg:
		// This is now handled by textPastedMsg, but keeping for compatibility
		trimmedText := trimTrailingWhitespace(msg.text)
		m.originalText = trimmedText
		m.originalEditor.SetValue(trimmedText)
		m.correctedText = ""
		m.correctedEditor.SetValue("")
		m.isLoading = true
		m.status = "[●] Correcting..."
		return m, m.streamCorrection(trimmedText)

	case correctionDoneMsg:
		// Trim trailing whitespace from both original and corrected
		trimmedOriginal := trimTrailingWhitespace(msg.original)
		trimmedCorrected := trimTrailingWhitespace(msg.corrected)
		m.originalText = trimmedOriginal
		m.correctedText = trimmedCorrected
		m.originalEditor.SetValue(trimmedOriginal)
		m.correctedEditor.SetValue(trimmedCorrected)
		m.isLoading = false
		m.status = "✓ Done"
		if m.config.AutoCopy {
			clipboard.Copy(trimmedCorrected)
			m.status = "✓ Done (copied)"
		}
		return m, nil

	case streamChunkMsg:
		m.correctedText += msg.chunk
		m.correctedEditor.SetValue(m.correctedText)
		return m, nil

	case errMsg:
		m.error = msg.Error()
		m.isLoading = false
		m.status = fmt.Sprintf("✗ Error: %s", msg.Error())
		return m, nil

	case statusMsg:
		m.status = string(msg)
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleGlobalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "v", "V":
		return m, m.pasteAndCorrect()
	case "c", "C":
		if m.correctedText != "" {
			if err := clipboard.Copy(m.correctedText); err != nil {
				return m, tea.Printf("Failed to copy: %v", err)
			}
			m.status = "✓ Copied to clipboard"
		}
		return m, nil
	case "e", "E":
		if m.correctedText != "" {
			m.mode = ModeEditCorrected
			m.correctedEditor.Focus()
			return m, textarea.Blink
		}
		return m, nil
	case "o", "O":
		if m.originalText != "" {
			m.mode = ModeEditOriginal
			m.originalEditor.Focus()
			return m, textarea.Blink
		}
		return m, nil
	case "r", "R":
		if m.originalText != "" {
			m.isLoading = true
			m.status = "[●] Correcting..."
			return m, m.correctText(m.originalText)
		}
		return m, nil
	case "d", "D":
		m.showDiff = !m.showDiff
		return m, nil
	case "a", "A":
		// Enter review mode to apply/skip changes word by word
		if m.originalText != "" && m.correctedText != "" {
			m.diffChanges = parseDiffIntoChanges(m.originalText, m.correctedText)
			m.currentChange = 0
			if len(m.diffChanges) > 0 {
				m.mode = ModeReviewDiff
				m.reviewedText = buildReviewedTextFromDiffs(m.originalText, m.correctedText, m.diffChanges)
				m.status = fmt.Sprintf("Reviewing changes (%d/%d) - Tab: Apply, Space: Skip, Esc: Exit", m.currentChange+1, len(m.diffChanges))
			} else {
				m.status = "No changes to review"
			}
		}
		return m, nil
	case "?", "f1":
		m.mode = ModeHelp
		return m, nil
	case "q", "Q":
		return m, tea.Quit
	case "ctrl+v":
		return m, tea.Sequence(m.pasteAndCorrect(), func() tea.Msg {
			time.Sleep(100 * time.Millisecond)
			return nil
		})
	case "ctrl+c":
		if m.correctedText != "" {
			clipboard.Copy(m.correctedText)
			return m, tea.Quit
		}
		return m, tea.Quit
	case "1":
		m.config.Mode = "casual"
		var err error
		m.corrector, err = corrector.New(m.config.APIKey, m.config.Model, "casual")
		if err != nil {
			return m, func() tea.Msg { return errMsg{err: err} }
		}
		m.status = "Mode: Casual"
		return m, nil
	case "2":
		m.config.Mode = "formal"
		var err error
		m.corrector, err = corrector.New(m.config.APIKey, m.config.Model, "formal")
		if err != nil {
			return m, func() tea.Msg { return errMsg{err: err} }
		}
		m.status = "Mode: Formal"
		return m, nil
	case "3":
		m.config.Mode = "academic"
		var err error
		m.corrector, err = corrector.New(m.config.APIKey, m.config.Model, "academic")
		if err != nil {
			return m, func() tea.Msg { return errMsg{err: err} }
		}
		m.status = "Mode: Academic"
		return m, nil
	case "4":
		m.config.Mode = "technical"
		var err error
		m.corrector, err = corrector.New(m.config.APIKey, m.config.Model, "technical")
		if err != nil {
			return m, func() tea.Msg { return errMsg{err: err} }
		}
		m.status = "Mode: Technical"
		return m, nil
	}

	return m, nil
}

func (m Model) handleEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		currentMode := m.mode
		m.mode = ModeGlobal
		if currentMode == ModeEditOriginal {
			m.originalText = trimTrailingWhitespace(m.originalEditor.Value())
		} else if currentMode == ModeEditCorrected {
			m.correctedText = trimTrailingWhitespace(m.correctedEditor.Value())
		}
		m.originalEditor.Blur()
		m.correctedEditor.Blur()
		return m, nil
	case "ctrl+s":
		if m.mode == ModeEditOriginal {
			m.originalText = trimTrailingWhitespace(m.originalEditor.Value())
			m.mode = ModeGlobal
			m.originalEditor.Blur()
			m.isLoading = true
			m.status = "[●] Correcting..."
			return m, m.correctText(m.originalText)
		}
		return m, nil
	}

	if m.mode == ModeEditOriginal {
		var cmd tea.Cmd
		m.originalEditor, cmd = m.originalEditor.Update(msg)
		return m, cmd
	} else if m.mode == ModeEditCorrected {
		var cmd tea.Cmd
		m.correctedEditor, cmd = m.correctedEditor.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleReviewMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		// Apply current change
		if m.currentChange < len(m.diffChanges) {
			m.diffChanges[m.currentChange].Applied = true
			m.diffChanges[m.currentChange].Skipped = false
			m.reviewedText = buildReviewedTextFromDiffs(m.originalText, m.correctedText, m.diffChanges)
			m.currentChange++

			if m.currentChange >= len(m.diffChanges) {
				// All changes reviewed - rebuild to ensure final state is correct
				m.reviewedText = buildReviewedTextFromDiffs(m.originalText, m.correctedText, m.diffChanges)
				m.correctedText = m.reviewedText
				m.correctedEditor.SetValue(m.reviewedText)
				// Disable diff view to show the actual corrected text, not a diff
				m.showDiff = false
				// Copy to clipboard
				if err := clipboard.Copy(m.reviewedText); err != nil {
					m.status = fmt.Sprintf("✓ All changes reviewed (copy failed: %v)", err)
				} else {
					m.status = "✓ All changes reviewed (copied)"
				}
				m.mode = ModeGlobal
			} else {
				m.status = fmt.Sprintf("Reviewing changes (%d/%d) - Tab: Apply, Space: Skip, Esc: Exit", m.currentChange+1, len(m.diffChanges))
			}
		}
		return m, nil
	case " ":
		// Skip current change
		if m.currentChange < len(m.diffChanges) {
			m.diffChanges[m.currentChange].Applied = false
			m.diffChanges[m.currentChange].Skipped = true
			m.reviewedText = buildReviewedTextFromDiffs(m.originalText, m.correctedText, m.diffChanges)
			m.currentChange++

			if m.currentChange >= len(m.diffChanges) {
				// All changes reviewed - rebuild to ensure final state is correct
				m.reviewedText = buildReviewedTextFromDiffs(m.originalText, m.correctedText, m.diffChanges)
				m.correctedText = m.reviewedText
				m.correctedEditor.SetValue(m.reviewedText)
				// Disable diff view to show the actual corrected text, not a diff
				m.showDiff = false
				// Copy to clipboard
				if err := clipboard.Copy(m.reviewedText); err != nil {
					m.status = fmt.Sprintf("✓ All changes reviewed (copy failed: %v)", err)
				} else {
					m.status = "✓ All changes reviewed (copied)"
				}
				m.mode = ModeGlobal
			} else {
				m.status = fmt.Sprintf("Reviewing changes (%d/%d) - Tab: Apply, Space: Skip, Esc: Exit", m.currentChange+1, len(m.diffChanges))
			}
		}
		return m, nil
	case "esc":
		// Exit review mode and apply reviewed changes
		// Rebuild reviewedText to ensure it's up-to-date with all decisions
		m.reviewedText = buildReviewedTextFromDiffs(m.originalText, m.correctedText, m.diffChanges)
		// Update correctedText with the reviewed text (which includes all applied changes)
		m.correctedText = m.reviewedText
		m.correctedEditor.SetValue(m.reviewedText)
		// Disable diff view to show the actual corrected text, not a diff
		m.showDiff = false
		// Copy to clipboard
		if err := clipboard.Copy(m.reviewedText); err != nil {
			m.status = fmt.Sprintf("Review mode exited (copy failed: %v)", err)
		} else {
			m.status = "Review mode exited (copied)"
		}
		m.mode = ModeGlobal
		return m, nil
	}
	return m, nil
}

func (m Model) pasteAndCorrect() tea.Cmd {
	return func() tea.Msg {
		text, err := clipboard.Paste()
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to read clipboard: %w", err)}
		}

		// Trim trailing whitespace before processing
		text = trimTrailingWhitespace(text)
		if text == "" {
			return errMsg{err: fmt.Errorf("clipboard is empty or contains only whitespace")}
		}

		// Check cache first
		if m.cache != nil {
			hash := m.cache.Hash(text)
			if cached := m.cache.Get(hash); cached != "" {
				// Cache hit - return immediately with both original and corrected
				trimmedCached := trimTrailingWhitespace(cached)
				return correctionDoneMsg{
					original:  text,
					corrected: trimmedCached,
				}
			}
		}

		// No cache - show original immediately, then start correction
		return textPastedMsg{text: text}
	}
}

type startStreamingMsg struct {
	text string
}

func (m Model) streamCorrection(text string) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			return statusMsg("[●] Correcting...")
		},
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			corrected := ""
			err := m.corrector.StreamCorrect(ctx, text, func(chunk string) {
				corrected += chunk
			})

			if err != nil {
				return errMsg{err: err}
			}

			// Trim trailing whitespace from corrected text
			trimmedCorrected := trimTrailingWhitespace(corrected)

			// Save to cache
			if m.cache != nil {
				hash := m.cache.Hash(text)
				m.cache.Set(hash, text, trimmedCorrected)
			}

			return correctionDoneMsg{
				original:  text,
				corrected: trimmedCorrected,
			}
		},
	)
}

func (m Model) correctText(text string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		corrected, err := m.corrector.Correct(ctx, text)
		if err != nil {
			return errMsg{err: err}
		}

		// Trim trailing whitespace from corrected text
		trimmedCorrected := trimTrailingWhitespace(corrected)

		// Save to cache
		if m.cache != nil {
			hash := m.cache.Hash(text)
			m.cache.Set(hash, text, trimmedCorrected)
		}

		return correctionDoneMsg{
			original:  text,
			corrected: trimmedCorrected,
		}
	}
}

func (m Model) View() string {
	if m.mode == ModeHelp {
		return m.renderHelp()
	}

	if m.mode == ModeReviewDiff {
		return m.renderReviewMode()
	}

	// Ensure we have valid dimensions
	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
	}

	var s strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		Padding(0, 1)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1)

	modeIndicator := ""
	switch m.config.Mode {
	case "casual":
		modeIndicator = "[Casual]"
	case "formal":
		modeIndicator = "[Formal]"
	case "academic":
		modeIndicator = "[Academic]"
	case "technical":
		modeIndicator = "[Technical]"
	}

	headerLoadingIndicator := ""
	if m.isLoading {
		headerLoadingIndicator = "[●] Correcting..."
	}

	header := headerStyle.Render("grammr v1.0") + " " + modeIndicator + " " + headerLoadingIndicator
	status := statusStyle.Render(m.status)

	if m.error != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true).
			Padding(0, 1)
		status = errorStyle.Render("✗ " + m.error)
	}

	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, header, status))
	s.WriteString("\n")
	s.WriteString(strings.Repeat("─", m.width))
	s.WriteString("\n\n")

	// Original text
	originalLabel := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("4")).
		Render("Original Text")

	s.WriteString(originalLabel)
	s.WriteString("\n")
	if m.mode == ModeEditOriginal {
		s.WriteString(m.originalEditor.View())
	} else {
		boxWidth := m.width - 4
		if boxWidth < 20 {
			boxWidth = 20
		}
		boxHeight := (m.height - 10) / 2
		if boxHeight < 5 {
			boxHeight = 5
		}
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(1, 2).
			Width(boxWidth).
			Height(boxHeight)
		s.WriteString(boxStyle.Render(m.originalText))
	}
	s.WriteString("\n\n")

	// Corrected text
	correctedLabelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("2"))

	loadingIndicator := ""
	if m.isLoading {
		loadingIndicator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Render(" [●] Correcting...")
	}

	correctedLabel := correctedLabelStyle.Render("Corrected Text") + loadingIndicator

	s.WriteString(correctedLabel)
	s.WriteString("\n")
	if m.mode == ModeEditCorrected {
		s.WriteString(m.correctedEditor.View())
	} else {
		boxWidth := m.width - 4
		if boxWidth < 20 {
			boxWidth = 20
		}
		boxHeight := (m.height - 10) / 2
		if boxHeight < 5 {
			boxHeight = 5
		}
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(1, 2).
			Width(boxWidth).
			Height(boxHeight)

		content := m.correctedText

		// Show loading indicator in the box if loading
		if m.isLoading && content == "" {
			loadingText := lipgloss.NewStyle().
				Foreground(lipgloss.Color("11")).
				Italic(true).
				Render("Correcting...")
			content = loadingText
		} else if m.showDiff && m.originalText != "" && m.correctedText != "" && m.mode != ModeReviewDiff {
			// Only show diff view when not in review mode (review mode has its own display)
			content = renderDiff(m.originalText, m.correctedText)
		}

		s.WriteString(boxStyle.Render(content))
	}
	s.WriteString("\n\n")

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1)

	footer := footerStyle.Render("V: Paste  C: Copy  E: Edit  R: Retry  D: Diff  A: Review  Q: Quit  ?: Help")
	s.WriteString(strings.Repeat("─", m.width))
	s.WriteString("\n")
	s.WriteString(footer)

	return s.String()
}

func (m Model) renderReviewMode() string {
	// Ensure we have valid dimensions
	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
	}

	var s strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		Padding(0, 1)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1)

	header := headerStyle.Render("grammr - Review Changes")
	status := statusStyle.Render(m.status)

	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, header, status))
	s.WriteString("\n")
	s.WriteString(strings.Repeat("─", m.width))
	s.WriteString("\n\n")

	// Show current change being reviewed
	if m.currentChange < len(m.diffChanges) {
		change := m.diffChanges[m.currentChange]

		changeLabel := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("11")).
			Render(fmt.Sprintf("Change %d of %d", m.currentChange+1, len(m.diffChanges)))

		s.WriteString(changeLabel)
		s.WriteString("\n\n")

		boxWidth := m.width - 4
		if boxWidth < 20 {
			boxWidth = 20
		}
		boxHeight := (m.height - 15) / 2
		if boxHeight < 5 {
			boxHeight = 5
		}

		// Show what's being changed
		if strings.Contains(change.Text, " → ") {
			// Paired change (delete → insert)
			parts := strings.SplitN(change.Text, " → ", 2)
			deletePart := parts[0]
			insertPart := parts[1]

			deleteStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("9")).
				Strikethrough(true).
				Bold(true)
			insertStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("10")).
				Bold(true)

			changeText := fmt.Sprintf("Change: %s → %s",
				deleteStyle.Render(deletePart),
				insertStyle.Render(insertPart))

			boxStyle := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("11")).
				Padding(1, 2).
				Width(boxWidth).
				Height(boxHeight)
			s.WriteString(boxStyle.Render(changeText))
		} else if change.Type == diffmatchpatch.DiffDelete {
			deleteStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("9")).
				Strikethrough(true).
				Bold(true)
			changeText := deleteStyle.Render(fmt.Sprintf("Remove: %q", change.Text))

			boxStyle := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("9")).
				Padding(1, 2).
				Width(boxWidth).
				Height(boxHeight)
			s.WriteString(boxStyle.Render(changeText))
		} else if change.Type == diffmatchpatch.DiffInsert {
			insertStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("10")).
				Bold(true)
			changeText := insertStyle.Render(fmt.Sprintf("Add: %q", change.Text))

			boxStyle := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("10")).
				Padding(1, 2).
				Width(boxWidth).
				Height(boxHeight)
			s.WriteString(boxStyle.Render(changeText))
		}

		s.WriteString("\n\n")

		// Show preview of reviewed text so far
		previewLabel := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("8")).
			Render("Preview:")

		s.WriteString(previewLabel)
		s.WriteString("\n")

		previewBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(1, 2).
			Width(boxWidth).
			Height(boxHeight)

		// Show the reviewed text with highlighting for current change
		previewText := m.renderReviewPreview()
		s.WriteString(previewBoxStyle.Render(previewText))
	} else {
		// All changes reviewed
		doneStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")).
			Bold(true)
		s.WriteString(doneStyle.Render("✓ All changes reviewed!"))
	}

	s.WriteString("\n\n")

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1)

	footer := footerStyle.Render("Tab: Apply  Space: Skip  Esc: Exit")
	s.WriteString(strings.Repeat("─", m.width))
	s.WriteString("\n")
	s.WriteString(footer)

	return s.String()
}

func (m Model) renderReviewPreview() string {
	// Build a preview showing the current state with the current change highlighted
	// Use the reviewed text which already has decisions applied
	previewText := m.reviewedText

	// If we have a current change, highlight it in the preview
	if m.currentChange < len(m.diffChanges) {
		// Find and highlight the current change in the text
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(m.originalText, m.correctedText, false)
		diffs = dmp.DiffCleanupSemantic(diffs)

		var result strings.Builder
		changeIdx := 0

		for i := 0; i < len(diffs); i++ {
			diff := diffs[i]

			if diff.Type == diffmatchpatch.DiffEqual {
				result.WriteString(
					lipgloss.NewStyle().
						Foreground(lipgloss.Color("8")).
						Render(diff.Text),
				)
			} else if diff.Type == diffmatchpatch.DiffDelete {
				// Check if paired with insert
				if i+1 < len(diffs) && diffs[i+1].Type == diffmatchpatch.DiffInsert {
					// Paired change
					if changeIdx == m.currentChange {
						// Highlight current change
						result.WriteString(
							lipgloss.NewStyle().
								Foreground(lipgloss.Color("11")).
								Background(lipgloss.Color("9")).
								Bold(true).
								Strikethrough(true).
								Render(diff.Text),
						)
						result.WriteString(
							lipgloss.NewStyle().
								Foreground(lipgloss.Color("11")).
								Background(lipgloss.Color("10")).
								Bold(true).
								Render(diffs[i+1].Text),
						)
					} else if changeIdx < len(m.diffChanges) {
						change := m.diffChanges[changeIdx]
						if change.Applied {
							result.WriteString(
								lipgloss.NewStyle().
									Foreground(lipgloss.Color("10")).
									Render(diffs[i+1].Text),
							)
						} else if change.Skipped {
							result.WriteString(
								lipgloss.NewStyle().
									Foreground(lipgloss.Color("8")).
									Render(diff.Text),
							)
						} else {
							// Not reviewed yet
							result.WriteString(
								lipgloss.NewStyle().
									Foreground(lipgloss.Color("9")).
									Strikethrough(true).
									Render(diff.Text),
							)
							result.WriteString(
								lipgloss.NewStyle().
									Foreground(lipgloss.Color("10")).
									Render(diffs[i+1].Text),
							)
						}
					}
					changeIdx++
					i++ // Skip insert
				} else {
					// Single delete
					if changeIdx == m.currentChange {
						result.WriteString(
							lipgloss.NewStyle().
								Foreground(lipgloss.Color("11")).
								Background(lipgloss.Color("9")).
								Bold(true).
								Strikethrough(true).
								Render(diff.Text),
						)
					} else if changeIdx < len(m.diffChanges) {
						change := m.diffChanges[changeIdx]
						if change.Skipped {
							result.WriteString(
								lipgloss.NewStyle().
									Foreground(lipgloss.Color("8")).
									Render(diff.Text),
							)
						} else if !change.Applied {
							result.WriteString(
								lipgloss.NewStyle().
									Foreground(lipgloss.Color("9")).
									Strikethrough(true).
									Render(diff.Text),
							)
						}
					}
					changeIdx++
				}
			} else if diff.Type == diffmatchpatch.DiffInsert {
				// Single insert (shouldn't happen if paired correctly, but handle it)
				if changeIdx == m.currentChange {
					result.WriteString(
						lipgloss.NewStyle().
							Foreground(lipgloss.Color("11")).
							Background(lipgloss.Color("10")).
							Bold(true).
							Render(diff.Text),
					)
				} else if changeIdx < len(m.diffChanges) {
					change := m.diffChanges[changeIdx]
					if change.Applied {
						result.WriteString(
							lipgloss.NewStyle().
								Foreground(lipgloss.Color("10")).
								Render(diff.Text),
						)
					}
				}
				changeIdx++
			}
		}

		return result.String()
	}

	// Fallback: just show the reviewed text
	return previewText
}

func (m Model) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(1, 2).
		Width(m.width - 4).
		Height(m.height - 4)

	content := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		Render("grammr - Keyboard Shortcuts\n\n")

	content += "Global Mode:\n"
	content += "  V, v      Paste from clipboard\n"
	content += "  C, c      Copy corrected text\n"
	content += "  E, e      Edit corrected text\n"
	content += "  O, o      Edit original text\n"
	content += "  R, r      Retry correction\n"
	content += "  D, d      Toggle diff view\n"
	content += "  A, a      Review changes word-by-word\n"
	content += "  Q, q      Quit\n"
	content += "  Ctrl+C    Force quit\n"
	content += "  ?, F1     Show this help\n\n"

	content += "Quick Actions:\n"
	content += "  Ctrl+V    Paste & auto-correct\n"
	content += "  Ctrl+C    Copy & quit\n\n"

	content += "Modes:\n"
	content += "  1         Casual (default)\n"
	content += "  2         Formal\n"
	content += "  3         Academic\n"
	content += "  4         Technical\n\n"

	content += "Edit Mode:\n"
	content += "  Esc       Exit edit mode\n"
	content += "  Ctrl+S    Save and re-correct (original only)\n\n"

	content += "Review Mode:\n"
	content += "  Tab       Apply current change\n"
	content += "  Space     Skip current change\n"
	content += "  Esc       Exit review mode\n"

	return helpStyle.Render(content)
}

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.APIKey == "" {
		return fmt.Errorf("API key not configured. Run: grammr config set api_key YOUR_KEY")
	}

	model, err := NewModel(cfg)
	if err != nil {
		return fmt.Errorf("failed to create model: %w", err)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("program error: %w", err)
	}

	return nil
}
