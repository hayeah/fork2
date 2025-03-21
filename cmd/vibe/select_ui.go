package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

// ExitState indicates how the program is exiting
type ExitState int

const (
	ExitStateNone    ExitState = iota // Not exiting
	ExitStateAbort                    // Exiting without saving (ESC, Ctrl+C)
	ExitStateConfirm                  // Exiting with confirmation (Enter)
)

// model is our Bubble Tea model, holding everything needed for the TUI.
type model struct {
	// Input handling
	textInput  textinput.Model
	searchTerm string

	// Files
	allItems      []item // All items in the entire tree
	filteredItems []item // Filtered subset after fuzzy search
	selected      map[string]bool
	lookup        map[string]int // Map from path -> index in allItems for quick toggle

	// For toggling entire sub‐trees
	childrenMap map[string][]string

	// Navigation
	cursor    int
	exitState ExitState

	// Viewport for scrolling
	viewport viewport.Model
	ready    bool

	// Token counting
	totalTokenCount int            // Total token count of selected files
	tokenEstimator  TokenEstimator // Function to estimate tokens
	tokenCache      map[string]int // Cache of token counts to avoid recalculating
}

// selectFilesInteractively runs the TUI for interactive file selection
// and returns the selected files and their total token count
func selectFilesInteractively(items []item, childrenMap map[string][]string, tokenEstimator TokenEstimator) ([]string, int, error) {
	// Set up initial model
	ti := textinput.New()
	ti.Placeholder = "Type to fuzzy-search..."
	ti.Prompt = "> "
	ti.CharLimit = 0
	ti.Focus()

	selected := make(map[string]bool)
	lookup := make(map[string]int, len(items))
	for i, it := range items {
		lookup[it.Path] = i
	}

	m := model{
		textInput:       ti,
		allItems:        items,
		filteredItems:   items, // default to showing them all
		selected:        selected,
		lookup:          lookup,
		childrenMap:     childrenMap,
		viewport:        viewport.New(0, 0), // Will be properly sized in tea.WindowSizeMsg
		ready:           false,
		totalTokenCount: 0, // Initialize token count
		tokenEstimator:  tokenEstimator,
		tokenCache:      make(map[string]int),
	}

	// Start Bubble Tea. Output TUI stderr so we can pipe the output to stdout
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return nil, 0, err
	}

	// Get the final state of the model
	finalM, ok := finalModel.(model)
	if !ok {
		return nil, 0, fmt.Errorf("could not get final model state")
	}

	// Only generate output if we're exiting with confirmation (Enter)
	if finalM.exitState != ExitStateConfirm {
		return nil, 0, nil
	}

	// On Enter, collect all selected items
	// Convert selected files to slice, sort them, and filter out directories
	var selectedFiles []string
	for path := range finalM.selected {
		// Get the index of this path in allItems to check if it's a directory
		if idx, ok := finalM.lookup[path]; ok && !finalM.allItems[idx].IsDir {
			selectedFiles = append(selectedFiles, path)
		}
	}

	return selectedFiles, finalM.totalTokenCount, nil
}

// Init is the first function called by Bubble Tea. We return an initial command (or nil).
func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.EnterAltScreen)
}

