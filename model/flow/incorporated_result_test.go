package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestIncorporatedResultID checks that the ID and Checksum of the Incorporated
// Result do not depend on the aggregatedSignatures placeholder.
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

// Test that the same result incorporated (from a different receipt) in the
// same block produces the same IncorporatedResult (with same ID).
func TestIDCollision(t *testing.T) {
	incorporatedBlockID := unittest.IdentifierFixture()
	executionResult := unittest.ExecutionResultFixture()

	// Note:
	//  * Only the ExecutionResultBody is fully independent of the executor.
	//  * The (full) ExecutionResult contains the additional field ExecutionResult.Signatures,
	//    which holds the signature of the EN computing the result
	ir := flow.NewIncorporatedResult(incorporatedBlockID, executionResult)
	id1 := ir.ID()

	// make alternative signature
	altSig := unittest.SignaturesFixture(1)
	assert.NotEqual(t, ir.Result.Signatures, altSig) // check that new signature is in fact different (not that they are both empty, or constant dummy values)
	ir.Result.Signatures = altSig
	id2 := ir.ID()

	assert.Equal(t, id1, id2)
}
