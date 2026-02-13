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
	"github.com/maximbilan/grammr/internal/ratelimit"
	"github.com/maximbilan/grammr/internal/translator"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// trimTrailingWhitespace removes trailing whitespace from text
func trimTrailingWhitespace(text string) string {
	return strings.TrimRight(text, " \t\n\r")
}

// createRateLimiter creates a rate limiter from config, or returns nil if disabled
func createRateLimiter(cfg *config.Config) *ratelimit.RateLimiter {
	if !cfg.RateLimitEnabled {
		return nil
	}
	maxRequests := cfg.RateLimitRequests
	if maxRequests <= 0 {
		maxRequests = 60 // Default
	}
	windowSeconds := cfg.RateLimitWindow
	if windowSeconds <= 0 {
		windowSeconds = 60 // Default: per minute
	}
	return ratelimit.New(maxRequests, time.Duration(windowSeconds)*time.Second, 100*time.Millisecond)
}

// createTimeoutContext creates a context with timeout from config, with default fallback
func createTimeoutContext(cfg *config.Config) (context.Context, context.CancelFunc) {
	timeoutSeconds := cfg.RequestTimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30 // Default fallback
	}
	return context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
}

// saveToCache saves corrected text to cache, handling errors gracefully
func (m Model) saveToCache(original, corrected string) {
	if m.cache != nil {
		hash := m.cache.Hash(original)
		if err := m.cache.Set(hash, original, corrected); err != nil {
			// Cache write failed, but correction succeeded
			// We'll return the correction normally, but could log this in the future
		}
	}
}

type Mode int

