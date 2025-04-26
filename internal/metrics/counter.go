package metrics

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

// Counter provides methods for counting bytes, tokens, and lines in text
type Counter interface {
	// Count returns the number of bytes, tokens, and lines in the given text
	Count(text string) (bytes, tokens, lines int)
}

// SimpleCounter provides a simple token counting mechanism
// that estimates tokens as bytes/4
type SimpleCounter struct{}

// Count returns bytes, estimated tokens, and lines for the given text
func (c *SimpleCounter) Count(text string) (int, int, int) {
	byteCount := len(text)
	lines := bytes.Count([]byte(text), []byte{'\n'}) + 1
	tokens := estimateTokenCountSimple(text)
	return byteCount, tokens, lines
}

// TiktokenCounter uses the tiktoken library to count tokens
type TiktokenCounter struct {
	model string
}

// NewTiktokenCounter creates a new TiktokenCounter for the given model
func NewTiktokenCounter(model string) (*TiktokenCounter, error) {
	// Validate that the model is supported
	if _, err := tiktoken.EncodingForModel(model); err != nil {
		return nil, fmt.Errorf("unsupported model for tiktoken: %s", model)
	}
	return &TiktokenCounter{model: model}, nil
}

// Count returns bytes, tokens (using tiktoken), and lines for the given text
func (c *TiktokenCounter) Count(text string) (int, int, int) {
	byteCount := len(text)
	lines := bytes.Count([]byte(text), []byte{'\n'}) + 1
	tokens := estimateTokenCountTiktoken(text, c.model)
	return byteCount, tokens, lines
}

// estimateTokenCountSimple provides a simple approximation of token count
// by dividing the byte count by 4 (average English token is ~4 bytes)
func estimateTokenCountSimple(text string) int {
	// Simple approximation: ~4 bytes per token for English text
	return len(text) / 4
}

// estimateTokenCountTiktoken uses the tiktoken library to count tokens
// more accurately based on the tokenization of a specific model
func estimateTokenCountTiktoken(text string, model string) int {
	encoding, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// Fall back to simple estimation if model not supported
		return estimateTokenCountSimple(text)
	}

	// Clean the text (optional)
	text = strings.TrimSpace(text)

	// Count tokens
	tokens := encoding.Encode(text, nil, nil)
	return len(tokens)
}
