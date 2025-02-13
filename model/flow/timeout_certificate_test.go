package flow_test

import (
	"testing"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestTimeoutCertificateID_Malleability confirms that the TimeoutCertificate struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestTimeoutCertificateID_Malleability(t *testing.T) {
	t.Run("TimeoutCertificate", func(t *testing.T) {
		unittest.RequireEntityNotMalleable(t, helper.MakeTC())
	})
}
