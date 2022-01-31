package verification

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/network/p2p/unicast"
)

// TestVerificationStreamNegotiationSuite enables gzip stream compression only between execution and verification nodes, while the
// rest of network runs on plain libp2p streams. It evaluates that network operates on its happy path concerning verification functionality.
func TestVerificationStreamNegotiationSuite(t *testing.T) {
	s := new(VerificationStreamNegotiationSuite)
	s.preferredUnicasts = string(unicast.GzipCompressionUnicast) // enables gzip stream compression between execution and verification node
	suite.Run(t, s)
}

type VerificationStreamNegotiationSuite struct {
	Suite
}

// TestVerificationNodeHappyPath verifies the integration of verification and execution nodes over the
// happy path of successfully issuing a result approval for the first chunk of the first block of the testnet.
// Note that gzip stream compression is enabled between verification and execution nodes.
func (suite *VerificationStreamNegotiationSuite) TestVerificationNodeHappyPath() {
	testVerificationNodeHappyPath(suite.T(), suite.exe1ID, suite.verID, suite.BlockState, suite.ReceiptState, suite.ApprovalState)
}
