You are a **code editing assistant**: You can fulfill edit requests and chat with the user about code or other questions. Provide complete instructions or code lines when replying with xml formatting.

### Capabilities

- Can create new files.
- Can rewrite entire files.
- Can perform partial search/replace modifications.
- Can delete existing files.

Avoid placeholders like `...` or `// existing code here`. Provide complete lines or code.

## Tools & Actions

1. **create** – Create a new file if it doesn’t exist.
2. **rewrite** – Replace the entire content of an existing file.
3. **modify** (search/replace) – For partial edits by search and replace.
4. **delete** – Remove a file entirely.

### **Format to Follow HEREDOC Diff Protocol**

```
:plan<HEREDOC
Add an email property to `User` via search/replace.
HEREDOC

:modify Models/User.swift

$description<HEREDOC
Add email property to User struct.
HEREDOC

$search<HEREDOC
struct User {
  let id: UUID
  var name: String
}
HEREDOC

$replace<HEREDOC
struct User {
    let id: UUID
    var name: String
    var email: String
}
HEREDOC
```

- A command starts with a line that has a leading `:`.
- A parameter starts with a line that has a leading `$`.
- A command may have parameters in lines that follow.

- A comment starts with a line that has a leading `#`.

- A command or a paramter may have a string payload.
- Heredoc: A heredoc value after a command or action, with `<`.
- If not using heredoc, the payload is the string up to the end of line.

## Format Guidelines

1. **plan**: Begin with a `:plan` block to explain your approach.
3. **modify,rewrite,create,delete**: Provide `:description` parameter to clarify each change.
4. **modify**: Provide code blocks enclosed by HEREDOC. Respect indentation exactly, ensuring the `$search` block matches the original source down to braces, spacing, and any comments. The new `$replace` block will replace the `$search` block, and should fit perfectly in the space left by it's removal.
5. **modify**: For changes to the same file, ensure that you use multiple change blocks, rather than separate file blocks.
6. **rewrite**: For large overhauls, use the `:rewrite` command.
7. **create**: For new files, put the full file in `$content` block.
8. **delete**: The file path to be removed provided as a payload to the command.


### Example: Nested HEREDOCs

If your output heredoc payload contains heredoc, choose a different UNIQUE heredoc string (e.g. PINEAPPLE, MAXIMUS, HEREDOC2, HEREDOCMETA, etc.) to delimit this output.

```
:modify path/heredoc.txt

$description<HEREDOC
make change to a HEREDOC payload
HEREDOC


$search<HEREDOC_2
$search<HEREDOC
heredoc payload
HEREDOC
HEREDOC_2

$replace<HEREDOC_2
$search<HEREDOC
new heredoc payload
HEREDOC
HEREDOC_2
```


### Example: Make Multiple Edits

To make multiple changes, for each change, you should issue a new `:modify` or `:rewrite` block.

```
:modify path/model.ts

$description<HEREDOC
add email to user model
HEREDOC

$search<HEREDOC
// User model
interface User {
    id: number;
    name: string;
}
HEREDOC

$replace<HEREDOC
// User model
interface User {
    id: number;
    name: string;
    email: string;
}
HEREDOC

:modify path/model.ts

$description<HEREDOC
add name to role model
HEREDOC

$search<HEREDOC
// Role model
interface Role {
    id: number;
    permissions: string[];
}
HEREDOC

$replace<HEREDOC
// Role model
interface Role {
    id: number;
    permissions: string[];
    name: string;
}
HEREDOC
```

### Example: Full File Rewrite

```
:plan<HEREDOC
Rewrite the entire User file to include an email property.
HEREDOC

:rewrite Models/User.swift

$description<HEREDOC
Full file rewrite with new email field
HEREDOC

$content<HEREDOC
import Foundation
struct User {
    let id: UUID
    var name: String
    var email: String

    init(name: String, email: String) {
        self.id = UUID()
        self.name = name
        self.email = email
    }
}
HEREDOC
```

### Example: Create New File

```
:plan<HEREDOC
Create a new RoundedButton for a custom Swift UIButton subclass.
HEREDOC

:create Views/RoundedButton.swift

$description<HEREDOC
Create custom RoundedButton class
HEREDOC

$content<HEREDOC
import UIKit
@IBDesignable
class RoundedButton: UIButton {
    @IBInspectable var cornerRadius: CGFloat = 0
}
HEREDOC
```


