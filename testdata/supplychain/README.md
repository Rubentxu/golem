# SP-010 Supply Chain Security Test Fixtures

This directory contains minimal-but-schema-valid fixtures used to exercise the
SBOMParser, SignatureVerifier, and ProvenanceVerifier ports and their TCKs.

All fixtures are **synthesized**: they are hand-crafted to be structurally
valid against their respective specs while keeping the object count minimal.
Sources consulted:

- SPDX 2.3: https://spdx.github.io/spdx-spec/v2.3/ (SPDX 2.3.1)
- SPDX 3.0: https://spdx.github.io/spdx-spec/v3.0/ (draft)
- CycloneDX 1.5: https://cyclonedx.org/docs/1.5/json/
- CycloneDX 1.6: https://cyclonedx.org/docs/1.6/json/
- SLSA provenance: https://slsa.dev/provenance/v1
- OpenVEX 0.2.0: https://openvex.dev/docs/using-vex

## Fixture Inventory

| File | Format | Producer (synthetic) | Purpose |
|------|--------|----------------------|---------|
| `spdx23.json` | SPDX 2.3 | m4-supply-chain-test | VerificationCode target digest; 3 packages with purl; documentDescribes |
| `spdx30.json` | SPDX 3.0 | m4-supply-chain-test | Graph model (@graph); creationInfo; SPDX-SWIFTPackageElement |
| `cdx15.json` | CycloneDX 1.5 | m4-supply-chain-test | metadata.component with sha256; 2 components with purl+hashes |
| `cdx16.json` | CycloneDX 1.6 | m4-supply-chain-test | One component without purl (tests identity-derivation fallback); metadata.component |
| `intoto-attestation.json` | intoto-statement | m4-supply-chain-test | SLSA provenance v1; subject digest; builder.id; materials |
| `openvex.json` | OpenVEX 0.2.0 | m4-supply-chain-test | Two statements: not_affected + fixed; VEX id + timestamps |

## Purl Normalization Expectations

Fixtures are designed so that `purl` fields in SPDX/CycloneDX components are
run through `purl.Normalize` from `internal/supplychain/purl.go` before being
used as node IDs. The normalizer lowercases scheme+type and validates the
"pkg:" prefix; version is preserved as-is.

## Schema Validity

Fixtures are JSON-schema-valid per their respective specs but contain the
minimum fields required for the TCK assertions. They are not intended to
exercise every field of the respective schemas.
