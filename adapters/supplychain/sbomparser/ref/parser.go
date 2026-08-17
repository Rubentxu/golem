// Package ref provides a stdlib-only reference parser for SBOM documents in SPDX
// 2.3, SPDX 3.0, CycloneDX 1.5, and CycloneDX 1.6 formats. It is used behind
// the SBOMParser port; format is detected from the document structure.
package ref

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/internal/supplychain"
)

// Parser implements ports.SBOMParser using stdlib JSON only.
type Parser struct{}

// NewParser builds a new reference SBOM parser.
func NewParser() *Parser { return &Parser{} }

// Parse interprets raw bytes as an SBOM and returns a normalized SBOMParsed.
func (p *Parser) Parse(ctx context.Context, raw []byte) (ports.SBOMParsed, error) {
	if len(raw) == 0 {
		return ports.SBOMParsed{}, fmt.Errorf("empty document")
	}

	// Detect format by structural cues.
	if isSPDX(raw) {
		return p.parseSPDX(ctx, raw)
	}
	if isCycloneDX(raw) {
		return p.parseCycloneDX(ctx, raw)
	}
	return ports.SBOMParsed{}, fmt.Errorf("unrecognized SBOM format")
}

func isSPDX(raw []byte) bool {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	_, ok := m["spdxVersion"]
	return ok
}

func isCycloneDX(raw []byte) bool {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	if bf, ok := m["bomFormat"].(string); ok && bf == "CycloneDX" {
		return true
	}
	return false
}

// parseSPDX delegates to the appropriate version based on spdxVersion value.
func (p *Parser) parseSPDX(ctx context.Context, raw []byte) (ports.SBOMParsed, error) {
	var header struct {
		SpdxVersion string `json:"spdxVersion"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return ports.SBOMParsed{}, fmt.Errorf("spdx header: %w", err)
	}
	if strings.HasPrefix(header.SpdxVersion, "SPDX-2.") {
		return p.parseSPDX23(ctx, raw)
	}
	if strings.HasPrefix(header.SpdxVersion, "SPDX-3.") {
		return p.parseSPDX30(ctx, raw)
	}
	return ports.SBOMParsed{}, fmt.Errorf("unsupported SPDX version: %s", header.SpdxVersion)
}

// SPDX 2.3 layout: packages[], verificationCode, documentDescribes[]
func (p *Parser) parseSPDX23(ctx context.Context, raw []byte) (ports.SBOMParsed, error) {
	var doc struct {
		SpdxVersion       string `json:"spdxVersion"`
		SPDXID            string `json:"SPDXID"`
		Name              string `json:"name"`
		DocumentNamespace string `json:"documentNamespace"`
		VerificationCode  *struct {
			Value         string   `json:"value"`
			ExcludedFiles []string `json:"excludedFileNames"`
		} `json:"verificationCode"`
		Packages []struct {
			SPDXID       string `json:"SPDXID"`
			Name         string `json:"name"`
			VersionInfo  string `json:"versionInfo"`
			ExternalRefs []struct {
				ReferenceCategory string `json:"referenceCategory"`
				ReferenceType     string `json:"referenceType"`
				ReferenceLocator  string `json:"referenceLocator"`
			} `json:"externalRefs"`
		} `json:"packages"`
		DocumentDescribes []string `json:"documentDescribes"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ports.SBOMParsed{}, fmt.Errorf("spdx23 parse: %w", err)
	}

	// Extract target digest: verificationCode.value is the canonical sha256
	// of the file the SBOM describes.
	artifactDigest := ""
	if doc.VerificationCode != nil && doc.VerificationCode.Value != "" {
		artifactDigest = "sha256:" + doc.VerificationCode.Value
	}

	comps := make([]ports.ParsedComponent, 0, len(doc.Packages))
	for _, pkg := range doc.Packages {
		purl := extractPurlFromExternalRefs(pkg.ExternalRefs)
		if purl == "" {
			// Fall back to synthetic purl: derive from name+version.
			v := pkg.VersionInfo
			if v == "" {
				v = "unknown"
			}
			purl = supplychain.Synthetic(pkg.Name, v)
			comps = append(comps, ports.ParsedComponent{
				Purl: purl, Name: pkg.Name, Version: v, Synthetic: true,
			})
		} else {
			norm, err := supplychain.Normalize(purl)
			if err != nil {
				// Fall back to synthetic.
				norm = supplychain.Synthetic(pkg.Name, pkg.VersionInfo)
				comps = append(comps, ports.ParsedComponent{
					Purl: norm, Name: pkg.Name, Version: pkg.VersionInfo, Synthetic: true,
				})
				continue
			}
			comps = append(comps, ports.ParsedComponent{
				Purl: norm, Name: pkg.Name, Version: pkg.VersionInfo, Synthetic: false,
			})
		}
	}

	return ports.SBOMParsed{
		DocID:          doc.SPDXID,
		DocName:        doc.Name,
		Format:         "spdx",
		SpecVersion:    doc.SpdxVersion,
		ArtifactDigest: artifactDigest,
		Components:     comps,
	}, nil
}

