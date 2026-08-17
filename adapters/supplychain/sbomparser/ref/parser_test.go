package ref

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/tck"
)

func TestSBOMParserTCK(t *testing.T) {
	// Resolve fixture directory relative to this test file: ../../../testdata/supplychain/
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("skipping (cannot determine testdata path): %v", err)
	}
	baseDir := filepath.Join(filepath.Dir(exe), "..", "..", "..", "testdata", "supplychain")
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
