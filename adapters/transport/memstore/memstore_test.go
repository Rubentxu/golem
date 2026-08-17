package memstore

import (
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/tck"
)

// The in-memory transport must conform to the EventTransport TCK: it is
// the semantic baseline for the NATS JetStream adapter (ADR-046).
func TestTransportConformance(t *testing.T) {
	tck.RunEventTransportTCK(t, func() ports.EventTransport { return NewTransport() })
}