const (
	ModeGlobal Mode = iota
	ModeEditOriginal
	ModeEditCorrected
	ModeEditTranslation
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
	mode            Mode
	originalText    string
	correctedText   string
	translatedText  string

	// UI Components
	originalEditor    textarea.Model
	correctedEditor   textarea.Model
	translationEditor textarea.Model
	viewport          viewport.Model

	// State flags
	isLoading      bool
	isTranslating  bool
	showDiff       bool
	error          string
	status         string

	// Diff review state
	diffChanges   []DiffChange // All changes from the diff
	currentChange int          // Index of current change being reviewed
	reviewedText  string       // Final text built from applied changes

	// Services
	corrector  *corrector.Corrector
	translator *translator.Translator
	cache      *cache.Cache
	config     *config.Config

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

type translationDoneMsg struct {
	translated string
}

type translationChunkMsg struct {
	chunk string
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

	// Create rate limiter if enabled
	rateLimiter := createRateLimiter(cfg)

	cor, err := corrector.NewWithRateLimit(cfg.APIKey, cfg.Model, cfg.Mode, cfg.Language, rateLimiter)
	if err != nil {
		return nil, fmt.Errorf("failed to create corrector: %w", err)
	}

	var trans *translator.Translator
	if cfg.TranslationLanguage != "" {
		trans, err = translator.NewWithRateLimit(cfg.APIKey, cfg.Model, cfg.TranslationLanguage, rateLimiter)
		if err != nil {
			return nil, fmt.Errorf("failed to create translator: %w", err)
		}
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

	translationEditor := textarea.New()
	translationEditor.Placeholder = "Translation will appear here..."
	translationEditor.CharLimit = 0
	translationEditor.SetWidth(80)
	translationEditor.SetHeight(10)

	vp := viewport.New(80, 20)

	return &Model{
		mode:             ModeGlobal,
		originalEditor:   originalEditor,
		correctedEditor:  correctedEditor,
		translationEditor: translationEditor,
		viewport:         vp,
		showDiff:         cfg.ShowDiff,
		corrector:        cor,
		translator:       trans,
		cache:            c,
		config:           cfg,
		status:           "Ready. Press V to paste, C to copy, ? for help",
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
		m.translationEditor.SetWidth(editorWidth)
		m.translationEditor.SetHeight(editorHeight)
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

		if m.mode == ModeEditOriginal || m.mode == ModeEditCorrected || m.mode == ModeEditTranslation {
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
		m.translatedText = ""
		m.translationEditor.SetValue("")
		m.isLoading = true
		m.isTranslating = false
		m.status = "[●] Correcting..."
		// Start async correction
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
		// Trigger translation if translator is configured
		if m.translator != nil && trimmedCorrected != "" {
			m.isTranslating = true
			m.status = "✓ Done [●] Translating..."
			return m, m.streamTranslation(trimmedCorrected)
		}
		return m, nil

	case translationDoneMsg:
		trimmedTranslated := trimTrailingWhitespace(msg.translated)
		m.translatedText = trimmedTranslated
		m.translationEditor.SetValue(trimmedTranslated)
		m.isTranslating = false
		if m.status == "✓ Done [●] Translating..." {
			m.status = "✓ Done ✓ Translated"
		}
		return m, nil

	case translationChunkMsg:
		m.translatedText += msg.chunk
		m.translationEditor.SetValue(m.translatedText)
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

// switchMode changes the correction mode and saves it to config
func (m Model) switchMode(modeName, displayName string) (tea.Model, tea.Cmd) {
	m.config.Mode = modeName
	rateLimiter := createRateLimiter(m.config)
	var err error
	m.corrector, err = corrector.NewWithRateLimit(m.config.APIKey, m.config.Model, modeName, m.config.Language, rateLimiter)
	if err != nil {
		return m, func() tea.Msg { return errMsg{err: err} }
	}
	// Save mode to config file
	if err := config.Save(m.config); err != nil {
		// Log error but don't fail - mode is still changed in memory
		m.status = fmt.Sprintf("Mode: %s (config save failed)", displayName)
	} else {
		m.status = fmt.Sprintf("Mode: %s", displayName)
	}
	return m, nil
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
	case "t", "T":
		if m.translatedText != "" {
			if err := clipboard.Copy(m.translatedText); err != nil {
				return m, tea.Printf("Failed to copy: %v", err)
			}
			m.status = "✓ Translation copied to clipboard"
		}
		return m, nil
	case "r", "R":
		if m.originalText != "" {
			m.isLoading = true
			m.isTranslating = false
			m.translatedText = ""
			m.translationEditor.SetValue("")
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
		return m.switchMode("casual", "Casual")
	case "2":
		return m.switchMode("formal", "Formal")
	case "3":
		return m.switchMode("academic", "Academic")
	case "4":
		return m.switchMode("technical", "Technical")
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
		} else if currentMode == ModeEditTranslation {
			m.translatedText = trimTrailingWhitespace(m.translationEditor.Value())
		}
		m.originalEditor.Blur()
		m.correctedEditor.Blur()
		m.translationEditor.Blur()
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
	} else if m.mode == ModeEditTranslation {
		var cmd tea.Cmd
		m.translationEditor, cmd = m.translationEditor.Update(msg)
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

func (m Model) streamCorrection(text string) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			return statusMsg("[●] Correcting...")
		},
		func() tea.Msg {
			ctx, cancel := createTimeoutContext(m.config)
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

			// Save to cache (handle errors gracefully - don't fail correction if cache fails)
			m.saveToCache(text, trimmedCorrected)

			return correctionDoneMsg{
				original:  text,
				corrected: trimmedCorrected,
			}
		},
	)
}

func (m Model) correctText(text string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := createTimeoutContext(m.config)
		defer cancel()

		corrected, err := m.corrector.Correct(ctx, text)
		if err != nil {
			return errMsg{err: err}
		}

		// Trim trailing whitespace from corrected text
		trimmedCorrected := trimTrailingWhitespace(corrected)

		// Save to cache (handle errors gracefully - don't fail correction if cache fails)
		m.saveToCache(text, trimmedCorrected)

		return correctionDoneMsg{
			original:  text,
			corrected: trimmedCorrected,
		}
	}
}

func (m Model) streamTranslation(text string) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			return statusMsg("[●] Translating...")
		},
		func() tea.Msg {
			ctx, cancel := createTimeoutContext(m.config)
			defer cancel()

			translated := ""
			err := m.translator.StreamTranslate(ctx, text, func(chunk string) {
				translated += chunk
			})

			if err != nil {
				return errMsg{err: err}
			}

			// Trim trailing whitespace from translated text
			trimmedTranslated := trimTrailingWhitespace(translated)

			return translationDoneMsg{
				translated: trimmedTranslated,
			}
		},
	)
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

	// Render mode indicator with visual styling
	modeIndicator := m.renderModeIndicator()

	headerLoadingIndicator := ""
	if m.isLoading {
		headerLoadingIndicator = "[●] Correcting..."
	}

	// Build header components
	headerLeft := headerStyle.Render("grammr v1.0.2") + " " + modeIndicator
	if headerLoadingIndicator != "" {
		headerLeft += " " + headerLoadingIndicator
	}

	status := statusStyle.Render(m.status)
	if m.error != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true).
			Padding(0, 1)
		status = errorStyle.Render("✗ " + m.error)
	}

	// Check if header fits on one line
	headerWidth := lipgloss.Width(headerLeft)
	statusWidth := lipgloss.Width(status)

	if headerWidth+statusWidth+2 <= m.width {
		// Fits on one line
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, headerLeft, status))
	} else {
		// Put status on next line if header is too wide
		s.WriteString(headerLeft)
		s.WriteString("\n")
		s.WriteString(status)
	}
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
		// Account for: header (1-2 lines), separator (1), spacing (1), labels (3 if translation enabled, else 2), spacing between boxes (2 if translation, else 1), separator (1), footer (1-2)
		// Total: ~12-14 lines for fixed content with translation, ~9-11 without
		hasTranslation := m.translator != nil
		fixedLines := 11
		if hasTranslation {
			fixedLines = 14
		}
		availableHeight := m.height - fixedLines
		if availableHeight < 10 {
			availableHeight = m.height - (fixedLines - 2) // Minimum space for very small terminals
		}
		numBoxes := 2
		if hasTranslation {
			numBoxes = 3
		}
		boxHeight := availableHeight / numBoxes
		if boxHeight < 3 {
			boxHeight = 3 // Minimum box height
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
		hasTranslation := m.translator != nil
		fixedLines := 11
		if hasTranslation {
			fixedLines = 14
		}
		availableHeight := m.height - fixedLines
		if availableHeight < 10 {
			availableHeight = m.height - (fixedLines - 2) // Minimum space for very small terminals
		}
		numBoxes := 2
		if hasTranslation {
			numBoxes = 3
		}
		boxHeight := availableHeight / numBoxes
		if boxHeight < 3 {
			boxHeight = 3 // Minimum box height
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

	// Translation text (only show if translator is configured)
	if m.translator != nil {
		translationLabelStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("5"))

		translationLoadingIndicator := ""
		if m.isTranslating {
			translationLoadingIndicator = lipgloss.NewStyle().
				Foreground(lipgloss.Color("11")).
				Render(" [●] Translating...")
		}

		translationLabel := translationLabelStyle.Render("Translation") + translationLoadingIndicator

		s.WriteString(translationLabel)
		s.WriteString("\n")
		if m.mode == ModeEditTranslation {
			s.WriteString(m.translationEditor.View())
		} else {
			boxWidth := m.width - 4
			if boxWidth < 20 {
				boxWidth = 20
			}
			hasTranslation := m.translator != nil
			fixedLines := 11
			if hasTranslation {
				fixedLines = 14
			}
			availableHeight := m.height - fixedLines
			if availableHeight < 10 {
				availableHeight = m.height - (fixedLines - 2) // Minimum space for very small terminals
			}
			numBoxes := 2
			if hasTranslation {
				numBoxes = 3
			}
			boxHeight := availableHeight / numBoxes
			if boxHeight < 3 {
				boxHeight = 3 // Minimum box height
			}
			boxStyle := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("8")).
				Padding(1, 2).
				Width(boxWidth).
				Height(boxHeight)

			content := m.translatedText

			// Show loading indicator in the box if translating
			if m.isTranslating && content == "" {
				loadingText := lipgloss.NewStyle().
					Foreground(lipgloss.Color("11")).
					Italic(true).
					Render("Translating...")
				content = loadingText
			}

			s.WriteString(boxStyle.Render(content))
		}
		s.WriteString("\n\n")
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1)

	// Create mode shortcuts with visual indication (compact version)
	modeShortcuts := m.renderModeShortcuts()

	// Build footer with mode shortcuts - always compact
	mainFooterText := "V: Paste  C: Copy  E: Edit  R: Retry  D: Diff  A: Review  Q: Quit  ?: Help"
	if m.translator != nil {
		mainFooterText = "V: Paste  C: Copy  T: Copy Translation  E: Edit  R: Retry  D: Diff  A: Review  Q: Quit  ?: Help"
	}
	mainFooter := footerStyle.Render(mainFooterText)
	modeShortcutsWidth := lipgloss.Width(modeShortcuts)
	mainFooterWidth := lipgloss.Width(mainFooter)

	var footer string
	if m.height > 22 && mainFooterWidth+modeShortcutsWidth+5 > m.width {
		// Two-line footer if there's space and content is too wide
		footer = mainFooter + "\n" + modeShortcuts
	} else {
		// Single-line footer
		separator := "  |  "
		if mainFooterWidth+modeShortcutsWidth+lipgloss.Width(separator) > m.width {
			// If still too wide, use shorter separator
			separator = " | "
		}
		footer = mainFooter + separator + modeShortcuts
	}

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

	// Render mode indicator with visual styling
	modeIndicator := m.renderModeIndicator()

	headerLeft := headerStyle.Render("grammr - Review Changes") + " " + modeIndicator
	status := statusStyle.Render(m.status)

	// Check if header fits on one line
	headerWidth := lipgloss.Width(headerLeft)
	statusWidth := lipgloss.Width(status)

	if headerWidth+statusWidth+2 <= m.width {
		// Fits on one line
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, headerLeft, status))
	} else {
		// Put status on next line if header is too wide
		s.WriteString(headerLeft)
		s.WriteString("\n")
		s.WriteString(status)
	}
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
		// Account for: header (1-2 lines), separator (1), spacing (1), change label (1), spacing (1),
		// preview label (1), spacing (1), separator (1), footer (1-2)
		// Total: ~9-11 lines for fixed content
		availableHeight := m.height - 11
		if availableHeight < 10 {
			availableHeight = m.height - 9 // Minimum space for very small terminals
		}
		boxHeight := availableHeight / 2
		if boxHeight < 3 {
			boxHeight = 3 // Minimum box height
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

// renderModeIndicator creates a visually styled badge for the current mode
func (m Model) renderModeIndicator() string {
	var label, color string

	switch m.config.Mode {
	case "casual":
		label = "Casual"
		color = "10" // Bright green
	case "formal":
		label = "Formal"
		color = "12" // Bright blue
	case "academic":
		label = "Academic"
		color = "13" // Bright magenta
	case "technical":
		label = "Technical"
		color = "11" // Bright yellow
	default:
		// Capitalize first letter
		if len(m.config.Mode) > 0 {
			label = strings.ToUpper(string(m.config.Mode[0])) + strings.ToLower(m.config.Mode[1:])
		} else {
			label = m.config.Mode
		}
		color = "8" // Gray
	}

	// Use a simple colored style with brackets
	modeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(color))

	return modeStyle.Render("[" + label + "]")
}

