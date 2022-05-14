package passthrough

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
)

type PassThroughTestSuit struct {
	Suite
}

func TestPassThrough(t *testing.T) {
	suite.Run(t, new(PassThroughTestSuit))
}

// TestSealingAndVerificationPassThrough evaluates the health of Corruptible Conduit Framework (CCF) for BFT testing.
// It runs with two corrupted execution nodes and one corrupted verification node.
// The corrupted nodes are controlled by a dummy orchestrator that lets all incoming events passing through.
// The test deploys a transaction into the testnet hence causing an execution result with more than
// one chunk, assigns all chunks to the same single verification node in this testnet, and then verifies whether verification node
// generates a result approval for all chunks of that execution result.
// It also enables sealing based on result approvals and verifies whether the block of that specific multi-chunk execution result is sealed
// affected by the emitted result approvals.
// Finally, it evaluates whether critical sealing-and-verification-related events from corrupted nodes are passed through the orchestrator.
func (p *PassThroughTestSuit) TestSealingAndVerificationPassThrough() {
	receipts, approvals := common.SealingAndVerificationHappyPathTest(
		p.T(),
		p.BlockState,
		p.ReceiptState,
		p.ApprovalState,
		p.AccessClient(),
		p.exe1ID,
		p.exe2ID,
		p.verID,
		p.net.Root().ID())

	// identifier of chunks involved in the sealing and verification test.
	chunkIds := flow.GetIDs(receipts[0].ExecutionResult.Chunks)

	// as orchestrator controls the corrupted execution and verification nodes, it must see
	// the execution receipts, chunk data pack requests and responses, as well as result approvals emitted by these nodes.
	p.Orchestrator.mustSeenFlowProtocolEvent(p.T(), typeExecutionReceipt, flow.GetIDs(receipts)...)
	p.Orchestrator.mustSeenFlowProtocolEvent(p.T(), typeChunkDataRequest, chunkIds...)
	p.Orchestrator.mustSeenFlowProtocolEvent(p.T(), typeChunkDataResponse, chunkIds...)
	p.Orchestrator.mustSeenFlowProtocolEvent(p.T(), typeResultApproval, flow.GetIDs(approvals)...)
}
