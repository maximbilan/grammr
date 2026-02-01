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
)

type Mode int

const (
	ModeGlobal Mode = iota
	ModeEditOriginal
	ModeEditCorrected
	ModeHelp
)

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

		if m.mode == ModeEditOriginal || m.mode == ModeEditCorrected {
			return m.handleEditMode(msg)
		}

		return m.handleGlobalMode(msg)

	case textPastedMsg:
		// Show pasted text immediately
		m.originalText = msg.text
		m.originalEditor.SetValue(msg.text)
		m.correctedText = ""
		m.correctedEditor.SetValue("")
		m.isLoading = true
		m.status = "[●] Correcting..."
		// Start async correction
		return m, m.streamCorrection(msg.text)

	case startStreamingMsg:
		// This is now handled by textPastedMsg, but keeping for compatibility
		m.originalText = msg.text
		m.originalEditor.SetValue(msg.text)
		m.correctedText = ""
		m.correctedEditor.SetValue("")
		m.isLoading = true
		m.status = "[●] Correcting..."
		return m, m.streamCorrection(msg.text)

	case correctionDoneMsg:
		m.originalText = msg.original
		m.correctedText = msg.corrected
		m.originalEditor.SetValue(msg.original)
		m.correctedEditor.SetValue(msg.corrected)
		m.isLoading = false
		m.status = "✓ Done"
		if m.config.AutoCopy {
			clipboard.Copy(msg.corrected)
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
			m.originalText = m.originalEditor.Value()
		} else if currentMode == ModeEditCorrected {
			m.correctedText = m.correctedEditor.Value()
		}
		m.originalEditor.Blur()
		m.correctedEditor.Blur()
		return m, nil
	case "ctrl+s":
		if m.mode == ModeEditOriginal {
			m.originalText = m.originalEditor.Value()
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

func (m Model) pasteAndCorrect() tea.Cmd {
	return func() tea.Msg {
		text, err := clipboard.Paste()
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to read clipboard: %w", err)}
		}

		if text == "" {
			return errMsg{err: fmt.Errorf("clipboard is empty")}
		}

		// Check cache first
		if m.cache != nil {
			hash := m.cache.Hash(text)
			if cached := m.cache.Get(hash); cached != "" {
				// Cache hit - return immediately with both original and corrected
				return correctionDoneMsg{
					original:  text,
					corrected: cached,
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

			// Save to cache
			if m.cache != nil {
				hash := m.cache.Hash(text)
				m.cache.Set(hash, text, corrected)
			}

			return correctionDoneMsg{
				original:  text,
				corrected: corrected,
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

		// Save to cache
		if m.cache != nil {
			hash := m.cache.Hash(text)
			m.cache.Set(hash, text, corrected)
		}

		return correctionDoneMsg{
			original:  text,
			corrected: corrected,
		}
	}
}

func (m Model) View() string {
	if m.mode == ModeHelp {
		return m.renderHelp()
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
		} else if m.showDiff && m.originalText != "" && m.correctedText != "" {
			content = renderDiff(m.originalText, m.correctedText)
		}

		s.WriteString(boxStyle.Render(content))
	}
	s.WriteString("\n\n")

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1)

	footer := footerStyle.Render("V: Paste  C: Copy  E: Edit  R: Retry  D: Diff  Q: Quit  ?: Help")
	s.WriteString(strings.Repeat("─", m.width))
	s.WriteString("\n")
	s.WriteString(footer)

	return s.String()
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
	content += "  Ctrl+S    Save and re-correct (original only)\n"

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
