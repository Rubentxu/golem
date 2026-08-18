package profile

import (
	"errors"
	"fmt"
	"strings"
)

// validateSchema checks structural invariants of a parsed Profile.
// Semantic validation (file resolution, env precedence) lives in profile.go.
func validateSchema(p *Profile) error {
	if p.Version != 1 {
		return fmt.Errorf("%w: %d (supported: [\"1\"])", ErrUnsupportedFormatVersion, p.Version)
	}

	if p.Name == "" {
		return errors.New("profile: name is required")
	}

	if !isValidName(p.Name) {
		return fmt.Errorf("profile: name %q must match [a-z][a-z0-9-]*", p.Name)
	}

	if p.Adapters == nil {
		return errors.New("profile: adapters map is required")
	}

	// Every configured port must have a known kind.
	for port, kind := range p.Adapters {
		if err := validateAdapterKind(port, kind); err != nil {
			return err
		}
	}

	// Options values may be any JSON type; we only validate structure
	// when it is a map (common case).
	if p.Options != nil {
		for key, val := range p.Options {
			if _, ok := val.(map[string]any); ok {
				if err := validateOptionMap(key, val.(map[string]any)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// isValidName reports whether a profile name matches [a-z][a-z0-9-]*.
func isValidName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for i, c := range name {
		if c == '-' {
			continue
		}
		if c >= 'a' && c <= 'z' {
			continue
		}
		if i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}

// validateAdapterKind returns an error if port/kind is not in the known set.
func validateAdapterKind(port, kind string) error {
	kinds, ok := knownAdapterKinds[port]
	if !ok {
		return fmt.Errorf("profile: unknown port %q", port)
	}
	for _, k := range kinds {
		if k == kind {
			return nil
		}
	}
	return fmt.Errorf("profile: unknown adapter kind %q for port %q (known: %v)", kind, port, kinds)
}

// validateOptionMap validates the structure of adapter-specific options.
// For bbolt: path must be a non-empty string.
// For natsjs: url must be a non-empty string.
func validateOptionMap(adapter string, opts map[string]any) error {
	switch adapter {
	case "bbolt":
		if path, ok := opts["path"]; ok {
			if path == "" {
				return errors.New("profile: options.bbolt.path must be non-empty")
			}
		}
	case "natsjs":
		if url, ok := opts["url"]; ok {
			if url == "" {
				return errors.New("profile: options.natsjs.url must be non-empty")
			}
		}
	}
	return nil
}

// knownAdapterKinds maps each port to its allowed adapter kinds.
var knownAdapterKinds = map[string][]string{
	"journal":    {"memstore", "bbolt"},
	"graph":      {"memstore"},
	"registry":   {"memstore"},
	"transport":  {"memstore", "natsjs"},
	"checkpoint": {"memstore"},
	"search":     {"memstore"},
}

// StripComments removes // and /* */ comments from YAML text.
// Since we parse JSON-shaped YAML with encoding/json, this is a no-op
// for valid JSON; it only handles accidental comment syntax.
func StripComments(data string) string {
	var out strings.Builder
	inLineComment := false
	inBlockComment := false
	i := 0
	for i < len(data) {
		if !inBlockComment && !inLineComment && i+1 < len(data) {
			if data[i] == '/' && data[i+1] == '/' {
				inLineComment = true
				i += 2
				continue
			}
			if data[i] == '/' && data[i+1] == '*' {
				inBlockComment = true
				i += 2
				continue
			}
		}
		if inLineComment {
			if data[i] == '\n' {
				inLineComment = false
				out.WriteByte(data[i])
			}
			i++
			continue
		}
		if inBlockComment {
			if i+1 < len(data) && data[i] == '*' && data[i+1] == '/' {
				inBlockComment = false
				i += 2
				continue
			}
			i++
			continue
		}
		out.WriteByte(data[i])
		i++
	}
	return out.String()
}
