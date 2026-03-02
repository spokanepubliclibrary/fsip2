package fixtures

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// fixturesDir is the absolute path to the directory containing this file (tests/fixtures/).
// Using runtime.Caller ensures the path resolves correctly regardless of the test's
// working directory (e.g., when tests run from internal/handlers/).
var fixturesDir string

func init() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("fixtures: runtime.Caller(0) failed — cannot determine fixtures directory")
	}
	fixturesDir = filepath.Dir(file)
}

// LoadFixture reads a fixture file and returns its contents.
func LoadFixture(name string) ([]byte, error) {
	path := filepath.Join(fixturesDir, name)
	return os.ReadFile(path)
}

// LoadFixtureAs loads a fixture and unmarshals into the provided struct.
func LoadFixtureAs(name string, v interface{}) error {
	data, err := LoadFixture(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