### Example: Delete a File

```
:plan<HEREDOC
Remove an obsolete file.
HEREDOC

:delete Obsolete/File.swift

$description<HEREDOC
Completely remove the file from the project
HEREDOC
```


### Example: Committing Changes to Git

After editing files, stage your changes using `git add`.

Provide space-separated file paths that you've modified, created, or deleted.

```
:exec git add

$args<HEREDOC
file1 file2 subpath/file3
HEREDOC
```

Once you've staged your files, commit the changes with a clear succint commit message:

```
:exec git commit

$args<HEREDOC
-m "Implement email property in User model"
HEREDOC
```

For larger changes, write a detailed commit messages.

```
:exec git commit

$args<HEREDOC
-m "Implement email property in User model" -m '
A detailed description of the changes you've made.

- point 1
- point 2
- point 3
'
HEREDOC
```

## Notes On Tool Uses

1. **modify** Avoid search blocks that are too short or too ambiguous. Single characters like `}` is too short.
2. **modify** The `$search` block must match the source code exactly—down to indentation, braces, spacing, and any comments. Even a minor mismatch causes failed merges.
3. **modify** Only replace exactly what you need. Avoid including entire functions or files if only a small snippet changes, and ensure the `$search` content is unique and easy to identify.
4. **rewrite** Use `rewrite` for major overhauls, and `modify` for smaller, localized edits. Rewrite requires the entire code to be replaced, so use it sparingly.
5. You can always **create** new files and **delete** existing files. Provide full code for create, and empty content for delete. Avoid creating files you know exist already.
6. If a file tree is provided, place your files logically within that structure. Respect the user’s relative or absolute paths.
9. **IMPORTANT** IF MAKING FILE CHANGES, YOU MUST USE THE AVAILABLE FORMATTING CAPABILITIES PROVIDED ABOVE - IT IS THE ONLY WAY FOR YOUR CHANGES TO BE APPLIED.
10. The final output must apply cleanly with no leftover syntax errors.
11. After making all your file changes, commit your changes to git using `:exec`.

## Repo Directory Tree

/Users/me/src/hayeah/prompt-experiments/fork2
├── cmd
│   ├── filemap
│   │   └── main.go
│   ├── test_gitignore
│   │   └── main.go
│   ├── test_prompt
│   │   └── main.go
│   ├── vibe
│   │   ├── templates
│   │   │   ├── base
│   │   │   └── coder
│   │   ├── ask.go
│   │   ├── ask_test.go
│   │   ├── diff-heredoc.md
│   │   ├── main.go
│   │   ├── merge.go
│   │   ├── render_context.go
│   │   ├── repoprompt-diff.md
│   │   ├── selectFiles.go
│   │   ├── select_ui.go
│   │   └── vibe_test.go
│   └── main.go
├── examples
│   ├── ask-diff-to-role.md
│   └── ask-diff-to-role.prompt.md
├── heredoc
│   ├── heredoc.examples
│   ├── heredoc.go
│   ├── heredoc.md
│   ├── heredoc_test.go
│   ├── scan_struct.go
│   └── scan_struct_test.go
├── ignore
│   └── ignore.go
├── merge
│   ├── action.go
│   ├── command.go
│   ├── create_test.go
│   ├── delete_test.go
│   ├── exec_test.go
│   ├── modify_test.go
│   └── rewrite_test.go
├── prompt
├── prompts
│   ├── environment_details.tmpl
│   └── system.md
├── render
│   ├── design.md
│   ├── render.go
│   └── render_test.go
├── tasks
│   ├── add-exec.md
│   ├── dot-vibe.md
│   ├── exec-prompt-user-yes.md
│   ├── fix-exec-args.md
│   ├── generate-readme.md
│   ├── instruct-exec-and-git.md
│   ├── multi-select.md
│   └── output-with-render.md
├── .DS_Store
├── .gitignore
├── HACK.md
├── Makefile
├── README.md
├── app.go
├── ask-diff-to-role.md
├── cfg.toml
├── filemap.go
├── filemap_cli.go
├── go.mod
├── go.sum
├── modd.conf
├── out.md
├── prompt.go
├── prompt.md
├── wire.go
└── wire_gen.go


## Selected Files

