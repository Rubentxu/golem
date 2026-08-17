// Package supplychain provides domain types and utilities for the supply chain
// security kernel: purl normalization, SBOM events, and graph-model constants.
package supplychain

import (
	"net/url"
	"strings"
)

// ErrInvalidPurl is returned when a purl string fails validation.
var ErrInvalidPurl = &purlError{msg: "invalid purl: must start with 'pkg:' and have a non-empty name"}

type purlError struct{ msg string }

func (e *purlError) Error() string { return e.msg }

// Normalize validates and normalizes a package URL.
// It lowercases the scheme ("pkg") and type segment, trims whitespace,
// percent-decodes the namespace and name, validates the "pkg:" prefix and
// non-empty name, and preserves version and qualifiers as-is.
// Returns the normalized purl string or an error.
func Normalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidPurl
	}

	// Accept "pkg:" or "PKG:" prefix (scheme is case-insensitive per RFC 3986).
	if len(raw) < 4 || !strings.HasPrefix(strings.ToLower(raw[:4]), "pkg:") {
		return "", ErrInvalidPurl
	}

	// Strip "pkg:" prefix and parse the rest.
	rest := raw[4:]
	if rest == "" {
		return "", ErrInvalidPurl
	}

	// Split off qualifiers (?...) if present.
	var qualifiers string
	if qIdx := strings.Index(rest, "?"); qIdx >= 0 {
		qualifiers = rest[qIdx:] // includes the "?"
		rest = rest[:qIdx]
	}

	// Extract type: everything before the first '/' is the type.
	slashIdx := strings.Index(rest, "/")
	if slashIdx <= 0 {
		return "", ErrInvalidPurl
	}
	typ := strings.ToLower(rest[:slashIdx])
	pathPart := rest[slashIdx+1:]

	// Extract version from pathPart if present (last @... is the version).
	var name, version string
	if atIdx := strings.LastIndex(pathPart, "@"); atIdx >= 0 {
		version = pathPart[atIdx:]
		pathPart = pathPart[:atIdx]
	}

	// Split namespace and name from pathPart.
	// pathPart is either "name" or "namespace/name" or "namespace/subnamespace/name"
	segs := strings.Split(pathPart, "/")
	if len(segs) == 0 || segs[0] == "" {
		return "", ErrInvalidPurl
	}
	var namespace string
	if len(segs) == 1 {
		name = segs[0]
	} else {
		namespace = strings.Join(segs[:len(segs)-1], "/")
		name = segs[len(segs)-1]
	}

	// Percent-decode namespace and name.
	if namespace != "" {
		if dec, err := url.QueryUnescape(namespace); err == nil {
			namespace = dec
		}
	}
	if name != "" {
		if dec, err := url.QueryUnescape(name); err == nil {
			name = dec
		}
	}

	// Validate non-empty name.
	if name == "" {
		return "", ErrInvalidPurl
	}

	// Reconstruct: pkg:type[/namespace]/name[@version][?qualifiers]
	var result strings.Builder
	result.WriteString("pkg:")
	result.WriteString(typ)
	result.WriteString("/")
	if namespace != "" {
		result.WriteString(namespace)
		result.WriteString("/")
	}
	result.WriteString(name)
	if version != "" {
		result.WriteString(version)
	}
	if qualifiers != "" {
		result.WriteString(qualifiers)
	}

	return result.String(), nil
}

// Synthetic constructs a synthetic purl for a component that lacks a native purl.
// The returned purl uses the "pkg:generic" scheme with the given name and version.
func Synthetic(rawName, version string) string {
	name := strings.TrimSpace(rawName)
	if name == "" {
		name = "unknown"
	}
	if version == "" {
		version = "unknown"
	}
	return "pkg:generic/" + name + "@" + version
}
