package packs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Rubentxu/golem/internal/ports"
)

// Manifest represents the capability pack manifest as defined in
// CAPABILITY_PACKS.md §Manifest. All JSON tags are verbatim to match the
// canonical schema used for integrity digest computation.
//
// The manifest is the authoritative declaration of a pack's capabilities,
// permissions, and metadata. It is loaded by a PackRegistry adapter,
// validated by Manifest.Validate, and passed to Activator.Activate.
type Manifest struct {
	FormatVersion   string           `json:"format_version"`         // MUST be "1"
	Name            string           `json:"name"`                   //
	Version         string           `json:"version"`                // semver
	GolemAPI        string           `json:"golem_api"`              // semver range, e.g. ">=0.5 <0.6"
	Capabilities    []string         `json:"capabilities,omitempty"` //
	Permissions     []string         `json:"permissions,omitempty"`  //
	Entrypoints     Entrypoints      `json:"entrypoints"`            //
	Budgets         Budgets          `json:"budgets"`                //
	Ontology        []string         `json:"ontology,omitempty"`     //
	Migrations      []Migration      `json:"migrations,omitempty"`   // structural in M5.1; activation rejects len>0
	UI              []UIContribution `json:"ui,omitempty"`           // structural in M5.1; activation rejects len>0
	IntegrityDigest string           `json:"integrity_digest"`       // sha256 hex (64 chars) of this manifest in canonical JSON
}

// Entrypoints declares how the pack's capabilities are invoked.
type Entrypoints struct {
	Wasm   []string `json:"wasm,omitempty"`
	Remote []string `json:"remote,omitempty"`
}

// Budgets declares runtime resource budgets for the pack.
type Budgets struct {
	MaxMs       int `json:"max_ms,omitempty"`
	MaxMemoryMB int `json:"max_memory_mb,omitempty"`
}

// Migration declares a named migration script. The schema is validated
// structurally (M5.1); activation rejects len>0 via ErrUnsupportedInM51.
type Migration struct {
	ID          string `json:"ID"`
	Description string `json:"Description"`
	Script      string `json:"Script"`
}

// UIContribution declares a UI asset contributed by the pack. The schema is
// validated structurally (M5.1); activation rejects len>0 via ErrUnsupportedInM51.
type UIContribution struct {
	Type  string `json:"Type"`
	Path  string `json:"Path"`
	Label string `json:"Label"`
}

// Validate checks that the manifest passes all semantic checks that can be
// performed without access to external state (catalog, journal, etc.).
//
// Returns nil if the manifest is valid; returns an error wrapping one of:
//   - ports.ErrPackManifestInvalid  — format_version != "1", integrity_digest
//     malformed, or golem_api does not intersect the supported range
//   - ports.ErrPackUnknownPermission — a permission is not in the closed v1 catalog
func (m *Manifest) Validate(catalog PermissionCatalog) error {
	if m.FormatVersion != "1" {
		return fmt.Errorf("%w: format_version is %q, expected \"1\"",
			ports.ErrPackManifestInvalid, m.FormatVersion)
	}
	if !isHex64(m.IntegrityDigest) {
		return fmt.Errorf("%w: integrity_digest must be a 64-character sha256 hex string",
			ports.ErrPackManifestInvalid)
	}
	if !golemAPIIntersectsSupported(m.GolemAPI) {
		return fmt.Errorf("%w: golem_api %q does not intersect [0.5, 0.6)",
			ports.ErrPackManifestInvalid, m.GolemAPI)
	}
	for _, perm := range m.Permissions {
		if !IsKnownPermission(perm, catalog) {
			return fmt.Errorf("%w: %q", ports.ErrPackUnknownPermission, perm)
		}
	}
	return nil
}

// semverBound is a (major, minor) pair compared lexicographically. Patch and
// pre-release segments are ignored for range intersection: GOLEM compatibility
// ranges operate at minor granularity.
type semverBound struct {
	major, minor int
}

func (a semverBound) less(b semverBound) bool {
	return a.major < b.major || (a.major == b.major && a.minor < b.minor)
}

