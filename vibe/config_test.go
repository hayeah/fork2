package vibe

import (
	"testing"

	"github.com/BurntSushi/toml"
)

func TestTemplateNamesUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TemplateNames
	}{
		{
			name:     "single template",
			input:    `before = "foo"`,
			expected: TemplateNames{"foo"},
		},
		{
			name:     "multiple templates",
			input:    `before = "foo;bar;baz"`,
			expected: TemplateNames{"foo", "bar", "baz"},
		},
		{
			name:     "with spaces",
			input:    `before = " foo ; bar ; baz "`,
			expected: TemplateNames{"foo", "bar", "baz"},
		},
		{
			name:     "empty string",
			input:    `before = ""`,
			expected: nil,
		},
		{
			name:     "empty segments ignored",
			input:    `before = "foo;;bar;;baz"`,
			expected: TemplateNames{"foo", "bar", "baz"},
		},
		{
			name: "both before and after",
			input: `before = "header;nav"
after = "footer;analytics"`,
			expected: TemplateNames{"header", "nav"}, // We'll check 'before' field
		},
		{
			name:     "layout field single",
			input:    `layout = "base"`,
			expected: TemplateNames{"base"},
		},
		{
			name:     "layout field multiple",
			input:    `layout = "wrapper;base;main"`,
			expected: TemplateNames{"wrapper", "base", "main"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config AppConfig
			if _, err := toml.Decode(tt.input, &config); err != nil {
				t.Fatalf("failed to decode TOML: %v", err)
			}

			// For specific tests, check different fields
			if tt.name == "both before and after" {
				if len(config.Before) != len(tt.expected) {
					t.Errorf("expected %d templates, got %d", len(tt.expected), len(config.Before))
				}
				for i := range config.Before {
					if config.Before[i] != tt.expected[i] {
						t.Errorf("expected template[%d] = %q, got %q", i, tt.expected[i], config.Before[i])
					}
				}
				// Also check after field
				expectedAfter := TemplateNames{"footer", "analytics"}
				if len(config.After) != len(expectedAfter) {
					t.Errorf("expected %d after templates, got %d", len(expectedAfter), len(config.After))
				}
				for i := range config.After {
					if config.After[i] != expectedAfter[i] {
						t.Errorf("expected after[%d] = %q, got %q", i, expectedAfter[i], config.After[i])
					}
				}
			} else if tt.name == "layout field single" || tt.name == "layout field multiple" {
				// Check the layout field for layout tests
				if len(config.Layout) != len(tt.expected) {
					t.Errorf("expected %d templates, got %d", len(tt.expected), len(config.Layout))
				}
				for i := range config.Layout {
					if config.Layout[i] != tt.expected[i] {
						t.Errorf("expected layout[%d] = %q, got %q", i, tt.expected[i], config.Layout[i])
					}
				}
			} else {
				// Check the before field for other tests
				if len(config.Before) != len(tt.expected) {
					t.Errorf("expected %d templates, got %d", len(tt.expected), len(config.Before))
				}
				for i := range config.Before {
					if config.Before[i] != tt.expected[i] {
						t.Errorf("expected template[%d] = %q, got %q", i, tt.expected[i], config.Before[i])
					}
				}
			}
		})
	}
}

func TestTemplateNamesString(t *testing.T) {
	tests := []struct {
		name     string
		tn       TemplateNames
		expected string
	}{
		{
			name:     "empty",
			tn:       nil,
			expected: "",
		},
		{
			name:     "single",
			tn:       TemplateNames{"foo"},
			expected: "foo",
		},
		{
			name:     "multiple",
			tn:       TemplateNames{"foo", "bar", "baz"},
			expected: "foo;bar;baz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tn.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
