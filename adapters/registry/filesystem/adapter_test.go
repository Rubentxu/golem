package filesystem

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	journalmem "github.com/Rubentxu/golem/adapters/journal/memstore"
	"github.com/Rubentxu/golem/internal/clock"
	"github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/packs"
	"github.com/Rubentxu/golem/internal/ports"
)

// writePack creates a valid pack directory with a digest-correct manifest
// and returns its source path relative to root.
func writePack(t *testing.T, root, name string, manifestOverride func(m map[string]any), rawOverride []byte) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Build manifest via the domain so digests are correct.
	m := packs.Manifest{
		FormatVersion: "1",
		Name:          name,
		Version:       "1.0.0",
		GolemAPI:      ">=0.5 <0.6",
		Permissions:   []string{"scm.read"},
	}
	digest, err := m.ComputeDigest()
	if err != nil {
		t.Fatal(err)
	}
	m.IntegrityDigest = digest
	raw, err := json.Marshal(&m)
	if err != nil {
		t.Fatal(err)
	}
	if rawOverride != nil {
		raw = rawOverride
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

func newRegistry(t *testing.T, root string) *Registry {
	ts := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	return New(root, journalmem.NewJournal(), ids.NewGenerator(clock.Fixed(ts)), clock.Fixed(ts))
}

func TestLoadAndVerify_HappyPath(t *testing.T) {
	root := t.TempDir()
	src := writePack(t, root, "demo-pack", nil, nil)
	reg := newRegistry(t, root)

	p, err := reg.Load(context.Background(), src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Name != "demo-pack" || p.IntegrityDigest == "" || len(p.RawManifest) == 0 {
		t.Errorf("LoadedPack = %+v", p)
	}
	if err := reg.Verify(context.Background(), src); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestLoad_NotFound(t *testing.T) {
	reg := newRegistry(t, t.TempDir())
	_, err := reg.Load(context.Background(), "nope")
	if !errors.Is(err, ports.ErrPackNotFound) {
		t.Fatalf("err = %v, want ErrPackNotFound", err)
	}
}

func TestLoad_ManifestInvalid(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "broken")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"format_version":"99"}`), 0o644)

	reg := newRegistry(t, root)
	_, err := reg.Load(context.Background(), "broken")
	if !errors.Is(err, ports.ErrPackManifestInvalid) {
		t.Fatalf("err = %v, want ErrPackManifestInvalid", err)
	}
}

func TestVerify_DigestMismatch(t *testing.T) {
	root := t.TempDir()
	// Valid structure but a tampered digest value.
	tampered := []byte(`{"format_version":"1","name":"demo-pack","version":"1.0.0",` +
		`"golem_api":">=0.5 <0.6","permissions":["scm.read"],` +
		`"integrity_digest":"` + strings.Repeat("c", 64) + `"}`)
	writePack(t, root, "tampered", nil, tampered)

	reg := newRegistry(t, root)
	err := reg.Verify(context.Background(), "tampered")
	if !errors.Is(err, ports.ErrPackIntegrityFailed) {
		t.Fatalf("err = %v, want ErrPackIntegrityFailed", err)
	}
}

func TestActivate_E2E_WithJournal(t *testing.T) {
	root := t.TempDir()
	src := writePack(t, root, "demo-pack", nil, nil)

	j := journalmem.NewJournal()
	ts := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	reg := New(root, j, ids.NewGenerator(clock.Fixed(ts)), clock.Fixed(ts))

	p, err := reg.Load(context.Background(), src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	pos, err := reg.Activate(context.Background(), "tenant-a", p)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if pos == 0 {
		t.Errorf("position = 0, want a real journal position")
	}

	// Re-activation rejected.
	if _, err := reg.Activate(context.Background(), "tenant-a", p); !errors.Is(err, ports.ErrPackAlreadyActivated) {
		t.Fatalf("re-Activate err = %v, want ErrPackAlreadyActivated", err)
	}

	// Status reflects the activation.
	st, err := reg.Status(context.Background(), "tenant-a", "demo-pack")
	if err != nil || st == nil {
		t.Fatalf("Status = (%v, %v)", st, err)
	}
	if st.Name != "demo-pack" || st.ActivatedEventID == "" {
		t.Errorf("PackStatus = %+v", st)
	}

	// Deactivate is a no-op in M5.1.
	if err := reg.Deactivate(context.Background(), "tenant-a", "demo-pack"); err != nil {
		t.Errorf("Deactivate: %v", err)
	}
}

func TestDefaultRoot(t *testing.T) {
	reg := New("", nil, nil, nil)
	if reg.root != DefaultRoot {
		t.Errorf("root = %q, want %q", reg.root, DefaultRoot)
	}
}
