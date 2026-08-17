// Package archtest contains GOLEM's architecture fitness functions
// (see golem-documentation/11_RESOURCES/TOOLING.md, "Architecture fitness").
//
// These tests encode the non-negotiable boundary rules as executable checks
// that run in CI alongside unit tests:
//
//   - ADR-002 / ADR-045: internal code (domain, application, ports and the
//     bounded-context packages) never imports third-party SDKs. Vendor
//     dependencies live only inside adapters/.
//   - ADR-047: vendor data types never cross adapter boundaries; internal/
//     must not import adapters/.
//
// If a check blocks a legitimate need, change the rule here consciously —
// with an ADR — instead of deleting the test.
package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const moduleName = "github.com/Rubentxu/golem"

// vendorDenyList documents known provider SDKs that must never leak into
// internal/. The blanket third-party ban below already covers them; this
// list exists to make intent explicit and failures self-explanatory.
var vendorDenyList = []string{
	"github.com/nats-io",           // EventTransport (ADR-012)
	"github.com/apache/hugegraph",  // GraphStore candidate (ADR-013)
	"github.com/vesoft-inc",        // NebulaGraph
	"github.com/open-policy-agent", // PolicyEvaluator (ADR-018)
	"github.com/aws",               // ObjectStore (ADR-014)
	"github.com/sigstore",          // Signing (ADR-025)
}

type violation struct {
	file string
	imp  string
	rule string
}

func TestForbiddenImports(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	var violations []violation

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			// Skip non-Go trees: documentation corpus, web UI, tooling
			// config and git internals. adapters/ is walked because its
			// imports are unrestricted, but it holds no rule subjects.
			switch name {
			case ".git", ".atl", ".github", "golem-documentation", "web",
				"node_modules", "testdata", "docs", "deployments":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		imports, err := fileImports(path)
		if err != nil {
			t.Errorf("parse %s: %v", rel, err)
			return nil
		}

		internal := strings.HasPrefix(rel, "internal/")

		for _, imp := range imports {
			switch {
			case isStdlib(imp), isModule(imp):
				// Allowed everywhere.
			case strings.HasPrefix(imp, "golang.org/x/"):
				// golang.org/x packages are stdlib-adjacent; currently
				// unused. If one becomes necessary, move it above this
				// case explicitly.
				violations = append(violations, violation{rel, imp, "golang.org/x not yet allowed"})
			default:
				if internal {
					violations = append(violations, violation{rel, imp, "third-party import inside internal/ (ADR-045: vendor SDKs only in adapters/)"})
				}
			}
			if internal && strings.HasPrefix(imp, moduleName+"/adapters/") {
				violations = append(violations, violation{rel, imp, "internal/ imports adapters/ (ADR-047: vendor types never cross adapter boundaries)"})
			}
			// Known vendor SDKs are allowed inside adapters/ only
			// (ADR-045); everywhere else — internal/, cmd/, tck/ — they
			// are denied with an explicit message. The blanket rule above
			// already blocks all third-party imports in internal/.
			if !strings.HasPrefix(rel, "adapters/") {
				for _, deny := range vendorDenyList {
					if strings.HasPrefix(imp, deny) {
						violations = append(violations, violation{rel, imp, "vendor SDK outside adapters/ (ADR-045: every external dependency sits behind a port)"})
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, v := range violations {
		t.Errorf("arch rule violation: %s imports %q — %s", v.file, v.imp, v.rule)
	}
	if len(violations) > 0 {
		t.Log("Architecture rules live in internal/archtest. Changing them requires an ADR (see 10_GOVERNANCE/CONTRIBUTING.md).")
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

func isStdlib(imp string) bool {
	// Standard library and the bare module name have no dot in the first
	// path element (vendormodule heuristic used by the Go toolchain).
	first := imp
	if i := strings.IndexByte(imp, '/'); i >= 0 {
		first = imp[:i]
	}
	return !strings.Contains(first, ".")
}

func isModule(imp string) bool {
	return imp == moduleName || strings.HasPrefix(imp, moduleName+"/")
}
