package ingest

import (
	"testing"
)

func TestVEXOpenVEXCommandIDDeterminism(t *testing.T) {
	payload := []byte(`{
		"doc_id": "vex-doc-1",
		"statements": [
			{
				"id": "stmt-1",
				"vuln_id": "CVE-2021-23337",
				"status": "not_affected",
				"product": {"identifier": "sha256:abc", "type": "artifact"},
				"provider": "openvex"
			}
		]
	}`)
	vex := VEXOpenVEX{}
	first, err := vex.Translate(payload)
	if err != nil {
		t.Fatalf("first translate: %v", err)
	}
	second, err := vex.Translate(payload)
	if err != nil {
		t.Fatalf("second translate: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("different number of commands: first=%d second=%d", len(first), len(second))
	}
	for i := range first {
		if first[i].CommandID != second[i].CommandID {
			t.Fatalf("command %d: first=%q second=%q", i, first[i].CommandID, second[i].CommandID)
		}
	}
}

func TestSBOMSPDXCommandIDDeterminism(t *testing.T) {
	payload := []byte(`{"external_id": "doc-1", "document": {"name": "test"}, "raw_b64": "ZG9j"}`)
	spdx := SBOMSPDX{}
	first, err := spdx.Translate(payload)
	if err != nil {
		t.Fatalf("first translate: %v", err)
	}
	second, err := spdx.Translate(payload)
	if err != nil {
		t.Fatalf("second translate: %v", err)
	}
	if first[0].CommandID != second[0].CommandID {
		t.Fatalf("CommandID not deterministic: first=%q second=%q", first[0].CommandID, second[0].CommandID)
	}
}

func TestAttestationInTotoCommandIDDeterminism(t *testing.T) {
	payload := []byte(`{"subject_digest": "sha256:abc123", "predicate_type": "slsa-provenance", "statement_b64": "e30=", "provider": "github"}`)
	att := AttestationInToto{}
	first, err := att.Translate(payload)
	if err != nil {
		t.Fatalf("first translate: %v", err)
	}
	second, err := att.Translate(payload)
	if err != nil {
		t.Fatalf("second translate: %v", err)
	}
	if first[0].CommandID != second[0].CommandID {
		t.Fatalf("CommandID not deterministic: first=%q second=%q", first[0].CommandID, second[0].CommandID)
	}
}