// Update is called when events occur (key presses, etc.). We handle them here,
// then return the updated model and an optional command.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If we're exiting, no further updates needed
	if m.exitState != ExitStateNone {
		return m, tea.Quit
	}

	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Set the viewport height to fit the terminal
		headerHeight := lipgloss.Height(m.textInput.View()) + 1 // Input field + blank line
		footerHeight := 2                                       // Status line + blank line
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - footerHeight
		m.viewport.YPosition = headerHeight // Position viewport below header

		if !m.ready {
			// This is the first time we're getting a WindowSizeMsg
			m.updateViewportContent()
			m.ready = true
		}

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "esc":
			m.exitState = ExitStateAbort
			// No selection is printed, we just bail:
			return m, tea.Quit

		case "enter":
			m.exitState = ExitStateConfirm
			return m, tea.Quit

		case "up":
			if m.cursor > 0 {
				m.cursor--
				m.updateViewportContent()
				// Ensure cursor is visible in viewport
				m.ensureCursorVisible()
			}
			return m, nil

		case "down":
			if m.cursor < len(m.filteredItems)-1 {
				m.cursor++
				m.updateViewportContent()
				// Ensure cursor is visible in viewport
				m.ensureCursorVisible()
			}
			return m, nil

		case " ":
			// Toggle current item
			if len(m.filteredItems) > 0 {
				path := m.filteredItems[m.cursor].Path
				m.toggleItem(path)
				m.updateViewportContent()
			}
			return m, nil

		case "pgup":
			m.viewport.HalfViewUp()
			return m, nil

		case "pgdown":
			m.viewport.HalfViewDown()
			return m, nil

		case "home":
			if len(m.filteredItems) > 0 {
				m.cursor = 0
				m.viewport.GotoTop()
				m.updateViewportContent()
			}
			return m, nil

		case "end":
			if len(m.filteredItems) > 0 {
				m.cursor = len(m.filteredItems) - 1
				m.viewport.GotoBottom()
				m.updateViewportContent()
			}
			return m, nil

		case "ctrl+a":
			// Select all currently filtered items
			for _, it := range m.filteredItems {
				m.selected[it.Path] = true
				// If it's a directory, recursively select all children
				if it.IsDir {
					m.toggleChildren(it.Path, true)
				}
			}
			// Recalculate total token count
			m.recalculateTotalTokenCount()
			m.updateViewportContent()
			return m, nil

		case "ctrl+q":
			// Deselect all currently filtered items
			for _, it := range m.filteredItems {
				m.selected[it.Path] = false
				// If it's a directory, recursively deselect all children
				if it.IsDir {
					m.toggleChildren(it.Path, false)
				}
			}
			// Recalculate total token count
			m.recalculateTotalTokenCount()
			m.updateViewportContent()
			return m, nil
		}
	}

	// Handle text input updates
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	// If the search term changed, update our filtered list
	newSearchTerm := m.textInput.Value()
	if newSearchTerm != m.searchTerm {
		m.searchTerm = newSearchTerm
		m.refilter()
		m.updateViewportContent()
	}

	// Handle viewport updates
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the TUI screen. We'll show:
//  1. The search input on top
//  2. A two‐column layout: file list on left, preview on right
func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Build the header with search input
	headerView := m.textInput.View() + "\n"

	// Show the viewport with our scrollable content
	listView := m.viewport.View()

	// Build the footer with status and usage hint
	statusLine := fmt.Sprintf(
		"%d/%d items, %d selected, %d tokens total",
		len(m.filteredItems),
		len(m.allItems),
		len(m.selected),
		m.totalTokenCount,
	)
	usageHint := "(↑/↓ to navigate, Space to toggle, Enter to confirm, Esc/Ctrl+C to abort, Ctrl+A to select all, Ctrl+Q to deselect all)"
	footerView := fmt.Sprintf("\n%s\n%s", statusLine, usageHint)

	// Combine everything in the correct order: header, viewport, footer
	return fmt.Sprintf("%s%s%s", headerView, listView, footerView)
}

// updateViewportContent updates the content of the viewport based on the current state
func (m *model) updateViewportContent() {
	var sb strings.Builder

	// Build the list view with the filtered items
	for i, it := range m.filteredItems {
		// Determine if this item is selected
		selected := m.selected[it.Path]

		// Cursor indicator
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}

		// Selection indicator
		check := " "
		if selected {
			check = "✓"
		}

		// Directory indicator
		dirIndicator := ""
		if it.IsDir {
			dirIndicator = "/"
		}

		// Display the full path instead of using indentation
		path := it.Path

		// Add token count info for selected files
		displayPath := path
		if selected && !it.IsDir {
			percentage := 0.0
			if m.totalTokenCount > 0 {
				percentage = float64(it.TokenCount) / float64(m.totalTokenCount) * 100
			}
			displayPath = fmt.Sprintf("%s (%d tokens, %.1f%%)", path, it.TokenCount, percentage)
		}

		// Format the line with the full path
		line := fmt.Sprintf("%s [%s] %s%s", cursor, check, displayPath, dirIndicator)

		// Highlight the current line
		if i == m.cursor {
			line = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render(line)
		}

		// Add newline after styling to prevent lipgloss from affecting spacing
		sb.WriteString(line + "\n")
	}

	// Set the viewport content
	m.viewport.SetContent(sb.String())
}

