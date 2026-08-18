// Package profile provides the provider profile loader for GOLEM.
// It resolves golem-profile.{name}.yaml from well-known paths and returns
// a typed Profile used to construct runtime.Options at bootstrap.
//
// Selection happens at bootstrap only (cmd/golem-api, cmd/golem-worker).
// internal/domain and internal/application never import this package —
// archtest enforces the boundary.
//
// ADR-057 §1-§5.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// ErrUnsupportedFormatVersion is returned when the profile format_version
// is not "1".
var ErrUnsupportedFormatVersion = errors.New("profile: unsupported format_version (supported: [\"1\"])")

// Profile describes the adapter selection for one runtime environment.
// It is parsed from a JSON-shaped YAML file via encoding/json.
type Profile struct {
	Version  int               `json:"version"`        // Must be 1
	Name     string            `json:"name"`           // "dev" | "durable"
	Adapters map[string]string `json:"adapters"`       // adapter kind per port
	Options  map[string]any    `json:"options"`        // adapter-specific knobs
	Eval     *EvalConfig       `json:"eval,omitempty"` // eval harness config
}

// EvalConfig configures the eval harness.
type EvalConfig struct {
	Enabled  bool   `json:"enabled"`
	Fixtures string `json:"fixtures,omitempty"` // path to fixtures directory
}

// adapterKinds enumerates the known adapter kinds per port.
var adapterKinds = map[string][]string{
	"journal":    {"memstore", "bbolt"},
	"graph":      {"memstore"},
	"registry":   {"memstore"},
	"transport":  {"memstore", "natsjs"},
	"checkpoint": {"memstore"},
	"search":     {"memstore"},
	"llm":        {"memstore", "openai-compatible"},
	"policy":     {"memstore"},
}

// Load resolves and validates the profile with the given name.
// Resolution order: ./, $GOLEM_PROFILE_DIR/, /etc/golem/.
//
// Embedded defaults:
//   - name == "" or "dev"  → DevProfile() (all memstore)
//   - name == "durable"     → DurableProfile() (bbolt + natsjs)
//   - name is unknown and no file found → fail-fast with clear error
func Load(name string) (Profile, error) {
	if name == "" {
		name = "dev"
	}

	filename := fmt.Sprintf("golem-profile.%s.yaml", name)
	dirs := []string{
		".",
		os.Getenv("GOLEM_PROFILE_DIR"),
		"/etc/golem",
	}

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, filename)
		data, err := os.ReadFile(path)
		if err == nil {
			profile, err := parseAndValidate(data)
			if err != nil {
				return Profile{}, fmt.Errorf("profile load: %w", err)
			}
			return profile, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return Profile{}, fmt.Errorf("profile load: reading %s: %w", path, err)
		}
	}

	// No file found — use embedded defaults for known profiles.
	if _, ok := knownProfiles[name]; ok {
		if name == "durable" {
			log.Printf("profile=%s source=embedded", name)
			return DurableProfile(), nil
		}
		// dev and ""
		log.Printf("profile=%s source=embedded", name)
		return DevProfile(), nil
	}

	// Unknown profile name with no file → fail fast.
	return Profile{}, fmt.Errorf("profile %q not found (no golem-profile.%s.yaml in ./, $GOLEM_PROFILE_DIR/, /etc/golem/) and no embedded defaults available", name, name)
}

// LoadFromEnv reads GOLEM_PROFILE from the environment, defaulting to "dev".
func LoadFromEnv() (Profile, error) {
	name := os.Getenv("GOLEM_PROFILE")
	if name == "" {
		name = "dev"
	}
	return Load(name)
}

// parseAndValidate decodes raw YAML bytes and validates the profile shape.
func parseAndValidate(data []byte) (Profile, error) {
	// JSON-shaped YAML: parse with encoding/json.
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("profile parse: %w", err)
	}

	if err := validate(&p); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// validate checks the profile schema: version, adapters map, known kinds.
func validate(p *Profile) error {
	if p.Version != 1 {
		return fmt.Errorf("%w: %d (supported: [1])", ErrUnsupportedFormatVersion, p.Version)
	}

	if p.Adapters == nil {
		return errors.New("profile: adapters map is required")
	}

	// Validate each adapter kind is known.
	for port, kind := range p.Adapters {
		kinds, ok := adapterKinds[port]
		if !ok {
			return fmt.Errorf("profile: unknown port %q", port)
		}
		if !contains(kinds, kind) {
			return fmt.Errorf("profile: unknown kind %q for port %q (known: %v)", kind, port, kinds)
		}
	}

	return nil
}

// contains reports whether v is in list.
func contains[T comparable](list []T, v T) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Adapter returns the adapter kind for the given port, or "" if not configured.
func (p Profile) Adapter(port string) string {
	if p.Adapters == nil {
		return ""
	}
	return p.Adapters[port]
}

// Option returns the option value for the given adapter, or nil if not set.
func (p Profile) Option(adapter string) map[string]any {
	if p.Options == nil {
		return nil
	}
	val, ok := p.Options[adapter]
	if !ok {
		return nil
	}
	m, ok := val.(map[string]any)
	if !ok {
		return nil
	}
	return m
}

// DevProfile returns the embedded dev profile (all memstore adapters).
func DevProfile() Profile {
	return Profile{
		Version: 1,
		Name:    "dev",
		Adapters: map[string]string{
			"journal":    "memstore",
			"graph":      "memstore",
			"registry":   "memstore",
			"transport":  "memstore",
			"checkpoint": "memstore",
			"search":     "memstore",
			"llm":        "memstore",
			"policy":     "memstore",
		},
		Options: nil,
		Eval: &EvalConfig{
			Enabled:  true,
			Fixtures: "./fixtures/cases",
		},
	}
}

// DurableProfile returns the embedded durable profile (bbolt journal + natsjs transport).
// Options carry the default bbolt path; callers may override at runtime.
func DurableProfile() Profile {
	return Profile{
		Version: 1,
		Name:    "durable",
		Adapters: map[string]string{
			"journal":    "bbolt",
			"graph":      "memstore",
			"registry":   "memstore",
			"transport":  "natsjs",
			"checkpoint": "memstore",
			"search":     "memstore",
			"llm":        "openai-compatible",
			"policy":     "memstore",
		},
		Options: map[string]any{
			"bbolt": map[string]any{
				"path": "./var/golem.journal",
			},
		},
		Eval: &EvalConfig{
			Enabled:  true,
			Fixtures: "./fixtures/cases",
		},
	}
}

// knownProfiles is the set of profiles with embedded defaults.
var knownProfiles = map[string]struct{}{
	"dev":     {},
	"durable": {},
}
