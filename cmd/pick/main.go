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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
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
	}

	// Start Bubble Tea
	if err := tea.NewProgram(m).Start(); err != nil {
		log.Fatal(err)
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
	return textinput.Blink
}

// Update is called when events occur (key presses, etc.). We handle them here,
// then return the updated model and an optional command.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If we’re quitting, no further updates needed
	if m.quitting {
		return m, tea.Quit
	}

	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "esc":
			m.quitting = true
			// No selection is printed, we just bail:
			return m, tea.Quit

		case "enter":
			// On Enter, print all selected items to stdout, then quit
			for path := range m.selected {
				fmt.Println(path)
			}
			m.quitting = true
			return m, tea.Quit

		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down":
			if m.cursor < len(m.filteredItems)-1 {
				m.cursor++
			}
			return m, nil

		case " ":
			// Space toggles selection of current item.
			if len(m.filteredItems) == 0 {
				return m, nil
			}
			currItem := m.filteredItems[m.cursor]
			m.toggleItem(currItem.Path)
			return m, nil
		}

	case tea.WindowSizeMsg:
		// Could handle resizing, if you want dynamic layouts. We'll ignore for now.
		return m, nil
	}

	// Let textInput handle updates (typing, backspace, etc.)
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	newSearchTerm := m.textInput.Value()

	// If the search term changed, re‐filter
	if newSearchTerm != m.searchTerm {
		m.searchTerm = newSearchTerm
		m.refilter()
	}

	return m, cmd
}

// View renders the TUI screen. We’ll show:
//  1. The search input on top
//  2. A two‐column layout: file list on left, preview on right
func (m model) View() string {
	if m.quitting {
		return ""
	}

	// If no files, show an error
	if len(m.allItems) == 0 {
		return "No files found.\n"
	}

	header := fmt.Sprintf("Pick your files from: %s\n", filepath.Dir(m.allItems[0].Path))
	searchView := m.textInput.View()

	// Build the list of filtered items on left
	var listBuilder strings.Builder
	for i, it := range m.filteredItems {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		check := " "
		if m.selected[it.Path] {
			check = "x"
		}
		// Show just the base name or full path? Let’s do relative to root for brevity.
		display := it.Path
		listBuilder.WriteString(fmt.Sprintf("[%s]%s %s\n", check, cursor, display))
	}

	// Right side: preview of current selection
	var previewBuilder strings.Builder
	if len(m.filteredItems) > 0 {
		curr := m.filteredItems[m.cursor]
		if curr.IsDir {
			previewBuilder.WriteString("Directory:\n")
			previewBuilder.WriteString(curr.Path)
		} else {
			lines, err := previewLines(curr.Path, 10)
			if err != nil {
				previewBuilder.WriteString(fmt.Sprintf("Error reading file: %v\n", err))
			} else {
				previewBuilder.WriteString(fmt.Sprintf("Preview of %s:\n\n", curr.Path))
				previewBuilder.WriteString(strings.Join(lines, "\n"))
			}
		}
	}

	// Layout: search box on top, then a line, then a 2-col split
	// For simplicity, we won’t do fancy width measuring.
	return fmt.Sprintf(
		"%s%s\n––––––––––––––––––––––––––––––––\n%s\n––––––––––––––––––––––––––––––––\n%s\n",
		header,
		searchView,
		columnize(listBuilder.String(), previewBuilder.String()),
		"(↑/↓ to navigate, Space to toggle, Enter to confirm, Esc/Ctrl+C to abort)",
	)
}

// refilter updates m.filteredItems based on the current fuzzy search term.
func (m *model) refilter() {
	if strings.TrimSpace(m.searchTerm) == "" {
		m.filteredItems = m.allItems
		m.cursor = 0
		return
	}

	var matches []item
	term := strings.ToLower(m.searchTerm)
	for _, it := range m.allItems {
		if fuzzyMatch(filepath.Base(it.Path), term) || fuzzyMatch(it.Path, term) {
			matches = append(matches, it)
		}
	}
	m.filteredItems = matches
	if m.cursor >= len(m.filteredItems) {
		m.cursor = len(m.filteredItems) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// toggleItem toggles the given path. If it's a directory, recursively toggles everything under it.
func (m *model) toggleItem(path string) {
	currentlySelected := m.selected[path]
	newVal := !currentlySelected
	m.selected[path] = newVal

	// If it's a dir, recursively toggle children
	idx, found := m.lookup[path]
	if !found {
		return
	}
	if !m.allItems[idx].IsDir {
		return
	}
	// BFS or DFS through children
	m.toggleChildren(path, newVal)
}

// toggleChildren recursively sets the selected value for everything under dirPath.
func (m *model) toggleChildren(dirPath string, selected bool) {
	childList := m.childrenMap[dirPath]
	for _, child := range childList {
		m.selected[child] = selected
		// If child is a directory, recurse
		childIdx := m.lookup[child]
		if m.allItems[childIdx].IsDir {
			m.toggleChildren(child, selected)
		}
	}
}

// previewLines returns the first n lines of the file at path.
func previewLines(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) >= n {
			break
		}
	}
	return lines, scanner.Err()
}

// fuzzyMatch is a trivial substring match ignoring case.
func fuzzyMatch(text string, term string) bool {
	text = strings.ToLower(text)
	return strings.Contains(text, term)
}

// columnize is a naive function to produce two columns: left and right, split by a gap.
func columnize(left string, right string) string {
	// Split them line by line
	leftLines := strings.Split(strings.TrimSuffix(left, "\n"), "\n")
	rightLines := strings.Split(strings.TrimSuffix(right, "\n"), "\n")

	// Determine how many lines in either
	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	var b strings.Builder
	for i := 0; i < maxLines; i++ {
		var l, r string
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		b.WriteString(fmt.Sprintf("%-50s  %s\n", l, r))
	}
	return b.String()
}