// SPDX 3.0 uses @graph structure with element types.
func (p *Parser) parseSPDX30(ctx context.Context, raw []byte) (ports.SBOMParsed, error) {
	var doc struct {
		SpdxVersion       string            `json:"spdxVersion"`
		SPDXID            string            `json:"SPDXID"`
		Name              string            `json:"name"`
		DocumentNamespace string            `json:"documentNamespace"`
		Graph             []json.RawMessage `json:"@graph"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ports.SBOMParsed{}, fmt.Errorf("spdx30 parse: %w", err)
	}

	comps := []ports.ParsedComponent{}
	var artifactDigest string

	for _, rawEl := range doc.Graph {
		var el struct {
			Type             string `json:"type"`
			SpdxID           string `json:"spdxId"`
			Name             string `json:"name"`
			DownloadLocation string `json:"downloadLocation"`
			ExternalRefs     []struct {
				ReferenceCategory string `json:"referenceCategory"`
				ReferenceType     string `json:"referenceType"`
				ReferenceLocator  string `json:"referenceLocator"`
			} `json:"externalRefs"`
			ContentInformation *struct {
				SHA256 string `json:"sha256"`
			} `json:"contentInformation"`
			ExternalIdentifiers []struct {
				Type       string `json:"type"`
				Identifier string `json:"identifier"`
			} `json:"externalIdentifiers"`
		}
		if err := json.Unmarshal(rawEl, &el); err != nil {
			continue
		}
		if el.Type != "Package" {
			continue
		}
		// Try to get artifact digest from contentInformation.sha256.
		if artifactDigest == "" && el.ContentInformation != nil && el.ContentInformation.SHA256 != "" {
			artifactDigest = "sha256:" + el.ContentInformation.SHA256
		}
		// Try external identifiers for sha256.
		if artifactDigest == "" {
			for _, id := range el.ExternalIdentifiers {
				if id.Type == "gitoid" && strings.HasPrefix(id.Identifier, "sha256:") {
					artifactDigest = id.Identifier
					break
				}
			}
		}
		// Extract purl.
		purl := extractPurlFromExternalRefs(el.ExternalRefs)
		version := extractVersionFromDownload(el.DownloadLocation)
		if purl == "" {
			v := version
			if v == "" {
				v = "unknown"
			}
			norm := supplychain.Synthetic(el.Name, v)
			comps = append(comps, ports.ParsedComponent{
				Purl: norm, Name: el.Name, Version: v, Synthetic: true,
			})
		} else {
			norm, err := supplychain.Normalize(purl)
			if err != nil {
				v := version
				if v == "" {
					v = "unknown"
				}
				norm = supplychain.Synthetic(el.Name, v)
				comps = append(comps, ports.ParsedComponent{
					Purl: norm, Name: el.Name, Version: v, Synthetic: true,
				})
				continue
			}
			comps = append(comps, ports.ParsedComponent{
				Purl: norm, Name: el.Name, Version: version, Synthetic: false,
			})
		}
	}

	return ports.SBOMParsed{
		DocID:          doc.SPDXID,
		DocName:        doc.Name,
		Format:         "spdx",
		SpecVersion:    doc.SpdxVersion,
		ArtifactDigest: artifactDigest,
		Components:     comps,
	}, nil
}

// CycloneDX 1.5/1.6 layout: metadata.component with hashes,
// components[] with purl and hashes.
func (p *Parser) parseCycloneDX(ctx context.Context, raw []byte) (ports.SBOMParsed, error) {
	var doc struct {
		BomFormat   string `json:"bomFormat"`
		SpecVersion string `json:"specVersion"`
		Metadata    *struct {
			Timestamp string `json:"timestamp"`
			Component *struct {
				Type      string `json:"type"`
				Name      string `json:"name"`
				Version   string `json:"version"`
				Group     string `json:"group"`
				Purl      string `json:"purl"`
				Publisher string `json:"publisher"`
				Hashes    []struct {
					Alg     string `json:"alg"`
					Content string `json:"content"`
				} `json:"hashes"`
			} `json:"component"`
		} `json:"metadata"`
		Components []struct {
			Type      string `json:"type"`
			Name      string `json:"name"`
			Version   string `json:"version"`
			Group     string `json:"group"`
			Purl      string `json:"purl"`
			Publisher string `json:"publisher"`
			Hashes    []struct {
				Alg     string `json:"alg"`
				Content string `json:"content"`
			} `json:"hashes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ports.SBOMParsed{}, fmt.Errorf("cyclonedx parse: %w", err)
	}

	// Extract artifact digest from metadata.component (primary artifact described).
	artifactDigest := ""
	if doc.Metadata != nil && doc.Metadata.Component != nil {
		for _, h := range doc.Metadata.Component.Hashes {
			if strings.EqualFold(h.Alg, "SHA-256") && h.Content != "" {
				artifactDigest = "sha256:" + h.Content
				break
			}
		}
	}

	comps := make([]ports.ParsedComponent, 0, len(doc.Components)+1)
	// Include metadata.component as a component if present.
	if doc.Metadata != nil && doc.Metadata.Component != nil {
		mc := doc.Metadata.Component
		purl := mc.Purl
		if purl == "" {
			v := mc.Version
			if v == "" {
				v = "unknown"
			}
			purl = supplychain.Synthetic(mc.Name, v)
			comps = append(comps, ports.ParsedComponent{
				Purl: purl, Name: mc.Name, Version: v, Synthetic: true,
			})
		} else {
			norm, err := supplychain.Normalize(purl)
			if err != nil {
				v := mc.Version
				if v == "" {
					v = "unknown"
				}
				norm = supplychain.Synthetic(mc.Name, v)
				comps = append(comps, ports.ParsedComponent{
					Purl: norm, Name: mc.Name, Version: v, Synthetic: true,
				})
			} else {
				comps = append(comps, ports.ParsedComponent{
					Purl: norm, Name: mc.Name, Version: mc.Version, Synthetic: false,
				})
			}
		}
	}

	for _, c := range doc.Components {
		purl := c.Purl
		if purl == "" {
			v := c.Version
			if v == "" {
				v = "unknown"
			}
			purl = supplychain.Synthetic(c.Name, v)
			comps = append(comps, ports.ParsedComponent{
				Purl: purl, Name: c.Name, Version: v, Synthetic: true,
			})
		} else {
			norm, err := supplychain.Normalize(purl)
			if err != nil {
				v := c.Version
				if v == "" {
					v = "unknown"
				}
				norm = supplychain.Synthetic(c.Name, v)
				comps = append(comps, ports.ParsedComponent{
					Purl: norm, Name: c.Name, Version: v, Synthetic: true,
				})
				continue
			}
			comps = append(comps, ports.ParsedComponent{
				Purl: norm, Name: c.Name, Version: c.Version, Synthetic: false,
			})
		}
	}

	return ports.SBOMParsed{
		DocName:        doc.Metadata.Component.Name,
		Format:         "cyclonedx",
		SpecVersion:    doc.SpecVersion,
		ArtifactDigest: artifactDigest,
		Components:     comps,
	}, nil
}

// extractPurlFromExternalRefs finds a purl external reference in SPDX packages.
func extractPurlFromExternalRefs(refs []struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}) string {
	for _, r := range refs {
		// PACKAGE-MANAGER category, purl type.
		if r.ReferenceCategory == "PACKAGE-MANAGER" && r.ReferenceType == "purl" {
			return r.ReferenceLocator
		}
	}
	return ""
}

// extractVersionFromDownload extracts version from download location URL.
func extractVersionFromDownload(dl string) string {
	// Simple heuristic: look for /v or /ref/tags/v or @ suffix.
	if strings.Contains(dl, "@") {
		if idx := strings.LastIndex(dl, "@"); idx >= 0 {
			return dl[idx+1:]
		}
	}
	return ""
}
