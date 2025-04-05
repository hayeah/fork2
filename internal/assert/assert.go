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

// equalFixture is a helper function that compares content with a fixture file.
// If GEN_FIXTURE=true is set, it writes the content to the fixture file and passes the test.
// Otherwise, it compares the content with the fixture content.
func (a *Assert) equalFixture(fixturePath string, content string) {

	// Check if we should generate the fixture
	if os.Getenv("GEN_FIXTURE") == "true" {
		// Create directory if it doesn't exist
		err := os.MkdirAll(filepath.Dir(fixturePath), 0755)
		a.NoError(err, "Failed to create fixture directory")

		err = os.WriteFile(fixturePath, []byte(content), 0644)
		a.NoError(err, "Failed to write fixture file")
		return // Skip comparison when generating fixtures
	}

	// Read the fixture file
	expected, err := os.ReadFile(fixturePath)
	a.NoError(err, "Failed to read fixture file")

	// Compare the result with the fixture
	a.Equal(string(expected), content, "Result does not match fixture")
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

	// Create fixture path with .json extension
	fixtureFileName := fmt.Sprintf("%s_%s.json", a.T.Name(), fixtureName)
	fixturePath := filepath.Join("fixtures", fixtureFileName)

	// Compare with fixture
	a.equalFixture(fixturePath, resultStr)
}

// EqualToStringFixture compares a string result with the content of a fixture file.
// If GEN_FIXTURE=true is set, it writes the string result to the fixture file and passes the test.
// Otherwise, it compares the string result with the fixture content.
// The fixture path is derived from the test name: fixtures/<a.T.Name()>/<fixtureName>
// Note: The .txt extension is automatically added to the fixture name
func (a *Assert) EqualToStringFixture(fixtureName string, result string) {
	// Create fixture path with .txt extension
	fixtureFileName := fmt.Sprintf("%s_%s.txt", a.T.Name(), fixtureName)
	fixturePath := filepath.Join("fixtures", fixtureFileName)

	// Compare with fixture
	a.equalFixture(fixturePath, result)
}
