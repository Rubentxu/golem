package tck

import (
	"context"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// RunSBOMParserTCK exercises an SBOMParser implementation against the SP-010
// fixture corpus. It asserts format detection, spec version extraction,
// artifact digest extraction, component counts, purl normalization, and
// synthetic fallback detection.
func RunSBOMParserTCK(t *testing.T, newParser func() ports.SBOMParser, fixtureLoader func(name string) []byte) {
	p := newParser()

	t.Run("spdx23 fixture parses correctly", func(t *testing.T) {
		raw := fixtureLoader("spdx23.json")
		result, err := p.Parse(context.Background(), raw)
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		if result.Format != "spdx" {
			t.Errorf("Format = %q, want spdx", result.Format)
		}
		if result.SpecVersion != "SPDX-2.3" {
			t.Errorf("SpecVersion = %q, want SPDX-2.3", result.SpecVersion)
		}
		// Artifact digest from verificationCode.
		if result.ArtifactDigest == "" {
			t.Error("ArtifactDigest is empty, expected sha256:... from verificationCode")
		}
		if len(result.Components) < 3 {
			t.Errorf("Components count = %d, want at least 3", len(result.Components))
		}
		// All components should have a purl.
		for i, c := range result.Components {
			if c.Purl == "" {
				t.Errorf("Component[%d].Purl is empty", i)
			}
		}
	})

	t.Run("spdx30 fixture parses correctly", func(t *testing.T) {
		raw := fixtureLoader("spdx30.json")
		result, err := p.Parse(context.Background(), raw)
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		if result.Format != "spdx" {
			t.Errorf("Format = %q, want spdx", result.Format)
		}
		if result.SpecVersion == "" {
			t.Error("SpecVersion is empty")
		}
		// Components should be extracted from graph.
		if len(result.Components) < 2 {
			t.Errorf("Components count = %d, want at least 2", len(result.Components))
		}
	})

	t.Run("cyclonedx15 fixture parses correctly", func(t *testing.T) {
		raw := fixtureLoader("cdx15.json")
		result, err := p.Parse(context.Background(), raw)
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		if result.Format != "cyclonedx" {
			t.Errorf("Format = %q, want cyclonedx", result.Format)
		}
		if result.SpecVersion != "1.5" {
			t.Errorf("SpecVersion = %q, want 1.5", result.SpecVersion)
		}
		// Artifact digest from metadata.component sha256.
		if result.ArtifactDigest == "" {
			t.Error("ArtifactDigest is empty, expected sha256 from metadata.component")
		}
		if len(result.Components) < 2 {
			t.Errorf("Components count = %d, want at least 2", len(result.Components))
		}
		// Components with purls.
		hasPurl := false
		for _, c := range result.Components {
			if c.Purl != "" && !c.Synthetic {
				hasPurl = true
			}
		}
		if !hasPurl {
			t.Error("No non-synthetic purl found in components")
		}
	})

	t.Run("cyclonedx16 fixture: component without purl gets synthetic", func(t *testing.T) {
		raw := fixtureLoader("cdx16.json")
		result, err := p.Parse(context.Background(), raw)
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		if result.SpecVersion != "1.6" {
			t.Errorf("SpecVersion = %q, want 1.6", result.SpecVersion)
		}
		// Should have at least one synthetic component.
		syntheticCount := 0
		for _, c := range result.Components {
			if c.Synthetic {
				syntheticCount++
			}
		}
		if syntheticCount == 0 {
			t.Error("Expected at least one Synthetic=true component (the one without purl)")
		}
		// Synthetic purl should be pkg:generic/<name>@<version>
		for _, c := range result.Components {
			if c.Synthetic {
				if c.Purl == "" {
					t.Error("Synthetic component has empty Purl")
				}
			}
		}
	})

	t.Run("intoto attestation is not an SBOM", func(t *testing.T) {
		raw := fixtureLoader("intoto-attestation.json")
		_, err := p.Parse(context.Background(), raw)
		if err == nil {
			t.Error("Parse of intoto-attestation should fail (not an SBOM)")
		}
	})

	t.Run("openvex is not an SBOM", func(t *testing.T) {
		raw := fixtureLoader("openvex.json")
		_, err := p.Parse(context.Background(), raw)
		if err == nil {
			t.Error("Parse of openvex should fail (not an SBOM)")
		}
	})

	t.Run("purl normalization is applied to all components", func(t *testing.T) {
		raw := fixtureLoader("spdx23.json")
		result, err := p.Parse(context.Background(), raw)
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		for i, c := range result.Components {
			// Normalized purls should have "pkg:" prefix and lowercase scheme.
			if c.Purl != "" && c.Purl[:4] != "pkg:" {
				t.Errorf("Component[%d].Purl = %q does not start with 'pkg:'", i, c.Purl)
			}
		}
	})

	t.Run("empty document returns error", func(t *testing.T) {
		_, err := p.Parse(context.Background(), []byte{})
		if err == nil {
			t.Error("Expected error for empty document")
		}
	})
}
