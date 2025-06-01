package metrics

import (
	"encoding/json"
	"fmt"
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

// job represents a pending metrics calculation job
type job struct {
	typ     string
	key     string
	content []byte
}

// OutputMetrics collects metrics for various components
type OutputMetrics struct {
	mu        sync.Mutex
	wg        sync.WaitGroup
	jobs      chan job
	closeOnce sync.Once
	Items     map[MetricKey]MetricItem
	Ctr       Counter // token/line/byte counter
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
		// Check if this key already exists, if so, ignore it
		if _, exists := m.Items[key]; !exists {
			// Only add metrics for new keys
			m.Items[key] = MetricItem{
				Bytes:  bytes,
				Tokens: tokens,
				Lines:  lines,
			}
		}
		m.mu.Unlock()
	}
}

// Add adds content to be processed for metrics
func (m *OutputMetrics) Add(typ, key string, content []byte) {
	m.jobs <- job{typ: typ, key: key, content: content}
}

// AddBytesCountAsEstimate adds metrics using byte count with estimated token count
// It estimates tokens by dividing bytes by 4 (approximate average bytes per token)
// This avoids the need to process the full content for token counting
func (m *OutputMetrics) AddBytesCountAsEstimate(typ, key string, byteCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metricKey := MetricKey{Type: typ, Key: key}
	// Check if this key already exists, if so, ignore it
	if _, exists := m.Items[metricKey]; !exists {
		// Estimate tokens as bytes/4 (rough approximation)
		// Estimate lines as bytes/50 (rough approximation of average line length)
		m.Items[metricKey] = MetricItem{
			Bytes:  byteCount,
			Tokens: byteCount / 4,
			Lines:  byteCount / 50,
		}
	}
}

// Wait waits for all pending jobs to complete
// It is idempotent and can be called multiple times safely
func (m *OutputMetrics) Wait() {
	// Close the jobs channel exactly once, if it exists
	m.closeOnce.Do(func() {
		if m.jobs != nil {
			close(m.jobs)
		}
	})

	// Wait for all workers to finish processing
	m.wg.Wait()
}

// sumByLocked returns the sum of all metrics for the given type.
// Caller **must** hold m.mu.
func (m *OutputMetrics) sumByLocked(typeName string) MetricItem {
	var sum MetricItem
	for k, v := range m.Items {
		if k.Type == typeName {
			sum.Bytes += v.Bytes
			sum.Tokens += v.Tokens
			sum.Lines += v.Lines
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
