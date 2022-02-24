package streamNegotiation

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/verification"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/stretchr/testify/suite"
)

// TestVerificationStreamNegotiationSuite enables gzip stream compression only between execution and verification nodes, while the
// rest of network runs on plain libp2p streams. It evaluates that network operates on its happy path concerning verification functionality.
func TestVerificationStreamNegotiationSuite(t *testing.T) {
	s := new(verification.VerificationStreamNegotiationSuite)
	s.PreferredUnicasts = string(unicast.GzipCompressionUnicast) // enables gzip stream compression between execution and verification node
	suite.Run(t, s)
}