// parseSemverBound extracts the leading "major.minor" of a version literal
// ("0.5", "v0.5.3", "0.6-beta"). Returns an error when no numeric prefix parses.
func parseSemverBound(s string) (semverBound, error) {
	s = strings.TrimPrefix(s, "v")
	major, minor, ok := 0, 0, false
	if _, err := fmt.Sscanf(s, "%d.%d", &major, &minor); err == nil {
		ok = true
	} else if _, err := fmt.Sscanf(s, "%d", &major); err == nil {
		minor, ok = 0, true
	}
	if !ok {
		return semverBound{}, fmt.Errorf("invalid semver literal %q", s)
	}
	return semverBound{major, minor}, nil
}

// supported API series for GOLEM 0.5.x: the 0.5 minor series.
const supportedSeries = 5 // major*1000 + minor of (0, 5)

// golemAPIIntersectsSupported reports whether the given semver range string
// (e.g. ">=0.5 <0.6", ">=0.1 <1.0", "0.5") includes the 0.5 minor series.
// Comparators operate at minor-series granularity: "<0.5" excludes the
// series, "<=0.5" and "0.5" include it, ">0.5" starts after it.
// Unparseable constraints fail CLOSED — a manifest that declares garbage
// compatibility must not activate.
func golemAPIIntersectsSupported(rangeStr string) bool {
	loSeries, loInclusive := 0, true      // default: no lower bound
	hiSeries, hiInclusive := 1<<30, false // default: no upper bound

	for _, part := range strings.Fields(rangeStr) {
		var op, lit string
		switch {
		case strings.HasPrefix(part, ">="), strings.HasPrefix(part, "<="):
			op, lit = part[:2], part[2:]
		case strings.HasPrefix(part, ">"), strings.HasPrefix(part, "<"), strings.HasPrefix(part, "="):
			op, lit = part[:1], part[1:]
		default:
			op, lit = "=", part
		}
		b, err := parseSemverBound(lit)
		if err != nil {
			return false
		}
		s := b.major*1000 + b.minor
		switch op {
		case ">=":
			loSeries, loInclusive = s, true
		case ">":
			loSeries, loInclusive = s, false
		case "<=":
			hiSeries, hiInclusive = s, true
		case "<":
			hiSeries, hiInclusive = s, false
		case "=":
			loSeries, loInclusive = s, true
			hiSeries, hiInclusive = s, true
		}
	}

	loOK := loSeries < supportedSeries || (loInclusive && loSeries == supportedSeries)
	hiOK := hiSeries > supportedSeries || (hiInclusive && hiSeries == supportedSeries)
	return loOK && hiOK
}

// isHex64 returns true when s is exactly 64 lowercase hexadecimal characters.
func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// ComputeDigest returns the sha256 hex digest of the manifest serialised in
// canonical JSON (keys sorted recursively, no insignificant whitespace).
// The integrity_digest field itself is normalised to empty before hashing —
// a digest cannot cover itself. The same manifest with different key
// ordering yields the same digest — this is the determinism contract of Q3.
func (m *Manifest) ComputeDigest() (string, error) {
	normalized := *m
	normalized.IntegrityDigest = ""
	canon, err := MarshalCanonical(&normalized)
	if err != nil {
		return "", fmt.Errorf("packs: canonical serialisation: %w", err)
	}
	sum := sha256.Sum256(canon)
	return hex.EncodeToString(sum[:]), nil
}

// DigestMatchesCanonical verifies the manifest's declared integrity_digest
// against the canonical digest of the raw manifest bytes. The raw bytes are
// parsed and re-serialised canonically before hashing, so files that differ
// only in key ordering or whitespace verify identically.
func (m *Manifest) DigestMatchesCanonical(rawManifestBytes []byte) bool {
	var parsed Manifest
	if err := json.Unmarshal(rawManifestBytes, &parsed); err != nil {
		return false
	}
	computed, err := parsed.ComputeDigest()
	if err != nil {
		return false
	}
	return computed == m.IntegrityDigest
}