File: cmd/vibe/ask.go
```go
package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/atotto/clipboard"
	"github.com/pkoukk/tiktoken-go"
)

// AskCmd contains the arguments for the 'ask' subcommand
type AskCmd struct {
	TokenEstimator string   `arg:"--token-estimator" help:"Token count estimator to use: 'simple' (size/4) or 'tiktoken'" default:"simple"`
	All            bool     `arg:"-a,--all" help:"Select all files and output immediately"`
	Copy           bool     `arg:"-c,--copy" help:"Copy output to clipboard instead of stdout"`
	Diff           bool     `arg:"--diff" help:"Enable diff output format"`
	Select         []string `arg:"--select,separate" help:"Select files matching fuzzy pattern and output immediately (can be specified multiple times)"`
	SelectRegex    string   `arg:"--select-re" help:"Select files matching regex pattern and output immediately"`
	Instruction    string   `arg:"positional" help:"User instruction or path to instruction file"`
}

// Merge merges src fields into the current AskCmd instance, with the current instance
// having precedence for any non-empty value or true boolean. So if this.All is already true,
// it stays true. If this.All is false, we take src's value. Same pattern
// for the rest.
func (cmd *AskCmd) Merge(src *AskCmd) {
	if src == nil {
		return
	}

	// If TokenEstimator is empty, overwrite it
	if cmd.TokenEstimator == "" {
		cmd.TokenEstimator = src.TokenEstimator
	}
	// Booleans: once set to true, keep them
	cmd.All = cmd.All || src.All
	cmd.Copy = cmd.Copy || src.Copy
	cmd.Diff = cmd.Diff || src.Diff

	// Strings: if empty, overwrite
	if len(cmd.Select) == 0 {
		cmd.Select = src.Select
	}
	if cmd.SelectRegex == "" {
		cmd.SelectRegex = src.SelectRegex
	}
	if cmd.Instruction == "" {
		cmd.Instruction = src.Instruction
	}
}

//go:embed repoprompt-diff.md
var diffPrompt string

//go:embed diff-heredoc.md
var diffHeredocPrompt string

// AskRunner encapsulates the state and behavior for the file picker
type AskRunner struct {
	Args            AskCmd
	RootPath        string
	Items           []item
	ChildrenMap     map[string][]string
	TokenEstimator  TokenEstimator
	UserInstruction string
}

// NewAskRunner creates and initializes a new PickRunner
func NewAskRunner(cmdArgs AskCmd, rootPath string) (*AskRunner, error) {
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("error accessing %s: %v", rootPath, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", rootPath)
	}

	// Parse front matter from instruction
	userInstruction, err := parseInstructionWithFrontMatter(&cmdArgs)
	if err != nil {
		return nil, err
	}

	// Select the token estimator based on the flag
	var tokenEstimator TokenEstimator
	switch cmdArgs.TokenEstimator {
	case "tiktoken":
		tokenEstimator = estimateTokenCountTiktoken
	case "simple":
		tokenEstimator = estimateTokenCountSimple
	default:
		return nil, fmt.Errorf("unknown token estimator: %s", cmdArgs.TokenEstimator)
	}

	return &AskRunner{
		Args:            cmdArgs,
		RootPath:        rootPath,
		TokenEstimator:  tokenEstimator,
		UserInstruction: userInstruction,
	}, nil
}

// Run executes the file picking process
func (r *AskRunner) Run() error {
	// Gather files/dirs
	var err error
	r.Items, r.ChildrenMap, err = gatherFiles(r.RootPath)
	if err != nil {
		return fmt.Errorf("failed to gather files: %v", err)
	}

	// Filter phase: select files either automatically or interactively
	selectedFiles, err := r.filterFiles()
	if err != nil {
		return err
	}

	// If no files were selected (user aborted), return early
	if selectedFiles == nil {
		return nil
	}

	// Output phase: generate user instruction and handle output
	if err := r.handleOutput(selectedFiles); err != nil {
		return err
	}

	// Calculate and report token count after output is handled
	totalTokenCount, err := calculateTokenCount(selectedFiles, r.TokenEstimator)
	if err != nil {
		return fmt.Errorf("error calculating token count: %v", err)
	}

	// Print total token count to stderr
	fmt.Fprintf(os.Stderr, "Total tokens: %d\n", totalTokenCount)

	return nil
}

// filterFiles handles the file selection phase, either automatically or interactively
func (r *AskRunner) filterFiles() ([]string, error) {
	var selectedFiles []string
	var err error

	if r.Args.All {
		// Select all files
		selectedFiles, err = selectAllFiles(r.Items)
	} else if len(r.Args.Select) > 0 {
		// Select files matching fuzzy patterns
		filesSet := make(map[string]struct{})

		for _, pattern := range r.Args.Select {
			patternFiles, patternErr := selectFuzzyFiles(r.Items, pattern)
			if patternErr != nil {
				return nil, fmt.Errorf("error selecting files with pattern '%s': %w", pattern, patternErr)
			}

			// Add to set to avoid duplicates
			for _, file := range patternFiles {
				filesSet[file] = struct{}{}
			}
		}

		// Convert set to slice
		selectedFiles = make([]string, 0, len(filesSet))
		for file := range filesSet {
			selectedFiles = append(selectedFiles, file)
		}

		err = nil // No errors encountered
	} else if r.Args.SelectRegex != "" {
		// Select files matching regex pattern
		pattern := r.Args.SelectRegex
		selectedFiles, err = selectRegexFiles(r.Items, pattern)
	} else {
		// Interactive selection
		selectedFiles, _, err = selectFilesInteractively(r.Items, r.ChildrenMap, r.TokenEstimator)
		if err != nil {
			return nil, err
		}

		// If no files were selected (user aborted), return early
		if selectedFiles == nil {
			return nil, nil
		}
	}

	if err != nil {
		return nil, err
	}

	return selectedFiles, nil
}

// calculateTokenCount calculates the total token count for a list of file paths
func calculateTokenCount(filePaths []string, tokenEstimator TokenEstimator) (int, error) {
	totalTokenCount := 0

	for _, path := range filePaths {
		tokenCount, err := tokenEstimator(path)
		if err != nil {
			log.Printf("Error estimating tokens for %s: %v", path, err)
		} else {
			totalTokenCount += tokenCount
		}
	}

	return totalTokenCount, nil
}

// handleOutput processes the user instruction and outputs the result
func (r *AskRunner) handleOutput(selectedFiles []string) error {
	// Create a new VibeContext
	vibeCtx, err := NewVibeContext(r)
	if err != nil {
		return fmt.Errorf("failed to create vibe context: %v", err)
	}

	var buf bytes.Buffer
	var out io.Writer
	out = os.Stdout
	if r.Args.Copy {
		out = &buf
	}

	role := "<base>"
	if r.Args.Diff {
		role = "<coder>"
	}

	// Use VibeContext.WriteOutput
	err = vibeCtx.WriteOutput(out, r.Args.Instruction, role, selectedFiles)
	if err != nil {
		return err
	}

	// Handle output based on --copy flag
	if r.Args.Copy {
		// Copy buffer contents to clipboard
		err = clipboard.WriteAll(buf.String())
		if err != nil {
			return fmt.Errorf("failed to copy to clipboard: %v", err)
		}

		fmt.Fprintln(os.Stderr, "Output copied to clipboard")
		return nil
	}

	return nil
}

// findRepoRoot returns the path to the repository root by looking for a .git directory.
// It starts from the given directory and moves up until it finds .git or reaches the filesystem root.
func findRepoRoot(startPath string) (string, error) {
	currentPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	for {
		gitPath := filepath.Join(currentPath, ".git")
		if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
			return currentPath, nil
		}

		// Check if we've reached the filesystem root
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// We've reached the root without finding .git
			return "", fmt.Errorf("no .git directory found up to filesystem root")
		}

		currentPath = parentPath
	}
}

// loadVibeFiles loads .vibe.md files from the current directory up to the repo root.
// It returns the content of all found .vibe.md files concatenated in order from repo root down to current dir.
func loadVibeFiles(startPath string) (string, error) {
	// Find the repo root
	repoRoot, err := findRepoRoot(startPath)
	if err != nil {
		// Not finding a repo root is not a fatal error, we just won't load .vibe.md files
		return "", nil
	}

	// Build the list of directories from repo root to current dir
	var dirs []string
	currentPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	// Start with the repo root
	dirs = append(dirs, repoRoot)

	// Add all directories from repo root down to current dir (if different)
	if currentPath != repoRoot {
		rel, err := filepath.Rel(repoRoot, currentPath)
		if err != nil {
			return "", err
		}

		parts := strings.Split(rel, string(filepath.Separator))
		path := repoRoot
		for _, part := range parts {
			path = filepath.Join(path, part)
			if path != repoRoot { // Don't add repo root twice
				dirs = append(dirs, path)
			}
		}
	}

	// Load .vibe.md files from each directory
	var content strings.Builder
	for _, dir := range dirs {
		vibePath := filepath.Join(dir, ".vibe.md")
		if info, err := os.Stat(vibePath); err == nil && !info.IsDir() {
			data, err := os.ReadFile(vibePath)
			if err != nil {
				return "", fmt.Errorf("error reading %s: %v", vibePath, err)
			}
			content.WriteString("<!-- From " + vibePath + " -->\n")
			content.Write(data)
			content.WriteString("\n\n")
		}
	}

	return content.String(), nil
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

		if node.path == rootPath {
			// Use the absolute path for the root node
			absPath, err := filepath.Abs(rootPath)
			if err != nil {
				absPath = rootPath // Fallback
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

	if err := writeTreeNode(rootNode, "", true); err != nil {
		return err
	}

	return nil
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

// parseFrontMatter attempts to parse front matter from data. It returns a *AskCmd
// and the remainder of the data after the front matter. If there's no front matter,
// returns nil, original data, no error. If front matter is found but not properly
// closed, returns an error.
func parseFrontMatter(data []byte) (*AskCmd, []byte, error) {
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) == 0 {
		return nil, data, nil
	}

	// Check if the first line is '---' or '+++'
	delimiter := ""
	if bytes.Equal(lines[0], []byte("---")) || bytes.Equal(lines[0], []byte("+++")) {
		delimiter = string(lines[0])
	} else {
		// no front matter
		return nil, data, nil
	}

	// Find the closing delimiter
	var frontMatterLines []byte
	foundEnd := false
	i := 1
	for ; i < len(lines); i++ {
		if bytes.Equal(lines[i], []byte(delimiter)) {
			foundEnd = true
			break
		}
		frontMatterLines = append(frontMatterLines, lines[i]...)
		frontMatterLines = append(frontMatterLines, '\n')
	}

	if !foundEnd {
		return nil, nil, fmt.Errorf("front matter not closed with %s", delimiter)
	}

	parsedCmd, err := parseFlags(frontMatterLines)
	if err != nil {
		return nil, nil, err
	}

	// remainder is everything after the closing delimiter line
	remainder := bytes.Join(lines[i+1:], []byte("\n"))
	return parsedCmd, remainder, nil
}

// parseFlags interprets lines of text as flags for AskCmd using go-arg library
// by temporarily overriding os.Args.
func parseFlags(frontMatter []byte) (*AskCmd, error) {
	cmd := &AskCmd{}

	// Convert front matter to a single line of args
	var args []string
	allLines := bytes.Split(frontMatter, []byte("\n"))
	for _, line := range allLines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// A line might have multiple flags: e.g. "--copy --diff"
		parts := strings.Fields(string(line))
		args = append(args, parts...)
	}

	// Skip parsing if no args
	if len(args) == 0 {
		return cmd, nil
	}

	// Temporarily save original args
	originalArgs := os.Args
	defer func() {
		// Restore original args
		os.Args = originalArgs
	}()

	// Override with our custom args (preserving program name as first arg)
	os.Args = append([]string{originalArgs[0]}, args...)

	// Use go-arg to parse the flags into our struct
	parser, err := arg.NewParser(arg.Config{}, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %v", err)
	}

	if err := parser.Parse(os.Args); err != nil {
		return nil, fmt.Errorf("failed to parse front matter flags: %v", err)
	}

	return cmd, nil
}

// parseInstructionWithFrontMatter parses the instruction from a file or string,
// extracts any front matter, and returns the remaining instruction content.
// It modifies the provided AskCmd with any flags found in the front matter.
func parseInstructionWithFrontMatter(cmdArgs *AskCmd) (string, error) {
	if cmdArgs.Instruction == "" {
		return "", nil
	}

	// First step: Read the instruction content (from file or use as-is)
	instructionContent, err := readInstructionContent(cmdArgs.Instruction)
	if err != nil {
		return "", err
	}

	// Second step: Parse front matter from the content
	frontCmd, remainder, err := parseFrontMatter(instructionContent)
	if err != nil && frontCmd == nil {
		// invalid front matter => error
		return "", err
	}

	// Apply any front matter flags to command args
	if frontCmd != nil {
		cmdArgs.Merge(frontCmd)
	}

	return string(remainder), nil
}

// readInstructionContent reads the instruction content from a file if the path exists,
// otherwise returns the instruction string as-is
func readInstructionContent(instruction string) ([]byte, error) {
	// Check if the instruction is a file path
	fileInfo, err := os.Stat(instruction)
	if err == nil && !fileInfo.IsDir() {
		// It's a file, read its content
		content, err := os.ReadFile(instruction)
		if err != nil {
			return nil, fmt.Errorf("failed to read instruction file: %v", err)
		}
		return content, nil
	}

	// It's not a file, return the instruction string itself
	return []byte(instruction), nil
}
```

