package profile

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoDomainLeak verifies that internal/domain and internal/application
// packages do not import internal/profile.
// This is an architectural fitness function complementing archtest.
//
// ADR-057 §3: "internal/domain/ and internal/application/ never see Profile{}"
func TestNoDomainLeak(t *testing.T) {
	root := filepath.Join("..", "..")

	var violations []string

	// Walk internal/domain and internal/application.
	dirs := []string{
		filepath.Join(root, "internal", "domain"),
		filepath.Join(root, "internal", "application"),
	}

	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			imports, err := fileImports(path)
			if err != nil {
				return nil // skip files we can't parse
			}
			for _, imp := range imports {
				if imp == "github.com/Rubentxu/golem/internal/profile" ||
					strings.HasPrefix(imp, "github.com/Rubentxu/golem/internal/profile/") {
					rel, _ := filepath.Rel(root, path)
					violations = append(violations, filepath.ToSlash(rel)+" imports "+imp)
				}
			}
			return nil
		})
		if err != nil {
			t.Logf("walk error: %v", err)
		}
	}

	for _, v := range violations {
		t.Errorf("arch violation: %s", v)
	}
}

func fileImports(path string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(f.Imports))
	for _, imp := range f.Imports {
		v := strings.Trim(imp.Path.Value, `"`)
		out = append(out, v)
	}
	return out, nil
}
