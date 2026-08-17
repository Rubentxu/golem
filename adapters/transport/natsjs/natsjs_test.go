package natsjs

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/tck"
)

// The JetStream adapter must pass the same conformance kit as the
// in-memory baseline (ADR-046). Runs against a real server when
// GOLEM_TEST_NATS_URL is set (e.g. a local `nats-server -js` or a CI
// service container); skipped otherwise so `just check` stays hermetic.
//
// Each transport instance gets a unique stream and subject namespace so
// runs never overlap (NATS rejects subject overlaps between streams) and
// never inherit state; the stream is deleted on cleanup.
func TestTransportConformance(t *testing.T) {
	url := os.Getenv("GOLEM_TEST_NATS_URL")
	if url == "" {
		t.Skip("GOLEM_TEST_NATS_URL not set; JetStream TCK requires a live NATS server")
	}

	newTransport := func() ports.EventTransport {
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		tr, err := Connect(context.Background(), Config{
			URL:         url,
			Stream:      "GOLEM_TCK_" + suffix,
			Consumer:    "tck",
			SubjectRoot: "golemtck" + suffix,
		})
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
		t.Cleanup(func() {
			_ = tr.DeleteStream()
			_ = tr.Close()
		})
		return tr
	}
	tck.RunEventTransportTCK(t, newTransport)
}
