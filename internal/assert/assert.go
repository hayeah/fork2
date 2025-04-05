package assert

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Assert is a wrapper around assert.Assertions and testing.T
type Assert struct {
	*assert.Assertions
	T *testing.T
}

// New creates a new Assert object
func New(t *testing.T) *Assert {
	return &Assert{
		Assertions: assert.New(t),
		T:          t,
	}
}

// EqualToJSONFixture marshals the result to JSON and compares it with the content of a fixture file.
// If GEN_FIXTURE=true is set, it writes the marshaled result to the fixture file and passes the test.
// Otherwise, it compares the marshaled result with the fixture content.
// The fixture path is derived from the test name: fixtures/<a.T.Name()>/<fixtureName>
// Note: The .json extension is automatically added to the fixture name
func (a *Assert) EqualToJSONFixture(fixtureName string, result any) {
	// Marshal the result to JSON
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	a.NoError(err, "Failed to marshal result to JSON")

	// Add newline at the end for consistency
	resultStr := string(resultJSON)

	fixtureFileName := fmt.Sprintf("%s_%s.json", a.T.Name(), fixtureName)

	// Create fixture path with .json extension
	fixturePath := filepath.Join("fixtures", fixtureFileName)

	// Create directory if it doesn't exist
	err = os.MkdirAll(filepath.Dir(fixturePath), 0755)
	a.NoError(err, "Failed to create fixture directory")

	// Check if we should generate the fixture
	if os.Getenv("GEN_FIXTURE") == "true" {
		err := os.WriteFile(fixturePath, []byte(resultStr), 0644)
		a.NoError(err, "Failed to write fixture file")
		return // Skip comparison when generating fixtures
	}

	// Read the fixture file
	expected, err := os.ReadFile(fixturePath)
	a.NoError(err, "Failed to read fixture file")

	// Compare the result with the fixture
	a.Equal(string(expected), resultStr, "Result does not match fixture")
}
