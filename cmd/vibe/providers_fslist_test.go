package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hayeah/fork2/render"
)

func TestProvideFSList_CwdFirst(t *testing.T) {
	// Create test directories
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	workDir := filepath.Join(repoRoot, "subdir", "work")

	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Change to work directory
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldCwd)

	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}

	// Create app environment
	env := &AppEnv{
		RootPath:         RootPath(repoRoot),
		WorkingDirectory: WorkingDirectory(workDir),
	}

	args := OutCmd{}

	// Get filesystem list
	fsList, err := ProvideFSList(env, args)
	if err != nil {
		t.Fatalf("ProvideFSList failed: %v", err)
	}

	// Verify we have at least 2 filesystems (cwd and repo root)
	if len(fsList) < 2 {
		t.Fatalf("expected at least 2 filesystems, got %d", len(fsList))
	}

	// First filesystem should be CwdFS
	if _, ok := fsList[0].(*render.CwdFS); !ok {
		t.Errorf("expected first filesystem to be CwdFS, got %T", fsList[0])
	}

	// Verify CwdFS points to current directory
	if cwdFS, ok := fsList[0].(*render.CwdFS); ok {
		// Normalize paths to handle symlinks (e.g., /var -> /private/var on macOS)
		expectedBase, _ := filepath.EvalSymlinks(workDir)
		actualBase, _ := filepath.EvalSymlinks(cwdFS.Base)
		if actualBase != expectedBase {
			t.Errorf("expected CwdFS base to be %q, got %q", expectedBase, actualBase)
		}
	}
}

func TestProvideFSList_SameDirectory(t *testing.T) {
	// Create test directory
	tmpDir := t.TempDir()

	// Change to test directory
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldCwd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create app environment where root and cwd are the same
	env := &AppEnv{
		RootPath:         RootPath(tmpDir),
		WorkingDirectory: WorkingDirectory(tmpDir),
	}

	args := OutCmd{}

	// Get filesystem list
	fsList, err := ProvideFSList(env, args)
	if err != nil {
		t.Fatalf("ProvideFSList failed: %v", err)
	}

	// Should still have CwdFS as first, but no duplicate for repo root
	if _, ok := fsList[0].(*render.CwdFS); !ok {
		t.Errorf("expected first filesystem to be CwdFS, got %T", fsList[0])
	}

	// Count how many times we have a filesystem for tmpDir
	// (Should be only once since cwd and root are the same)
	dirFSCount := 0
	for i, fs := range fsList {
		switch fs.(type) {
		case *render.CwdFS:
			if i == 0 {
				dirFSCount++
			}
		}
	}

	if dirFSCount != 1 {
		t.Errorf("expected tmpDir to appear once in filesystem list, but found %d occurrences", dirFSCount)
	}
}

func TestProvideFSList_TemplatePathsIncluded(t *testing.T) {
	// Create test directories
	tmpDir := t.TempDir()
	customPath1 := filepath.Join(tmpDir, "custom1")
	customPath2 := filepath.Join(tmpDir, "custom2")

	if err := os.MkdirAll(customPath1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(customPath2, 0755); err != nil {
		t.Fatal(err)
	}

	env := &AppEnv{
		RootPath:         RootPath(tmpDir),
		WorkingDirectory: WorkingDirectory(tmpDir),
	}

	args := OutCmd{
		TemplatePaths: []string{customPath1, customPath2},
	}

	// Get filesystem list
	fsList, err := ProvideFSList(env, args)
	if err != nil {
		t.Fatalf("ProvideFSList failed: %v", err)
	}

	// Verify order: cwd, custom paths, then others
	expectedMinLen := 3 // cwd + 2 custom paths
	if len(fsList) < expectedMinLen {
		t.Fatalf("expected at least %d filesystems, got %d", expectedMinLen, len(fsList))
	}

	// First should be CwdFS
	if _, ok := fsList[0].(*render.CwdFS); !ok {
		t.Errorf("expected first filesystem to be CwdFS, got %T", fsList[0])
	}
}