// renderModeShortcuts creates a visual indicator showing all modes with the active one highlighted
func (m Model) renderModeShortcuts() string {
	modes := []struct {
		key   string
		name  string
		color string
	}{
		{"1", "Casual", "10"},
		{"2", "Formal", "12"},
		{"3", "Academic", "13"},
		{"4", "Technical", "11"},
	}

	var shortcuts []string
	for _, mode := range modes {
		var style lipgloss.Style
		if m.config.Mode == strings.ToLower(mode.name) {
			// Active mode - highlighted with color and bold
			style = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(mode.color))
			shortcuts = append(shortcuts, style.Render("["+mode.key+": "+mode.name+"]"))
		} else {
			// Inactive mode - subtle gray
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))
			shortcuts = append(shortcuts, style.Render(mode.key+": "+mode.name))
		}
	}

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	return footerStyle.Render("Modes: " + strings.Join(shortcuts, " "))
}

func (m Model) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(1, 2).
		Width(m.width - 4).
		Height(m.height - 4)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6"))

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6"))

	// Build content line by line with proper alignment
	var content strings.Builder

	content.WriteString(headerStyle.Render("grammr - Keyboard Shortcuts"))
	content.WriteString("\n\n")

	content.WriteString(sectionStyle.Render("Global Mode:"))
	content.WriteString("\n")
	content.WriteString("  V, v      Paste from clipboard\n")
	content.WriteString("  C, c      Copy corrected text\n")
	if m.translator != nil {
		content.WriteString("  T, t      Copy translation\n")
	}
	content.WriteString("  E, e      Edit corrected text\n")
	content.WriteString("  O, o      Edit original text\n")
	content.WriteString("  R, r      Retry correction\n")
	content.WriteString("  D, d      Toggle diff view\n")
	content.WriteString("  A, a      Review changes word-by-word\n")
	content.WriteString("  Q, q      Quit\n")
	content.WriteString("  Ctrl+C    Force quit\n")
	content.WriteString("  ?, F1     Show this help\n\n")

	content.WriteString(sectionStyle.Render("Quick Actions:"))
	content.WriteString("\n")
	content.WriteString("  Ctrl+V    Paste & auto-correct\n")
	content.WriteString("  Ctrl+C    Copy & quit\n\n")

	content.WriteString(sectionStyle.Render("Modes:"))
	content.WriteString("\n")
	content.WriteString("  1         Casual (default)\n")
	content.WriteString("  2         Formal\n")
	content.WriteString("  3         Academic\n")
	content.WriteString("  4         Technical\n\n")

	content.WriteString(sectionStyle.Render("Edit Mode:"))
	content.WriteString("\n")
	content.WriteString("  Esc       Exit edit mode\n")
	content.WriteString("  Ctrl+S    Save and re-correct (original only)\n\n")

	content.WriteString(sectionStyle.Render("Review Mode:"))
	content.WriteString("\n")
	content.WriteString("  Tab       Apply current change\n")
	content.WriteString("  Space     Skip current change\n")
	content.WriteString("  Esc       Exit review mode\n")

	return helpStyle.Render(content.String())
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
