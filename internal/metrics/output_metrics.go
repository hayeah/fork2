package metrics

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// MetricKey identifies a specific metric by type and key
type MetricKey struct {
	Type string // "file" | "template" | "user" | "final"
	Key  string
}

// String returns a string representation of the MetricKey
func (k MetricKey) String() string {
	return fmt.Sprintf("%s:%s", k.Type, k.Key)
}

// NewKey creates a new MetricKey with the given type and key
func NewKey(typ, key string) MetricKey {
	return MetricKey{Type: typ, Key: key}
}

// MetricItem stores the metrics for a specific item
type MetricItem struct {
	Bytes  int `json:"bytes"`
	Tokens int `json:"tokens"`
	Lines  int `json:"lines"`
}

// Add adds the given metrics to this item
func (m *MetricItem) Add(bytes, tokens, lines int) {
	m.Bytes += bytes
	m.Tokens += tokens
	m.Lines += lines
}

// job represents a pending metrics calculation job
type job struct {
	typ     string
	key     string
	content []byte
}

// OutputMetrics collects metrics for various components
type OutputMetrics struct {
	mu    sync.Mutex
	wg    sync.WaitGroup
	jobs  chan job
	Items map[MetricKey]MetricItem
	Ctr   Counter // token/line/byte counter
}

// NewOutputMetrics creates a new OutputMetrics with the given counter and worker count
func NewOutputMetrics(counter Counter, workers int) *OutputMetrics {
	if workers < 1 {
		workers = 1
	}

	m := &OutputMetrics{
		jobs:  make(chan job, workers*2), // Buffer the channel
		Items: make(map[MetricKey]MetricItem),
		Ctr:   counter,
	}

	// Start worker goroutines
	m.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go m.worker()
	}

	return m
}

// worker processes jobs from the jobs channel
func (m *OutputMetrics) worker() {
	defer m.wg.Done()

	for job := range m.jobs {
		// Process the job
		text := string(job.content)
		bytes, tokens, lines := m.Ctr.Count(text)

		// Update the metrics
		m.mu.Lock()
		key := MetricKey{Type: job.typ, Key: job.key}
		item := m.Items[key]
		item.Add(bytes, tokens, lines)
		m.Items[key] = item
		m.mu.Unlock()
	}
}

// Add adds content to be processed for metrics
func (m *OutputMetrics) Add(typ, key string, content []byte) {
	m.jobs <- job{typ: typ, key: key, content: content}
}

// Wait waits for all pending jobs to complete
func (m *OutputMetrics) Wait() {
	close(m.jobs)
	m.wg.Wait()
}

// sumByLocked returns the sum of all metrics for the given type.
// Caller **must** hold m.mu.
func (m *OutputMetrics) sumByLocked(typeName string) MetricItem {
	var sum MetricItem
	for k, v := range m.Items {
		if k.Type == typeName {
			sum.Add(v.Bytes, v.Tokens, v.Lines)
		}
	}
	return sum
}

// SumBy returns the sum of all metrics for the given type
func (m *OutputMetrics) SumBy(typeName string) MetricItem {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sumByLocked(typeName)
}

// MarshalJSON marshals the metrics to JSON with string keys
func (m *OutputMetrics) MarshalJSON() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a map with string keys
	result := make(map[string]MetricItem, len(m.Items))
	for k, v := range m.Items {
		result[k.String()] = v
	}

	return json.Marshal(result)
}

// HumanSummary returns a human-readable summary of the metrics
func HumanSummary(m *OutputMetrics) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var sb strings.Builder

	// Final output
	if final, ok := m.Items[MetricKey{Type: "final", Key: ""}]; ok {
		sb.WriteString(fmt.Sprintf(
			"Final output: %d tokens, %d bytes, %d lines\n",
			final.Tokens, final.Bytes, final.Lines))
	}

	// Files
	fileSum := m.sumByLocked("file")
	if fileSum.Tokens > 0 {
		sb.WriteString(fmt.Sprintf(
			"Files: %d tokens, %d bytes, %d lines\n",
			fileSum.Tokens, fileSum.Bytes, fileSum.Lines))

		fileCount := 0
		for k := range m.Items {
			if k.Type == "file" {
				fileCount++
			}
		}
		sb.WriteString(fmt.Sprintf("  (%d files processed)\n", fileCount))
	}

	// Templates
	templateSum := m.sumByLocked("template")
	if templateSum.Tokens > 0 {
		sb.WriteString(fmt.Sprintf(
			"Templates: %d tokens, %d bytes, %d lines\n",
			templateSum.Tokens, templateSum.Bytes, templateSum.Lines))
	}

	// User input
	userSum := m.sumByLocked("user")
	if userSum.Tokens > 0 {
		sb.WriteString(fmt.Sprintf(
			"User input: %d tokens, %d bytes, %d lines\n",
			userSum.Tokens, userSum.Bytes, userSum.Lines))
	}

	return sb.String()
}
