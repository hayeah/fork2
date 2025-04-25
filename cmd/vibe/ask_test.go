package main

import (
	"os"
	"testing"

	"github.com/hayeah/fork2/internal/assert"
)

func TestAskRunnerData(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "vibe-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test cases for data parameter parsing
	tests := []struct {
		name     string
		data     []string
		expected map[string]string
	}{
		{
			name: "Single key-value pair",
			data: []string{"model=gpt4"},
			expected: map[string]string{
				"model": "gpt4",
			},
		},
		{
			name: "Multiple key-value pairs",
			data: []string{"model=gpt4", "diff=v4"},
			expected: map[string]string{
				"model": "gpt4",
				"diff":  "v4",
			},
		},
		{
			name: "URL-style query parameters",
			data: []string{"model=gpt4&diff=v4"},
			expected: map[string]string{
				"model": "gpt4",
				"diff":  "v4",
			},
		},
		{
			name: "Mixed styles",
			data: []string{"model=gpt4", "diff=v4&format=json"},
			expected: map[string]string{
				"model":  "gpt4",
				"diff":   "v4",
				"format": "json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)

			// Create AskCmd with test data
			cmdArgs := AskCmd{
				TokenEstimator: "simple",
				Data:           tt.data,
			}

			// Create AskRunner
			r, err := NewAskRunner(cmdArgs, tempDir)
			a.NoError(err)

			// Check that data was parsed correctly
			a.Equal(len(tt.expected), len(r.Data))
			for k, v := range tt.expected {
				a.Equal(v, r.Data[k])
			}

			// Create VibeContext
			ctx, err := NewVibeContext(r)
			a.NoError(err)

			// Check that data was copied to VibeContext
			a.Equal(len(tt.expected), len(ctx.Data))
			for k, v := range tt.expected {
				a.Equal(v, ctx.Data[k])
			}

			// Create memoized context
			memoCtx := newVibeContextMemoized(ctx)

			// Check that data was copied to memoized context
			a.Equal(len(tt.expected), len(memoCtx.Data))
			for k, v := range tt.expected {
				a.Equal(v, memoCtx.Data[k])
			}
		})
	}
}
