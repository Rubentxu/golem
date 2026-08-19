package profile

import (
	"os"
	"path/filepath"
	"testing"
)

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

// TestProfileLoadDevFromCwd verifies that dev profile is resolved from cwd when
// golem-profile.dev.yaml exists there (S2).
func TestProfileLoadDevFromCwd(t *testing.T) {
	tmpDir := t.TempDir()
	profileFile := filepath.Join(tmpDir, "golem-profile.dev.yaml")
	content := `{"version":1,"name":"custom-dev","adapters":{"journal":"memstore","graph":"memstore","registry":"memstore","transport":"memstore","checkpoint":"memstore","search":"memstore"}}`
	if err := os.WriteFile(profileFile, []byte(content), 0644); err != nil {
		t.Fatalf("write profile file: %v", err)
	}

	// Change to the tmp dir so Load resolves from ".".
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getcwd: %v", err)
	}
	defer os.Chdir(oldCwd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	profile, err := Load("dev")
	if err != nil {
		t.Fatalf("Load(dev): %v", err)
	}
	if profile.Name != "custom-dev" {
		t.Errorf("Name = %q, want %q", profile.Name, "custom-dev")
	}
}

// TestProfileLoadDurableEmbeddedDefaults verifies that GOLEM_PROFILE=durable
// without a file returns DurableProfile() embedded defaults (S4+S5).
func TestProfileLoadDurableEmbeddedDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getcwd: %v", err)
	}
	defer os.Chdir(oldCwd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	profile, err := Load("durable")
	if err != nil {
		t.Fatalf("Load(durable): %v", err)
	}
	if profile.Name != "durable" {
		t.Errorf("Name = %q, want %q", profile.Name, "durable")
	}
	// S5: durable selects bbolt for journal and natsjs for transport.
	if profile.Adapter("journal") != "bbolt" {
		t.Errorf("Adapters[journal] = %q, want %q", profile.Adapter("journal"), "bbolt")
	}
	if profile.Adapter("transport") != "natsjs" {
		t.Errorf("Adapters[transport] = %q, want %q", profile.Adapter("transport"), "natsjs")
	}
	// Other adapters default to memstore.
	if profile.Adapter("graph") != "memstore" {
		t.Errorf("Adapters[graph] = %q, want %q", profile.Adapter("graph"), "memstore")
	}
	// Options carry bbolt path default.
	opts := profile.Option("bbolt")
	if opts == nil {
		t.Fatal("Option(bbolt) = nil, want non-nil")
	}
	if opts["path"] == nil {
		t.Error("Option(bbolt)[path] = nil, want non-nil")
	}
}

// TestProfileLoadUnknownProfileFailFast verifies that an unknown profile name
// without a file returns a clear error (C-1: no silent dev fallback).
func TestProfileLoadUnknownProfileFailFast(t *testing.T) {
	tmpDir := t.TempDir()
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getcwd: %v", err)
	}
	defer os.Chdir(oldCwd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, err = Load("nonexistent")
	if err == nil {
		t.Fatal("Load(nonexistent): expected error, got nil")
	}
	// Error message should mention the unknown profile name.
}

// TestProfile_Validate_LLMAndPolicyRequired verifies that llm and policy adapters
// are validated correctly (REQ-001).
func TestProfile_Validate_LLMAndPolicyRequired(t *testing.T) {
	// Valid profile with llm and policy adapters
	data := []byte(`{"version":1,"name":"test","adapters":{"journal":"memstore","graph":"memstore","registry":"memstore","transport":"memstore","checkpoint":"memstore","search":"memstore","llm":"memstore","policy":"memstore"}}`)
	p, err := parseAndValidate(data)
	if err != nil {
		t.Fatalf("valid profile failed: %v", err)
	}
	if p.Adapter("llm") != "memstore" {
		t.Errorf("Adapter(llm) = %q, want memstore", p.Adapter("llm"))
	}
	if p.Adapter("policy") != "memstore" {
		t.Errorf("Adapter(policy) = %q, want memstore", p.Adapter("policy"))
	}
}

// TestProfile_LoadProd verifies that the prod.yaml profile validates
// with all M8 port keys (AC-1 / ESC-001).
func TestProfile_LoadProd(t *testing.T) {
	tmpDir := t.TempDir()
	prodYaml := `{"version":1,"name":"prod","adapters":{"journal":"bbolt","graph":"memstore","registry":"memstore","transport":"memstore","checkpoint":"memstore","search":"memstore","llm":"openai-compatible","policy":"memstore","cell-router":"staticrouter","tenant-catalog":"memstore","quota":"memstore","meter":"meter","paging":"webhook","slo":"slo","authn":"oidc","pack_registry":"filesystem"}}`
	profileFile := filepath.Join(tmpDir, "golem-profile.prod.yaml")
	if err := os.WriteFile(profileFile, []byte(prodYaml), 0644); err != nil {
		t.Fatalf("write prod profile: %v", err)
	}

	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getcwd: %v", err)
	}
	defer os.Chdir(oldCwd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	profile, err := Load("prod")
	if err != nil {
		t.Fatalf("Load(prod): %v", err)
	}
	if profile.Name != "prod" {
		t.Errorf("Name = %q, want prod", profile.Name)
	}
	// Verify all 8 M8 ports are present
	for _, port := range []string{"cell-router", "tenant-catalog", "quota", "meter", "paging", "slo", "authn"} {
		if profile.Adapter(port) == "" {
			t.Errorf("Adapter(%q) = empty, want non-empty", port)
		}
	}
	if profile.Adapter("pack_registry") == "" {
		t.Error("Adapter(pack_registry) = empty, want filesystem")
	}
}

// TestProfile_EvalConfig verifies that eval config is parsed correctly.
func TestProfile_EvalConfig(t *testing.T) {
	dev := DevProfile()
	if dev.Eval == nil {
		t.Fatal("DevProfile().Eval = nil, want non-nil")
	}
	if !dev.Eval.Enabled {
		t.Error("Eval.Enabled = false, want true")
	}
	if dev.Eval.Fixtures == "" {
		t.Error("Eval.Fixtures = empty, want non-empty")
	}
	durable := DurableProfile()
	if durable.Eval == nil {
		t.Fatal("DurableProfile().Eval = nil, want non-nil")
	}
	if !durable.Eval.Enabled {
		t.Error("Eval.Enabled = false, want true")
	}
}
