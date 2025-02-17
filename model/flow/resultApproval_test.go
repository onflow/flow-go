package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestResultApprovalEncode(t *testing.T) {
	ra := unittest.ResultApprovalFixture()
	id := ra.ID()
	assert.NotEqual(t, flow.ZeroID, id)
}

// TestAttestationID_Malleability confirms that the Attestation struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestAttestationID_Malleability(t *testing.T) {
	unittest.RequireEntityNotMalleable(t, unittest.AttestationFixture())
}
