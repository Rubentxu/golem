package packs

import "slices"

// PermissionCatalog is the closed v1 permission catalogue for capability packs.
// It is transcribed verbatim from PLUGIN_RUNTIME.md §Permissions (Q1 resolution).
// A pack's manifest must declare only permissions from this set; unknown
// permissions cause ErrPackUnknownPermission during manifest validation.
type PermissionCatalog struct {
	permissions []string
}

// NewPermissionCatalogV1 returns the canonical closed permission catalogue v1.
// Transcribed verbatim from PLUGIN_RUNTIME.md §Permissions.
// Note: golem-documentation/examples/capability-pack/manifest.json is a stale
// aspirational example (it mixes capabilities and permissions) — when the two
// disagree, PLUGIN_RUNTIME.md wins.
func NewPermissionCatalogV1() PermissionCatalog {
	return PermissionCatalog{
		permissions: []string{
			"graph.read:lens",
			"proposal.write",
			"object.read",
			"scm.read",
			"ci.trigger",
		},
	}
}

// Allowed returns true iff p is in the catalogue.
func (c PermissionCatalog) Allowed(p string) bool {
	return slices.Contains(c.permissions, p)
}

// IsKnownPermission returns true when the given permission string is present
// in the closed v1 catalogue. Unknown permissions cause manifest validation
// to fail with ErrPackUnknownPermission.
func IsKnownPermission(permission string, catalog PermissionCatalog) bool {
	return catalog.Allowed(permission)
}
