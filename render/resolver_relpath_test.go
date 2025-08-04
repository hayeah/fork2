package render

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePartialPath_RelativeWithoutParent(t *testing.T) {
	// Create a test directory structure
	tmpDir := t.TempDir()

	// Create test files
	fooPath := filepath.Join(tmpDir, "foo.md")
	if err := os.WriteFile(fooPath, []byte("foo content"), 0644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	barPath := filepath.Join(subDir, "bar.md")
	if err := os.WriteFile(barPath, []byte("bar content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to subdir to test relative paths
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldCwd)

	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}

	// Create resolver with empty partials
	resolver := NewResolver("", os.DirFS(tmpDir))

	tests := []struct {
		name        string
		partialPath string
		wantContent string
		wantErr     bool
	}{
		{
			name:        "relative parent path without parent template",
			partialPath: "../foo.md",
			wantContent: "foo content",
			wantErr:     false,
		},
		{
			name:        "current dir path without parent template",
			partialPath: "./bar.md",
			wantContent: "bar content",
			wantErr:     false,
		},
		{
			name:        "parent dir reference without parent template",
			partialPath: "..",
			wantErr:     true, // Directory, not a file
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys, path, err := resolver.ResolvePartialPath(tt.partialPath, nil)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify we can open and read the file
			file, err := fsys.Open(path)
			if err != nil {
				t.Errorf("failed to open resolved file: %v", err)
				return
			}
			defer file.Close()

			info, err := file.Stat()
			if err != nil {
				t.Errorf("failed to stat file: %v", err)
				return
			}

			content := make([]byte, info.Size())
			_, err = file.Read(content)
			if err != nil {
				t.Errorf("failed to read file: %v", err)
				return
			}

			if string(content) != tt.wantContent {
				t.Errorf("got content %q, want %q", string(content), tt.wantContent)
			}
		})
	}
}

func TestResolvePartialPath_RelativeWithParent(t *testing.T) {
	// Create a test directory structure
	tmpDir := t.TempDir()

	// Create nested directories with templates
	parentDir := filepath.Join(tmpDir, "parent")
	childDir := filepath.Join(parentDir, "child")

	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test files
	parentFile := filepath.Join(parentDir, "parent.md")
	if err := os.WriteFile(parentFile, []byte("parent content"), 0644); err != nil {
		t.Fatal(err)
	}

	childFile := filepath.Join(childDir, "child.md")
	if err := os.WriteFile(childFile, []byte("child content"), 0644); err != nil {
		t.Fatal(err)
	}

	siblingFile := filepath.Join(parentDir, "sibling.md")
	if err := os.WriteFile(siblingFile, []byte("sibling content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create resolver and parent template
	fsys := os.DirFS(tmpDir)
	resolver := NewResolver("", fsys)

	parentTemplate := &Template{
		Path: "parent/parent.md",
		FS:   fsys,
	}

	tests := []struct {
		name        string
		partialPath string
		wantContent string
	}{
		{
			name:        "relative child path",
			partialPath: "./child/child.md",
			wantContent: "child content",
		},
		{
			name:        "relative sibling path",
			partialPath: "./sibling.md",
			wantContent: "sibling content",
		},
		{
			name:        "relative parent path from nested template",
			partialPath: "../sibling.md",
			wantContent: "sibling content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			childTemplate := &Template{
				Path: "parent/child/child.md",
				FS:   fsys,
			}

			// Use child template for ".." test, parent template for others
			tmpl := parentTemplate
			if tt.partialPath == "../sibling.md" {
				tmpl = childTemplate
			}

			resolvedFS, path, err := resolver.ResolvePartialPath(tt.partialPath, tmpl)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify we can open and read the file
			file, err := resolvedFS.Open(path)
			if err != nil {
				t.Errorf("failed to open resolved file: %v", err)
				return
			}
			defer file.Close()

			content, err := fs.ReadFile(resolvedFS, path)
			if err != nil {
				t.Errorf("failed to read file: %v", err)
				return
			}

			if string(content) != tt.wantContent {
				t.Errorf("got content %q, want %q", string(content), tt.wantContent)
			}
		})
	}
}
