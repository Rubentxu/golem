package ref

import (
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/tck"
)

func TestProvenanceVerifierTCK(t *testing.T) {
	tck.RunProvenanceVerifierTCK(t, func() ports.ProvenanceVerifier {
		return NewVerifier()
	})
}
