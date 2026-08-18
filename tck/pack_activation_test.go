package tck

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
	fsregistry "github.com/Rubentxu/golem/adapters/registry/filesystem"
	"github.com/Rubentxu/golem/internal/clock"
	idgen "github.com/Rubentxu/golem/internal/ids"
	"github.com/Rubentxu/golem/internal/ports"
)

// examplesRoot is the repo examples/ directory as seen from tck/ (go test
// runs with the package directory as cwd).
const examplesRoot = "../examples"

// demoFixtureSource is the committed demo pack (stable, never regenerated —
// Q8/T4.1). Its manifest digest is verified by TestPackDemoFixtureDigest.
const demoFixtureSource = "capability-pack-demo"

func newPackRegistry(t *testing.T, root string) (ports.PackRegistry, *journalmem.Store) {
	t.Helper()
	j := journalmem.NewJournal()
	ts := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	return fsregistry.New(root, j, idgen.NewGenerator(clock.Fixed(ts)), clock.Fixed(ts)), j
}

// The committed fixture's digest is stable: the canonical digest computed
// from the committed manifest.json must equal the declared integrity_digest.
// If this test fails after editing the fixture, recompute the digest — do
// NOT edit the expectation.
func TestPackDemoFixtureDigest(t *testing.T) {
	reg, _ := newPackRegistry(t, examplesRoot)
	if err := reg.Verify(context.Background(), demoFixtureSource); err != nil {
		t.Fatalf("committed fixture digest invalid: %v", err)
	}
}

// S18/S21 — activation E2E through the filesystem registry, with the event
// visible via journal Replay (the pack subsystem's only consumer surface in
// M5.1).
func TestPackActivationE2E(t *testing.T) {
	reg, journal := newPackRegistry(t, examplesRoot)

	pack, err := reg.Load(context.Background(), demoFixtureSource)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if pack.Name != "golem-demo" {
		t.Fatalf("fixture name = %q, want golem-demo", pack.Name)
	}

	pos, err := reg.Activate(context.Background(), "tenant-e2e", pack)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if pos == 0 {
		t.Fatal("position = 0, want a real journal position")
	}

	events, _, err := journal.Replay(context.Background(), 0, 100)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	var found bool
	for _, e := range events {
		if e.EventType != ports.EventExtensionPackActivated {
			continue
		}
		found = true
		if !strings.HasPrefix(e.EventType, ports.ReservedEventPrefixExtensionPack) {
			t.Errorf("event type %q escapes the reserved prefix", e.EventType)
		}
		if e.TenantID != "tenant-e2e" {
			t.Errorf("tenant = %q", e.TenantID)
		}
		if e.StreamID != "extension.pack.tenant-e2e.golem-demo" {
			t.Errorf("stream = %q", e.StreamID)
		}
		var payload map[string]any
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			t.Fatalf("payload decode: %v", err)
		}
		if payload["name"] != "golem-demo" || payload["version"] != "0.1.0" {
			t.Errorf("payload identity = %v/%v", payload["name"], payload["version"])
		}
		if _, dup := payload["tenant_id"]; dup {
			t.Error("payload duplicates envelope field tenant_id (Q7 violation)")
		}
	}
	if !found {
		t.Fatal("extension.pack.activated.v1 not found in journal replay")
	}

	st, err := reg.Status(context.Background(), "tenant-e2e", "golem-demo")
	if err != nil || st == nil {
		t.Fatalf("Status = (%v, %v)", st, err)
	}
	if st.ActivatedEventID == "" {
		t.Error("Status missing ActivatedEventID")
	}
}

// S14 — re-activation is rejected; the journal still holds exactly one event.
func TestPackActivationIdempotencyE2E(t *testing.T) {
	reg, journal := newPackRegistry(t, examplesRoot)
	pack, err := reg.Load(context.Background(), demoFixtureSource)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := reg.Activate(context.Background(), "t-idem", pack); err != nil {
		t.Fatalf("first Activate: %v", err)
	}
	if _, err := reg.Activate(context.Background(), "t-idem", pack); !errors.Is(err, ports.ErrPackAlreadyActivated) {
		t.Fatalf("second Activate err = %v, want ErrPackAlreadyActivated", err)
	}
	events, _, _ := journal.Replay(context.Background(), 0, 100)
	var count int
	for _, e := range events {
		if e.EventType == ports.EventExtensionPackActivated {
			count++
		}
	}
	if count != 1 {
		t.Errorf("activation events = %d, want 1", count)
	}
}

// writeTempPack writes a manifest into a temp registry root.
func writeTempPack(t *testing.T, root, name string, manifest string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func validDemoManifestJSON() string {
	return `{"format_version":"1","name":"tmp-pack","version":"1.0.0",` +
		`"golem_api":">=0.5 <0.6","permissions":["scm.read"],` +
		`"integrity_digest":"REPLACED"}`
}

// S5/S6 — tampered digest and migration-declaring packs are rejected without
// journal mutation.
func TestPackActivationRejectionsE2E(t *testing.T) {
	root := t.TempDir()

	// Digest tampered.
	tampered := strings.Replace(validDemoManifestJSON(), "REPLACED", strings.Repeat("c", 64), 1)
	writeTempPack(t, root, "tampered", tampered)

	// Migrations declared → ErrUnsupportedInM51 (structural pass, gate fires).
	// Digest is irrelevant: the gate runs before digest verification.
	migrations := `{"format_version":"1","name":"mig-pack","version":"1.0.0",` +
		`"golem_api":">=0.5 <0.6","permissions":[],` +
		`"migrations":[{"ID":"001","Description":"d","Script":"s"}],` +
		`"integrity_digest":"` + strings.Repeat("a", 64) + `"}`
	writeTempPack(t, root, "migs", migrations)

	// Unknown permission.
	unknown := strings.Replace(
		strings.Replace(validDemoManifestJSON(), "REPLACED", strings.Repeat("a", 64), 1),
		"scm.read", "proposal:create", 1)
	writeTempPack(t, root, "unknown-perm", unknown)

	reg, journal := newPackRegistry(t, root)

	cases := []struct {
		src  string
		want error
	}{
		{"tampered", ports.ErrPackIntegrityFailed},
		{"migs", ports.ErrUnsupportedInM51},
		{"unknown-perm", ports.ErrPackUnknownPermission},
		{"missing", ports.ErrPackNotFound},
	}
	for _, c := range cases {
		pack, err := reg.Load(context.Background(), c.src)
		if err == nil {
			// Load may pass (tampered digests are structurally valid); the
			// gate must fire at Activate.
			_, err = reg.Activate(context.Background(), "t-reject", pack)
		}
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.src, err, c.want)
		}
	}

	if h, _ := journal.Head(context.Background()); h != 0 {
		t.Errorf("journal mutated by rejected activations: head = %d", h)
	}
}
