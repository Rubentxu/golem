package profile

import "testing"

// TestProfileLoadDevDefault verifies that the embedded dev profile
// matches the hardcoded DevProfile().
func TestProfileLoadDevDefault(t *testing.T) {
	profile, err := Load("dev")
	if err != nil {
		t.Fatalf("Load(dev): %v", err)
	}

	dev := DevProfile()
	if profile.Version != dev.Version {
		t.Errorf("Version = %d, want %d", profile.Version, dev.Version)
	}
	if profile.Name != dev.Name {
		t.Errorf("Name = %q, want %q", profile.Name, dev.Name)
	}
	if len(profile.Adapters) != len(dev.Adapters) {
		t.Errorf("Adapters len = %d, want %d", len(profile.Adapters), len(dev.Adapters))
	}
	for port, kind := range dev.Adapters {
		if profile.Adapters[port] != kind {
			t.Errorf("Adapters[%q] = %q, want %q", port, profile.Adapters[port], kind)
		}
	}
}

// TestProfileLoadFromEnv verifies LoadFromEnv uses the GOLEM_PROFILE env var.
func TestProfileLoadFromEnv(t *testing.T) {
	t.Setenv("GOLEM_PROFILE", "dev")
	profile, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	if profile.Name != "dev" {
		t.Errorf("Name = %q, want %q", profile.Name, "dev")
	}
}

// TestProfileLoadEmptyNameFallsToDev verifies that Load("") falls back to dev.
func TestProfileLoadEmptyNameFallsToDev(t *testing.T) {
	profile, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}
	if profile.Name != "dev" {
		t.Errorf("Name = %q, want %q (empty name should fallback to dev)", profile.Name, "dev")
	}
}

// TestProfileValidateUnknownPort verifies unknown ports are rejected.
func TestProfileValidateUnknownPort(t *testing.T) {
	data := []byte(`{"version":1,"name":"test","adapters":{"journal":"memstore","unknown-port":"memstore"}}`)
	_, err := parseAndValidate(data)
	if err == nil {
		t.Fatal("expected error for unknown port, got nil")
	}
}

// TestProfileValidateUnknownKind verifies unknown kinds are rejected.
func TestProfileValidateUnknownKind(t *testing.T) {
	data := []byte(`{"version":1,"name":"test","adapters":{"journal":"unknown-kind"}}`)
	_, err := parseAndValidate(data)
	if err == nil {
		t.Fatal("expected error for unknown kind, got nil")
	}
}

// TestProfileValidateFormatVersionMismatch verifies format_version != 1 is rejected.
func TestProfileValidateFormatVersionMismatch(t *testing.T) {
	data := []byte(`{"version":99,"name":"test","adapters":{"journal":"memstore"}}`)
	_, err := parseAndValidate(data)
	if err == nil {
		t.Fatal("expected ErrUnsupportedFormatVersion, got nil")
	}
}

// TestProfileValidateMissingAdapters verifies missing adapters map is rejected.
func TestProfileValidateMissingAdapters(t *testing.T) {
	data := []byte(`{"version":1,"name":"test"}`)
	_, err := parseAndValidate(data)
	if err == nil {
		t.Fatal("expected error for missing adapters, got nil")
	}
}

// TestProfileAdapterHelper verifies the Adapter() helper.
func TestProfileAdapterHelper(t *testing.T) {
	p := Profile{
		Adapters: map[string]string{"journal": "bbolt"},
	}
	if got := p.Adapter("journal"); got != "bbolt" {
		t.Errorf("Adapter(journal) = %q, want %q", got, "bbolt")
	}
	if got := p.Adapter("unknown"); got != "" {
		t.Errorf("Adapter(unknown) = %q, want %q", got, "")
	}
}

// TestProfileOptionHelper verifies the Option() helper.
func TestProfileOptionHelper(t *testing.T) {
	p := Profile{
		Options: map[string]any{
			"bbolt": map[string]any{"path": "/tmp/golem.db"},
		},
	}
	opts := p.Option("bbolt")
	if opts == nil {
		t.Fatal("Option(bbolt) = nil, want non-nil")
	}
	if opts["path"] != "/tmp/golem.db" {
		t.Errorf("Option(bbolt)[path] = %v, want %v", opts["path"], "/tmp/golem.db")
	}
	if p.Option("unknown") != nil {
		t.Errorf("Option(unknown) = non-nil, want nil")
	}
}
