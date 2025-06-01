package selection

import "testing"

func TestIsLockFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// JavaScript/Node.js
		{"npm lock file", "package-lock.json", true},
		{"npm lock file with path", "/path/to/package-lock.json", true},
		{"npm shrinkwrap", "npm-shrinkwrap.json", true},
		{"yarn lock", "yarn.lock", true},
		{"pnpm lock", "pnpm-lock.yaml", true},
		{"bun lock", "bun.lockb", true},

		// Go
		{"go sum", "go.sum", true},
		{"go sum with path", "/project/go.sum", true},

		// Python
		{"pipfile lock", "Pipfile.lock", true},
		{"poetry lock", "poetry.lock", true},
		{"pdm lock", "pdm.lock", true},
		{"requirements lock", "requirements.lock", true},

		// Ruby
		{"gemfile lock", "Gemfile.lock", true},

		// Rust
		{"cargo lock", "Cargo.lock", true},

		// PHP
		{"composer lock", "composer.lock", true},

		// .NET
		{"packages lock", "packages.lock.json", true},

		// Swift
		{"package resolved", "Package.resolved", true},

		// Dart/Flutter
		{"pubspec lock", "pubspec.lock", true},

		// Case insensitive check
		{"uppercase npm lock", "PACKAGE-LOCK.JSON", true},
		{"mixed case yarn", "Yarn.Lock", true},

		// Non-lock files
		{"regular json", "package.json", false},
		{"regular go file", "main.go", false},
		{"text file", "README.md", false},
		{"similar name", "package-lock.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLockFile(tt.path)
			if result != tt.expected {
				t.Errorf("isLockFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}
