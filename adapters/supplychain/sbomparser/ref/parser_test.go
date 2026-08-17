package ref

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/tck"
)

func TestSBOMParserTCK(t *testing.T) {
	// Resolve fixture directory relative to this test file's source location.
	// Test file is at adapters/supplychain/sbomparser/ref/parser_test.go.
	// Repo root is 4 levels up: ref → sbomparser → supplychain → adapters → repo.
	_, srcFile, _, _ := runtime.Caller(0)
	srcDir := filepath.Dir(srcFile)                           // adapters/supplychain/sbomparser/ref
	repoRoot := filepath.Join(srcDir, "..", "..", "..", "..") // repo root
	baseDir := filepath.Join(repoRoot, "testdata", "supplychain")
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		t.Skipf("skipping (fixture dir not found at %s): %v", baseDir, err)
	}

	loader := func(name string) []byte {
		path := filepath.Join(baseDir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to load fixture %q: %v", name, err)
		}
		return b
	}

	tck.RunSBOMParserTCK(t, func() ports.SBOMParser { return NewParser() }, loader)
}
