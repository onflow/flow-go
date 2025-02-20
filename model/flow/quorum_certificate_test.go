package flow_test

import (
	"github.com/onflow/flow-go/utils/unittest"
	"testing"
)

// TestQuorumCertificateID_Malleability confirms that the QuorumCertificate struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestQuorumCertificateID_Malleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, unittest.QuorumCertificateFixture())
}
