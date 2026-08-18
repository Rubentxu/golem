package packs

import (
	"encoding/json"
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
)

// -- Manifest.Validate tests --

func TestManifestValidate_Valid(t *testing.T) {
	catalog := NewPermissionCatalogV1()
	m := &Manifest{
		FormatVersion:   "1",
		Name:            "test-pack",
		Version:         "1.0.0",
		GolemAPI:        ">=0.5 <0.6",
		IntegrityDigest: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		Permissions:     []string{"proposal.write", "ci.trigger"},
		Migrations:      nil,
		UI:              nil,
	}
	if err := m.Validate(catalog); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestManifestValidate_FormatVersionWrong(t *testing.T) {
	catalog := NewPermissionCatalogV1()
	m := &Manifest{
		FormatVersion:   "2",
		Name:            "test-pack",
		Version:         "1.0.0",
		GolemAPI:        ">=0.5 <0.6",
		IntegrityDigest: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		Permissions:     []string{"proposal.write"},
	}
	err := m.Validate(catalog)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isErr(err, ports.ErrPackManifestInvalid) {
		t.Errorf("expected ports.ErrPackManifestInvalid, got %v", err)
	}
}

func TestManifestValidate_UnknownPermission(t *testing.T) {
	catalog := NewPermissionCatalogV1()
	m := &Manifest{
		FormatVersion:   "1",
		Name:            "test-pack",
		Version:         "1.0.0",
		GolemAPI:        ">=0.5 <0.6",
		IntegrityDigest: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		Permissions:     []string{"proposal.write", "unknown:permission"},
	}
	err := m.Validate(catalog)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isErr(err, ports.ErrPackUnknownPermission) {
		t.Errorf("expected ports.ErrPackUnknownPermission, got %v", err)
	}
}

func TestManifestValidate_IntegrityDigestMalformed(t *testing.T) {
	catalog := NewPermissionCatalogV1()
	m := &Manifest{
		FormatVersion:   "1",
		Name:            "test-pack",
		Version:         "1.0.0",
		GolemAPI:        ">=0.5 <0.6",
		IntegrityDigest: "not-a-valid-hex",
		Permissions:     []string{},
	}
	err := m.Validate(catalog)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isErr(err, ports.ErrPackManifestInvalid) {
		t.Errorf("expected ports.ErrPackManifestInvalid, got %v", err)
	}
}

func TestManifestValidate_IntegrityDigestShort(t *testing.T) {
	catalog := NewPermissionCatalogV1()
	m := &Manifest{
		FormatVersion:   "1",
		Name:            "test-pack",
		Version:         "1.0.0",
		GolemAPI:        ">=0.5 <0.6",
		IntegrityDigest: "a1b2c3d4", // only 8 chars, not 64
		Permissions:     []string{},
	}
	err := m.Validate(catalog)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isErr(err, ports.ErrPackManifestInvalid) {
		t.Errorf("expected ports.ErrPackManifestInvalid, got %v", err)
	}
}

func TestManifestValidate_GolemAPINoIntersection(t *testing.T) {
	catalog := NewPermissionCatalogV1()
	m := &Manifest{
		FormatVersion:   "1",
		Name:            "test-pack",
		Version:         "1.0.0",
		GolemAPI:        ">=0.1 <0.3", // does NOT intersect [0.5, 0.6)
		IntegrityDigest: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		Permissions:     []string{},
	}
	err := m.Validate(catalog)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !isErr(err, ports.ErrPackManifestInvalid) {
		t.Errorf("expected ports.ErrPackManifestInvalid, got %v", err)
	}
}

func TestManifestValidate_GolemAPIIntersects(t *testing.T) {
	catalog := NewPermissionCatalogV1()
	cases := []string{
		">=0.5 <0.6", // exact
		">=0.4 <0.7", // wider
		">=0.1 <1.0", // covers everything
		">0.4 <0.7",  // open lower
		"<0.7",       // no lower bound
		">=0.5",      // no upper bound
	}
	for _, api := range cases {
		m := &Manifest{
			FormatVersion:   "1",
			Name:            "test-pack",
			Version:         "1.0.0",
			GolemAPI:        api,
			IntegrityDigest: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			Permissions:     []string{},
		}
		if err := m.Validate(catalog); err != nil {
			t.Errorf("golem_api=%q: expected nil, got %v", api, err)
		}
	}
}

func TestManifestValidate_GolemAPIDoesNotIntersect(t *testing.T) {
	catalog := NewPermissionCatalogV1()
	cases := []string{
		">=0.1 <0.5", // upper bound is 0.5, exclusive — no overlap with [0.5,0.6)
		"<0.5",       // upper < 0.5
		">=0.6 <0.7", // lower bound 0.6, which is outside [0.5, 0.6)
		">=0.6",      // no upper bound but lower >= 0.6
	}
	for _, api := range cases {
		m := &Manifest{
			FormatVersion:   "1",
			Name:            "test-pack",
			Version:         "1.0.0",
			GolemAPI:        api,
			IntegrityDigest: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			Permissions:     []string{},
		}
		err := m.Validate(catalog)
		if err == nil {
			t.Errorf("golem_api=%q: expected error, got nil", api)
		}
	}
}

func TestManifestValidate_MigrationsStructuralPass(t *testing.T) {
	// Migrations[] is validated structurally (schema correct) — Validate passes.
	// Activation later rejects len>0 via ErrUnsupportedInM51.
	catalog := NewPermissionCatalogV1()
	m := &Manifest{
		FormatVersion:   "1",
		Name:            "test-pack",
		Version:         "1.0.0",
		GolemAPI:        ">=0.5 <0.6",
		IntegrityDigest: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		Permissions:     []string{},
		Migrations: []Migration{
			{ID: "001", Description: "add foo table", Script: "CREATE TABLE foo()"},
		},
	}
	if err := m.Validate(catalog); err != nil {
		t.Errorf("expected nil (structural pass), got %v", err)
	}
}

func TestManifestValidate_UIStructuralPass(t *testing.T) {
	// UI[] is validated structurally — Validate passes.
	// Activation later rejects len>0 via ErrUnsupportedInM51.
	catalog := NewPermissionCatalogV1()
	m := &Manifest{
		FormatVersion:   "1",
		Name:            "test-pack",
		Version:         "1.0.0",
		GolemAPI:        ">=0.5 <0.6",
		IntegrityDigest: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		Permissions:     []string{},
		UI: []UIContribution{
			{Type: "dashboard", Path: "ui/main.json", Label: "Main"},
		},
	}
	if err := m.Validate(catalog); err != nil {
		t.Errorf("expected nil (structural pass), got %v", err)
	}
}

// -- DigestMatchesCanonical tests --

func TestDigestMatchesCanonical_Valid(t *testing.T) {
	// Create a manifest, serialise it canonically, compute digest, assign.
	m := &Manifest{
		FormatVersion:   "1",
		Name:            "test-pack",
		Version:         "1.0.0",
		GolemAPI:        ">=0.5 <0.6",
		IntegrityDigest: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		Permissions:     []string{},
	}
	// A mismatched digest should return false
	if m.DigestMatchesCanonical([]byte(`{"name":"other"}`)) {
		t.Error("expected false for mismatched bytes")
	}
}

func TestDigestMatchesCanonical_ReorderedKeys(t *testing.T) {
	// Two JSON objects with the same logical content but keys in different
	// order must produce the same canonical digest after re-ordering.
	bytesA := []byte(`{"name":"test","version":"1.0.0","format_version":"1"}`)
	bytesB := []byte(`{"format_version":"1","version":"1.0.0","name":"test"}`)

	var a, b Manifest
	if err := json.Unmarshal(bytesA, &a); err != nil {
		t.Fatalf("unmarshal a: %v", err)
	}
	if err := json.Unmarshal(bytesB, &b); err != nil {
		t.Fatalf("unmarshal b: %v", err)
	}

	// Both should canonicalise to the same JSON string.
	canonA, err := MarshalCanonical(&a)
	if err != nil {
		t.Fatalf("MarshalCanonical a: %v", err)
	}
	canonB, err := MarshalCanonical(&b)
	if err != nil {
		t.Fatalf("MarshalCanonical b: %v", err)
	}

	// MarshalCanonical must produce byte-identical output for reordered keys.
	if string(canonA) != string(canonB) {
		t.Errorf("canonical JSON differs for reordered keys:\nA: %s\nB: %s", canonA, canonB)
	}

	// Both must produce the same SHA-256 digest (computed, not declared).
	digestA, err := a.ComputeDigest()
	if err != nil {
		t.Fatalf("ComputeDigest a: %v", err)
	}
	digestB, err := b.ComputeDigest()
	if err != nil {
		t.Fatalf("ComputeDigest b: %v", err)
	}
	if digestA != digestB {
		t.Errorf("computed digests differ for equivalent manifests: %q vs %q", digestA, digestB)
	}
	if len(digestA) != 64 {
		t.Errorf("digest is not sha256 hex 64: %q", digestA)
	}
}

// -- Permissions catalog tests --

func TestPermissionCatalog_Allowed(t *testing.T) {
	catalog := NewPermissionCatalogV1()
	// Verbatim from PLUGIN_RUNTIME.md §Permissions.
	valid := []string{
		"graph.read:lens",
		"proposal.write",
		"object.read",
		"scm.read",
		"ci.trigger",
	}
	for _, p := range valid {
		if !catalog.Allowed(p) {
			t.Errorf("expected %q to be allowed", p)
		}
	}
}

func TestPermissionCatalog_NotAllowed(t *testing.T) {
	catalog := NewPermissionCatalogV1()
	// Names from the stale aspirational example in
	// golem-documentation/examples/capability-pack/manifest.json (it mixes
	// capabilities and permissions) must be rejected.
	invalid := []string{
		"graph.lens:*",
		"graph.lens:release-security",
		"proposal:create",
		"artifact.read",
		"graph.read",
		"unknown:perm",
		"",
	}
	for _, p := range invalid {
		if catalog.Allowed(p) {
			t.Errorf("expected %q to NOT be allowed", p)
		}
	}
}

// -- golemAPIIntersectsSupported tests --

func TestGolemAPIIntersectsSupported(t *testing.T) {
	cases := []struct {
		rangeStr   string
		intersects bool
	}{
		{">=0.5 <0.6", true},
		{">=0.4 <0.7", true},
		{">=0.1 <1.0", true},
		{">0.4 <0.7", true},
		{"<0.7", true},
		{">=0.5", true},
		{"0.5", true},
		{">=0.1 <0.5", false},
		{"<0.5", false},
		{">=0.6 <0.7", false},
		{">=0.6", false},
	}
	for _, c := range cases {
		got := golemAPIIntersectsSupported(c.rangeStr)
		if got != c.intersects {
			t.Errorf("golemAPIIntersectsSupported(%q) = %v, want %v", c.rangeStr, got, c.intersects)
		}
	}
}

// isErr reports whether err wraps target.
func isErr(err, target error) bool {
	if err == nil {
		return false
	}
	for {
		if err == target {
			return true
		}
		unwrap, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrap.Unwrap()
		if err == nil {
			return false
		}
	}
}
