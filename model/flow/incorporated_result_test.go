package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestIncorporatedResultID checks that the ID and Checksum of the Incorporated
// Result do not depend on the chunkApprovals placeholder.
func TestIncorporatedResultID(t *testing.T) {

	ir := flow.NewIncorporatedResult(
		unittest.IdentifierFixture(),
		unittest.ExecutionResultFixture(),
	)

	// Compute the ID and Checksum when the aggregated signatures are empty
	id1 := ir.ID()
	cs1 := ir.Checksum()

	// Add two signatures
	ir.AddSignature(0, unittest.IdentifierFixture(), unittest.SignatureFixture())
	ir.AddSignature(1, unittest.IdentifierFixture(), unittest.SignatureFixture())

	// Compute the ID and Checksum again
	id2 := ir.ID()
	cs2 := ir.Checksum()

	// They should not have changed
	assert.Equal(t, id1, id2)
	assert.Equal(t, cs1, cs2)
}
