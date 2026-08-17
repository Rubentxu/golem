// This file marks the documentation corpus as a separate, never-built Go
// module so the parent module's ./... patterns do not try to compile the
// reference examples (which intentionally contain multiple packages in one
// directory). The corpus is immutable; see MANIFEST.json.
module golem/documentation

go 1.26
