package metrics

import (
	"testing"
)

func TestOutputMetrics(t *testing.T) {
	// Create a simple counter for testing
	counter := &SimpleCounter{}

	// Create metrics with 2 workers
	metrics := NewOutputMetrics(counter, 2)

	// Add some test content
	testText := "This is a test.\nIt has two lines."
	metrics.Add("test", "item1", []byte(testText))
	metrics.Add("test", "item2", []byte("Another test item"))
	metrics.Add("other", "item3", []byte("Different type"))

	// Wait for processing to complete
	metrics.Wait()

	// Check that we have the expected number of items
	if len(metrics.Items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(metrics.Items))
	}

	// Check SumBy functionality
	testSum := metrics.SumBy("test")
	if testSum.Tokens <= 0 {
		t.Errorf("Expected positive token count for 'test' type, got %d", testSum.Tokens)
	}

	// Check that lines are counted correctly for the first item
	key := MetricKey{Type: "test", Key: "item1"}
	if item, ok := metrics.Items[key]; ok {
		if item.Lines != 2 {
			t.Errorf("Expected 2 lines for item1, got %d", item.Lines)
		}
	} else {
		t.Errorf("Item 'test:item1' not found in metrics")
	}
}

func TestMetricKey(t *testing.T) {
	// Test NewKey and String methods
	key := NewKey("file", "path/to/file.go")
	if key.Type != "file" || key.Key != "path/to/file.go" {
		t.Errorf("NewKey failed, got %v", key)
	}

	if key.String() != "file:path/to/file.go" {
		t.Errorf("String() failed, expected 'file:path/to/file.go', got '%s'", key.String())
	}
}

func TestSimpleCounter(t *testing.T) {
	counter := &SimpleCounter{}

	// Test with empty string
	bytes, tokens, lines := counter.Count("")
	if bytes != 0 || tokens != 0 || lines != 1 {
		t.Errorf("Empty string count wrong, got bytes=%d, tokens=%d, lines=%d", bytes, tokens, lines)
	}

	// Test with simple text
	text := "Hello, world!\nThis is a test."
	bytes, tokens, lines = counter.Count(text)

	if bytes != len(text) {
		t.Errorf("Byte count wrong, expected %d, got %d", len(text), bytes)
	}

	if lines != 2 {
		t.Errorf("Line count wrong, expected 2, got %d", lines)
	}

	// We can't test exact token count with SimpleCounter, but it should be positive
	if tokens <= 0 {
		t.Errorf("Token count should be positive, got %d", tokens)
	}
}
