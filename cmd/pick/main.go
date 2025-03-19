package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// item represents each file or directory in the listing.
type item struct {
	Path     string
	IsDir    bool
	Children []string // immediate children (for toggling entire sub-tree)
}

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
	cursor   int
	quitting bool

	// Viewport for scrolling
	viewport viewport.Model
	ready    bool
}

// main is our entrypoint: parse args, collect the files, and run the Bubble Tea program.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [directory]\n", os.Args[0])
		os.Exit(1)
	}

	rootPath := os.Args[1]
	info, err := os.Stat(rootPath)
	if err != nil {
		log.Fatalf("Error accessing %s: %v", rootPath, err)
	}

	if !info.IsDir() {
		log.Fatalf("Not a directory: %s", rootPath)
	}

	// Gather files/dirs
	items, childrenMap, err := gatherFiles(rootPath)
	if err != nil {
		log.Fatalf("Failed to gather files: %v", err)
	}

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
		textInput:     ti,
		allItems:      items,
		filteredItems: items, // default to showing them all
		selected:      selected,
		lookup:        lookup,
		childrenMap:   childrenMap,
		viewport:      viewport.New(0, 0), // Will be properly sized in tea.WindowSizeMsg
		ready:         false,
	}

	// Start Bubble Tea
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}

	// On Enter, print all selected items to stdout, then quit
	// Convert selected files to slice, sort them, and filter out directories
	var selectedFiles []string
	for path := range m.selected {
		// Get the index of this path in allItems to check if it's a directory
		if idx, ok := m.lookup[path]; ok && !m.allItems[idx].IsDir {
			selectedFiles = append(selectedFiles, path)
		}
	}

	// Sort the selected files
	sort.Strings(selectedFiles)

	// Print the sorted, filtered files
	for _, path := range selectedFiles {
		fmt.Println(path)
	}
}

// gatherFiles recursively walks the directory and returns a sorted list of item
// plus a children map for toggling entire subtrees.
func gatherFiles(rootPath string) ([]item, map[string][]string, error) {
	var items []item
	childrenMap := make(map[string][]string)

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip the root itself? Usually we want to display it as a togglable folder as well.
		// If you want to skip, do:
		// if path == rootPath { return nil }
		isDir := d.IsDir()
		items = append(items, item{
			Path:  path,
			IsDir: isDir,
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Sort items by path for a consistent listing
	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})

	// Build childrenMap (immediate children only). We'll handle deeper toggling recursively.
	pathIndex := make(map[string]int, len(items))
	for i, it := range items {
		pathIndex[it.Path] = i
	}

	for _, it := range items {
		if it.IsDir {
			var childList []string
			// We'll gather direct children by checking whether:
			//   parent == it.Path
			// or path.Dir(child) == it.Path
			for _, possibleChild := range items {
				// If the dir of possibleChild is it.Path, consider it a child
				if filepath.Dir(possibleChild.Path) == it.Path && possibleChild.Path != it.Path {
					childList = append(childList, possibleChild.Path)
				}
			}
			childrenMap[it.Path] = childList
		}
	}

	return items, childrenMap, nil
}

// Init is the first function called by Bubble Tea. We return an initial command (or nil).
func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.EnterAltScreen)
}

// Update is called when events occur (key presses, etc.). We handle them here,
// then return the updated model and an optional command.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If we're quitting, no further updates needed
	if m.quitting {
		return m, tea.Quit
	}

	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Set the viewport height to fit the terminal
		headerHeight := 2 // Input field + blank line
		footerHeight := 2 // Status line + blank line
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - footerHeight

		if !m.ready {
			// This is the first time we're getting a window size
			m.ready = true
			// Update the viewport content immediately
			m.updateViewportContent()
		}

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "esc":
			m.quitting = true
			// No selection is printed, we just bail:
			return m, tea.Quit

		case "enter":
			m.quitting = true
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

	// Build the search input
	inputView := m.textInput.View()

	// Show the viewport with our scrollable content
	listView := m.viewport.View()

	// Status line showing total/filtered/selected counts
	statusLine := fmt.Sprintf(
		"%d/%d items, %d selected",
		len(m.filteredItems),
		len(m.allItems),
		len(m.selected),
	)

	// Usage hint
	usageHint := "(↑/↓ to navigate, Space to toggle, Enter to confirm, Esc/Ctrl+C to abort)"

	// Combine everything
	return fmt.Sprintf("%s\n\n%s\n\n%s\n%s", inputView, listView, statusLine, usageHint)
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

		// Calculate indentation based on directory depth
		// This ensures proper alignment of the file tree
		path := it.Path
		depth := strings.Count(path, string(filepath.Separator))
		// Adjust depth to start from 0 for the root
		rootDepth := strings.Count(m.allItems[0].Path, string(filepath.Separator))
		relativeDepth := depth - rootDepth
		indent := strings.Repeat("  ", relativeDepth)

		// Get just the base name for display
		displayName := filepath.Base(path)

		// Format the line with proper indentation
		line := fmt.Sprintf("%s [%s] %s%s%s\n", cursor, check, indent, displayName, dirIndicator)

		// Highlight the current line
		if i == m.cursor {
			line = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render(line)
		}

		sb.WriteString(line)
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
	// Get the index of this path in allItems
	idx, ok := m.lookup[path]
	if !ok {
		return
	}

	it := m.allItems[idx]
	// Toggle this item
	selected := !m.selected[path]
	m.selected[path] = selected

	// If it's a directory, recursively toggle all children
	if it.IsDir {
		m.toggleChildren(path, selected)
	}
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

// previewLines returns the first n lines of the file at path.
func previewLines(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for i := 0; i < n && scanner.Scan(); i++ {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// fuzzyMatch is a trivial substring match ignoring case.
func fuzzyMatch(text string, term string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(term))
}

// min returns the smaller of a and b
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