File: cmd/vibe/ask_test.go
```go
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFrontMatter_NoFrontMatter(t *testing.T) {
	data := []byte(`some instructions
line2
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.Nil(t, cmd, "cmd should be nil if no front matter")
	assert.Equal(t, data, remainder)
	assert.NoError(t, err)
}

func TestParseFrontMatter_Delimited(t *testing.T) {
	data := []byte(`+++
--diff
--all
+++
some instructions
line2
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	assert.True(t, cmd.All)
	assert.True(t, cmd.Diff)
	assert.Equal(t, []byte("some instructions\nline2\n"), remainder)
}

func TestParseFrontMatter_UnclosedDelimiter(t *testing.T) {
	data := []byte(`+++
--select=merge/.go
some instructions
line2
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.Nil(t, cmd)
	assert.Nil(t, remainder)
	assert.Error(t, err)
}

func TestAskCmd_Merge_Precedence(t *testing.T) {
	dst := &AskCmd{
		TokenEstimator: "simple",
		Diff:           true,
		Instruction:    "CLI instructions",
	}
	src := &AskCmd{
		TokenEstimator: "tiktoken",
		Diff:           false,
		Select:         []string{"some/path"},
		Instruction:    "front matter instructions",
	}
	dst.Merge(src)
	assert.Equal(t, "simple", dst.TokenEstimator, "dst wins if non-empty")
	assert.True(t, dst.Diff, "dst wins if it's true")
	assert.Equal(t, []string{"some/path"}, dst.Select, "src sets select if dst was empty")
	assert.Equal(t, "CLI instructions", dst.Instruction, "dst instruction wins if present")
}

func TestParseFrontMatter_MultipleFlags(t *testing.T) {
	data := []byte(`---
--copy --select-re=xyz
---
real content
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	assert.True(t, cmd.Copy)
	assert.Equal(t, "xyz", cmd.SelectRegex)
	assert.Equal(t, []byte("real content\n"), remainder)
}

func TestParseFrontMatter_TokenEstimator(t *testing.T) {
	data := []byte(`+++
--token-estimator=tiktoken
+++
user instructions
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	assert.Equal(t, "tiktoken", cmd.TokenEstimator)
	assert.Equal(t, []byte("user instructions\n"), remainder)
}

func TestParseFrontMatter_InvalidClosing(t *testing.T) {
	data := []byte(`+++
--copy
---
content
`)
	cmd, remainder, err := parseFrontMatter(data)
	assert.Error(t, err)
	assert.Nil(t, cmd)
	assert.Nil(t, remainder)
}

func TestNewAskRunner_FrontMatterParsing(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Test with instruction string containing front matter
	cmdArgs := AskCmd{
		TokenEstimator: "simple",
		Instruction:    "---\n--diff\n---\nThis is a test instruction",
	}

	runner, err := NewAskRunner(cmdArgs, tempDir)
	assert.NoError(t, err)
	assert.NotNil(t, runner)
	assert.True(t, runner.Args.Diff)
	assert.Equal(t, "This is a test instruction", runner.UserInstruction)
}
```



## User Task

+++
--select cmd/vibe/askgo
--copy
+++

from: d73eaad

- vibe: change `diff` flag to `role`
	- remove `--diff`
	- add the "--role" flag
		- default to coder
	- when rendering in handleOutput, wrap the specified role in "<...>"
- fix tests


## Final Reminders

IMPORTANT: Output your response in the format described in the instructions. Quote the response as code for display, so user can read it and copy it easily.

In follow up messages, assume previous commands have already applied.
