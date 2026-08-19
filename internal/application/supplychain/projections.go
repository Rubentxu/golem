// Package supplychain hosts the application handlers and projections of the
// Supply Chain bounded context: SBOM, vulnerability, VEX, and attestation.
package supplychain

import (
	"github.com/Rubentxu/golem/internal/application/projection"
	"github.com/Rubentxu/golem/internal/ports"
	domainsupplychain "github.com/Rubentxu/golem/internal/supplychain"
)

// SupplychainProjection implements the projection.Projection interface for
// the supplychain bounded context. It handles SBOM, vulnerability, VEX, and
// attestation events.
type SupplychainProjection struct{}

// Domain returns "supplychain".
func (SupplychainProjection) Domain() string { return "supplychain" }

// EventTypes returns the event types claimed by this projection.
func (SupplychainProjection) EventTypes() []string {
	return []string{
		domainsupplychain.EventSBOMIngested,
		domainsupplychain.EventVulnerabilityReported,
		domainsupplychain.EventVEXStatementRecorded,
		domainsupplychain.EventAttestationIngested,
	}
}

// Handle processes supplychain events and returns the corresponding graph mutation.
func (SupplychainProjection) Handle(env ports.RawEvent) (ports.GraphMutation, bool, error) {
	switch env.EventType {
	case domainsupplychain.EventSBOMIngested:
		m, err := projection.ProjectSBOMIngested(env)
		return m, true, err
	case domainsupplychain.EventVulnerabilityReported:
		m, err := projection.ProjectVulnerabilityReported(env)
		return m, true, err
	case domainsupplychain.EventVEXStatementRecorded:
		m, err := projection.ProjectVEXStatement(env)
		return m, true, err
	case domainsupplychain.EventAttestationIngested:
		m, err := projection.ProjectAttestationIngested(env)
		return m, true, err
	}
	return ports.GraphMutation{}, false, nil
}

// NewProjection creates a new SupplychainProjection.
func NewProjection() projection.Projection {
	return SupplychainProjection{}
}
