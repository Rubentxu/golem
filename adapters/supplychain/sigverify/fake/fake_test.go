package fake

import (
	"testing"

	"github.com/Rubentxu/golem/internal/ports"
	"github.com/Rubentxu/golem/tck"
)

func TestSignatureVerifierTCK(t *testing.T) {
	tck.RunSignatureVerifierTCK(t, func() ports.SignatureVerifier {
		return NewVerifier()
	})
}
