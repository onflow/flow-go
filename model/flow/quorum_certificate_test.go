package flow_test

import (
	"testing"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestQuorumCertificateID_Malleability confirms that the QuorumCertificate struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestQuorumCertificateID_Malleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, unittest.QuorumCertificateFixture())
}
