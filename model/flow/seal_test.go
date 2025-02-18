package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestSealMalleability checks that Seal is not malleable: any change in its data
// should result in a different ID. This is REQUIRED FOR the STORAGE layer
// to correctly retrieve the block payload!
func TestSealMalleability(t *testing.T) {
	seal := unittest.Seal.Fixture()
	unittest.RequireEntityNotMalleable(t, seal)
}

// Test_SealID checks that two seals that only differ in their approval
// signatures have different IDs. This is REQUIRED FOR the STORAGE layer
// to correctly retrieve the block payload!
func Test_SealID(t *testing.T) {

	seal := unittest.Seal.Fixture()
	id := seal.ID()
	cs := seal.Checksum()

	// Change signatures of first chunk
	seal.AggregatedApprovalSigs[0] = unittest.Seal.AggregatedSignatureFixture()

	// The ID of the seal should now be different
	assert.NotEqual(t, id, seal.ID())
	assert.NotEqual(t, cs, seal.Checksum())
}
