package behavior

import (
	"context"
	"fmt"

	"github.com/Rubentxu/golem/internal/ports"
)

// EventBehaviorFound is emitted by the demo cve-gate behavior.
const EventBehaviorFound = "behavior.cve_gate.found.v1"

// EventBehaviorMitigation is emitted only by cve-gate v2 — the observable
// difference the shadow diff detects.
const EventBehaviorMitigation = "behavior.cve_gate.mitigation_suggested.v1"

// vulnerabilityFromLens scans the lens result for Vulnerability nodes.
// The CVE label comes from the attributes when present; otherwise the
// node ID carries it (the M4 projector stores vuln identity in the node
// ID "vuln-<cve>" and keeps severity/status/provider as attributes).
func vulnerabilityFromLens(in HandlerInput) ([]string, error) {
	var cves []string
	for _, n := range in.LensResult.Nodes {
		if n.Kind != "Vulnerability" {
			continue
		}
		if cve, ok := n.Attributes["cve"].(string); ok && cve != "" {
			cves = append(cves, cve)
			continue
		}
		cves = append(cves, "id:"+n.ID)
	}
	return cves, nil
}

func emitBehaviorEvent(in HandlerInput, eventType string, detail string) (ports.RawEvent, error) {
	id := in.IDs.NewID()
	payload := []byte(fmt.Sprintf(`{"detail":%q}`, detail))
	return ports.RawEvent{
		EventID:       id,
		TenantID:      in.Event.TenantID,
		StreamID:      "behavior:cve-gate",
		EventType:     eventType,
		SchemaVersion: 1,
		OccurredAt:    in.Clock.Now(),
		Actor:         ports.Actor{Type: "service", ID: "cve-gate"},
		Payload:       payload,
	}, nil
}

// CveGateV1 is the deterministic demo behavior: a vulnerability visible in
// the lens emits exactly one found event.
func CveGateV1(ctx context.Context, in HandlerInput) (HandlerOutput, error) {
	cves, err := vulnerabilityFromLens(in)
	if err != nil {
		return HandlerOutput{}, err
	}
	if len(cves) == 0 {
		return HandlerOutput{}, nil
	}
	ev, err := emitBehaviorEvent(in, EventBehaviorFound, fmt.Sprintf("cves=%v", cves))
	if err != nil {
		return HandlerOutput{}, err
	}
	return HandlerOutput{Events: []ports.RawEvent{ev}}, nil
}

// CveGateV2 additionally emits a mitigation suggestion — the deliberate
// v1/v2 divergence exercised by the shadow diff and the E2E demo.
func CveGateV2(ctx context.Context, in HandlerInput) (HandlerOutput, error) {
	cves, err := vulnerabilityFromLens(in)
	if err != nil {
		return HandlerOutput{}, err
	}
	if len(cves) == 0 {
		return HandlerOutput{}, nil
	}
	found, err := emitBehaviorEvent(in, EventBehaviorFound, fmt.Sprintf("cves=%v", cves))
	if err != nil {
		return HandlerOutput{}, err
	}
	mitigation, err := emitBehaviorEvent(in, EventBehaviorMitigation, "suggested: review VEX statements")
	if err != nil {
		return HandlerOutput{}, err
	}
	return HandlerOutput{
		Events:    []ports.RawEvent{found, mitigation},
		Proposals: []ProposalNote{{Title: "CVE mitigation", Body: "Review VEX statements for affected components"}},
	}, nil
}
