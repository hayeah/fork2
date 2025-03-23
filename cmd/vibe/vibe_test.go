package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindRepoRoot(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()

	// Create a mock repository structure
	repoRoot := filepath.Join(tempDir, "repo")
	subDir := filepath.Join(repoRoot, "subdir")
	subSubDir := filepath.Join(subDir, "subsubdir")

	// Create the directories
	err := os.MkdirAll(subSubDir, 0755)
	assert.NoError(t, err)

	// Create a .git directory at the repo root
	gitDir := filepath.Join(repoRoot, ".git")
	err = os.Mkdir(gitDir, 0755)
	assert.NoError(t, err)

	// Test finding repo root from various directories
	tests := []struct {
		startPath string
		expected  string
	}{
		{repoRoot, repoRoot},
		{subDir, repoRoot},
		{subSubDir, repoRoot},
	}

	for _, test := range tests {
		root, err := findRepoRoot(test.startPath)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, root)
	}

	// Test with a directory that is not in a repo
	root, err := findRepoRoot(tempDir)
	assert.Error(t, err)
	assert.Empty(t, root)
}

func TestLoadVibeFiles(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()

	// Create a mock repository structure
	repoRoot := filepath.Join(tempDir, "repo")
	subDir := filepath.Join(repoRoot, "subdir")
	subSubDir := filepath.Join(subDir, "subsubdir")

	// Create the directories
	err := os.MkdirAll(subSubDir, 0755)
	assert.NoError(t, err)

	// Create a .git directory at the repo root
	gitDir := filepath.Join(repoRoot, ".git")
	err = os.Mkdir(gitDir, 0755)
	assert.NoError(t, err)

	// Create .vibe.md files at different levels
	repoVibeContent := "# Repo level configuration\nsome config here"
	subDirVibeContent := "# Subdir level configuration\nmore config here"
	subSubDirVibeContent := "# SubSubdir level configuration\neven more config"

	err = os.WriteFile(filepath.Join(repoRoot, ".vibe.md"), []byte(repoVibeContent), 0644)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(subDir, ".vibe.md"), []byte(subDirVibeContent), 0644)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(subSubDir, ".vibe.md"), []byte(subSubDirVibeContent), 0644)
	assert.NoError(t, err)

	// Test loading vibe files from the deepest directory
	content, err := loadVibeFiles(subSubDir)
	assert.NoError(t, err)

	// Verify content includes all .vibe.md files
	assert.Contains(t, content, repoVibeContent)
	assert.Contains(t, content, subDirVibeContent)
	assert.Contains(t, content, subSubDirVibeContent)

	// Verify the correct ordering (repo root first, then down to current dir)
	repoPos := strings.Index(content, repoVibeContent)
	subDirPos := strings.Index(content, subDirVibeContent)
	subSubDirPos := strings.Index(content, subSubDirVibeContent)

	assert.True(t, repoPos < subDirPos, "Repo root .vibe.md should appear before subdir .vibe.md")
	assert.True(t, subDirPos < subSubDirPos, "Subdir .vibe.md should appear before subsubdir .vibe.md")
}
