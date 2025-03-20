package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hayeah/fork2"
	"github.com/hayeah/fork2/ignore"
	"github.com/pkoukk/tiktoken-go"
)

// Args defines the command-line arguments
type Args struct {
	TokenEstimator string `arg:"--token-estimator" help:"Token count estimator to use: 'simple' (size/4) or 'tiktoken'" default:"simple"`
	Directory      string `arg:"positional,required" help:"Directory to pick files from"`
}

// item represents each file or directory in the listing.
type item struct {
	Path       string
	IsDir      bool
	Children   []string // immediate children (for toggling entire sub-tree)
	TokenCount int      // Number of tokens in this file
}

// TokenEstimator is a function type that estimates token count for a file
type TokenEstimator func(filePath string) (int, error)

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

// main is our entrypoint: parse args and run the application
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run parses args, collects the files, and runs the Bubble Tea program
func run() error {
	// Parse command-line arguments
	var args Args
	arg.MustParse(&args)

	rootPath := args.Directory
	info, err := os.Stat(rootPath)
	if err != nil {
		return fmt.Errorf("error accessing %s: %v", rootPath, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", rootPath)
	}

	// Select the token estimator based on the flag
	var tokenEstimator TokenEstimator
	switch args.TokenEstimator {
	case "tiktoken":
		tokenEstimator = estimateTokenCountTiktoken
	case "simple":
		tokenEstimator = estimateTokenCountSimple
	default:
		return fmt.Errorf("unknown token estimator: %s", args.TokenEstimator)
	}

	// Gather files/dirs
	items, childrenMap, err := gatherFiles(rootPath)
	if err != nil {
		return fmt.Errorf("failed to gather files: %v", err)
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
		return err
	}

	// Get the final state of the model
	finalM, ok := finalModel.(model)
	if !ok {
		return fmt.Errorf("could not get final model state")
	}

	// Only generate output if we're exiting with confirmation (Enter)
	if finalM.exitState != ExitStateConfirm {
		return nil
	}

	// On Enter, print all selected items to stdout
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

	// Generate directory tree structure and write to stdout using the already gathered items
	err = generateDirectoryTree(os.Stdout, rootPath, m.allItems)
	if err != nil {
		return fmt.Errorf("failed to generate directory tree: %v", err)
	}

	// Write the file map of selected files
	err = fork2.WriteFileMap(os.Stdout, selectedFiles, rootPath)
	if err != nil {
		return fmt.Errorf("failed to write file map: %v", err)
	}

	// Print total token count to stderr
	fmt.Fprintf(os.Stderr, "Total tokens: %d\n", finalM.totalTokenCount)

	return nil
}

// generateDirectoryTree creates a tree-like structure for the directory and writes it to the provided writer
// using the items already gathered by gatherFiles
func generateDirectoryTree(w io.Writer, rootPath string, items []item) error {
	type treeNode struct {
		path     string
		name     string
		isDir    bool
		children []*treeNode
	}

	// Create a map to store nodes by their path
	nodeMap := make(map[string]*treeNode)

	// Create the root node
	rootName := filepath.Base(rootPath)
	rootNode := &treeNode{
		path:     rootPath,
		name:     rootName,
		isDir:    true,
		children: []*treeNode{},
	}
	nodeMap[rootPath] = rootNode

	// Process the items to build the tree structure
	for _, item := range items {
		// Skip the root itself
		if item.Path == rootPath {
			continue
		}

		name := filepath.Base(item.Path)
		parent := filepath.Dir(item.Path)

		// Create a new node if it doesn't exist yet
		if _, exists := nodeMap[item.Path]; !exists {
			node := &treeNode{
				path:     item.Path,
				name:     name,
				isDir:    item.IsDir,
				children: []*treeNode{},
			}
			nodeMap[item.Path] = node
		}

		// Add this node to its parent's children
		if parentNode, ok := nodeMap[parent]; ok {
			if childNode, ok := nodeMap[item.Path]; ok {
				parentNode.children = append(parentNode.children, childNode)
			}
		}
	}

	// Write the opening file_map tag
	fmt.Fprintln(w, "<file_map>")

	// Function to recursively build the tree string
	var writeTreeNode func(node *treeNode, prefix string, isLast bool) error
	writeTreeNode = func(node *treeNode, prefix string, isLast bool) error {
		// Sort children by name
		sort.Slice(node.children, func(i, j int) bool {
			// Directories first, then files
			if node.children[i].isDir != node.children[j].isDir {
				return node.children[i].isDir
			}
			return node.children[i].name < node.children[j].name
		})

		// Add this node to the result
		if node.path == rootPath {
			// Use the absolute path for the root node
			absPath, err := filepath.Abs(rootPath)
			if err != nil {
				absPath = rootPath // Fallback to rootPath if Abs fails
			}
			_, err = fmt.Fprintln(w, absPath)
			if err != nil {
				return err
			}
		} else {
			connector := "├── "
			if isLast {
				connector = "└── "
			}
			_, err := fmt.Fprintln(w, prefix+connector+node.name)
			if err != nil {
				return err
			}
		}

		// Add children
		for i, child := range node.children {
			isLastChild := i == len(node.children)-1
			newPrefix := prefix
			if node.path != rootPath {
				if isLast {
					newPrefix += "    "
				} else {
					newPrefix += "│   "
				}
			}
			if err := writeTreeNode(child, newPrefix, isLastChild); err != nil {
				return err
			}
		}

		return nil
	}

	// Write the tree structure
	if err := writeTreeNode(rootNode, "", true); err != nil {
		return err
	}

	// Write the closing file_map tag
	fmt.Fprintln(w, "</file_map>")

	return nil
}

// gatherFiles recursively walks the directory and returns a sorted list of item
// plus a children map for toggling entire subtrees.
// It respects .gitignore patterns in the directory.
func gatherFiles(rootPath string) ([]item, map[string][]string, error) {
	var items []item
	childrenMap := make(map[string][]string)

	ig, err := ignore.NewIgnore(rootPath)
	if err != nil {
		return nil, nil, err
	}

	// Use WalkDirGitIgnore to walk the directory tree while respecting gitignore patterns
	err = ig.WalkDir(rootPath, func(path string, d os.DirEntry, isDir bool) error {
		items = append(items, item{
			Path:       path,
			IsDir:      isDir,
			TokenCount: 0, // Initialize token count to 0
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

// fuzzyMatch is a trivial substring match ignoring case.
func fuzzyMatch(text string, term string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(term))
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

// min returns the smaller of a and b
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// estimateTokenCount estimates the number of tokens in a file
// This is kept for backward compatibility
func estimateTokenCount(filePath string) (int, error) {
	return estimateTokenCountSimple(filePath)
}

// estimateTokenCountSimple estimates tokens using the simple size/4 method
func estimateTokenCountSimple(filePath string) (int, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}

	// Simple estimation: 1 token per 4 characters
	return len(data) / 4, nil
}

// estimateTokenCountTiktoken estimates tokens using the tiktoken-go library
func estimateTokenCountTiktoken(filePath string) (int, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}

	// Use tiktoken-go to count tokens
	tke, err := tiktoken.GetEncoding("cl100k_base") // Using the same encoding as GPT-4
	if err != nil {
		return 0, fmt.Errorf("failed to get tiktoken encoding: %v", err)
	}

	tokens := tke.Encode(string(data), nil, nil)
	return len(tokens), nil
}
