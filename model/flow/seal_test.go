package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/utils/unittest"
)

// Test_SealID checks that two seals that only differ in their approval
// signatures have different IDs. This is REQUIRED FOR the STORAGE layer
// to correctly retrieve the block payload!
func Test_SealID(t *testing.T) {

	seal := unittest.Seal.Fixture()
	id := seal.ID()
	cs := seal.Checksum()

	// Change signatures of first chunk
	seal.AggregatedApprovalSigs[0] = unittest.Seal.AggregatedSignatureFixture()

	// They should not have changed
	assert.NotEqual(t, id, seal.ID())
	assert.NotEqual(t, cs, seal.Checksum())
}