// ensureCursorVisible makes sure the cursor is visible in the viewport
func (m *model) ensureCursorVisible() {
	// Calculate the line number of the cursor in the viewport content
	cursorLine := m.cursor

	// Get current viewport position
	top := m.viewport.YOffset
	bottom := m.viewport.YOffset + m.viewport.Height - 1

	if cursorLine < top {
		// Cursor is above viewport, scroll up
		m.viewport.SetYOffset(cursorLine)
	} else if cursorLine > bottom {
		// Cursor is below viewport, scroll down
		m.viewport.SetYOffset(cursorLine - m.viewport.Height + 1)
	}
}

// refilter updates m.filteredItems based on the current fuzzy search term.
func (m *model) refilter() {
	if m.searchTerm == "" {
		// No search term, show all items
		m.filteredItems = m.allItems
		m.cursor = min(m.cursor, len(m.filteredItems)-1)
		return
	}

	// Filter items based on fuzzy search
	var filtered []item
	for _, it := range m.allItems {
		if fuzzyMatch(it.Path, m.searchTerm) {
			filtered = append(filtered, it)
		}
	}

	m.filteredItems = filtered
	// Adjust cursor if it's now out of bounds
	if len(filtered) == 0 {
		m.cursor = 0
	} else {
		m.cursor = min(m.cursor, len(filtered)-1)
	}
}

// toggleItem toggles the given path. If it's a directory, recursively toggles everything under it.
func (m *model) toggleItem(path string) {
	idx, ok := m.lookup[path]
	if !ok {
		return
	}

	// Toggle the selected state
	m.selected[path] = !m.selected[path]

	// If it's a directory, toggle all children
	if m.allItems[idx].IsDir {
		m.toggleChildren(path, m.selected[path])
	}

	// Recalculate total token count
	m.recalculateTotalTokenCount()
}

// toggleChildren recursively sets the selected value for everything under dirPath.
func (m *model) toggleChildren(dirPath string, selected bool) {
	// Get the children of this directory
	children, ok := m.childrenMap[dirPath]
	if !ok {
		return
	}

	// Toggle each child
	for _, childPath := range children {
		m.selected[childPath] = selected

		// If this child is also a directory, recursively toggle its children
		idx, ok := m.lookup[childPath]
		if ok && m.allItems[idx].IsDir {
			m.toggleChildren(childPath, selected)
		}
	}
}

// fuzzyMatch performs fuzzy matching using the sahilm/fuzzy library.
// It returns true if the text matches the search term according to fuzzy matching rules.
func fuzzyMatch(text string, term string) bool {
	// If the term is empty, everything matches
	if term == "" {
		return true
	}

	// Create a slice with just the text we want to match
	haystack := []string{text}

	// Perform the fuzzy match
	matches := fuzzy.Find(term, haystack)

	// Return true if we got any matches
	return len(matches) > 0
}

// recalculateTotalTokenCount updates the total token count based on selected files
func (m *model) recalculateTotalTokenCount() {
	total := 0
	for path, selected := range m.selected {
		if selected {
			idx, ok := m.lookup[path]
			if ok && !m.allItems[idx].IsDir {
				// If we have a cached token count, use it
				if cachedCount, ok := m.tokenCache[path]; ok {
					total += cachedCount
				} else {
					// Otherwise calculate it
					tokenCount, err := m.tokenEstimator(path)
					if err != nil {
						log.Printf("Error estimating tokens for %s: %v", path, err)
					} else {
						m.tokenCache[path] = tokenCount
						m.allItems[idx].TokenCount = tokenCount
						total += tokenCount
					}
				}
			}
		}
	}
	m.totalTokenCount = total
}
