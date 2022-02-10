package execution

import (
	"context"
	"math/rand"

	"github.com/stretchr/testify/require"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/proof"
	"github.com/onflow/flow-go/ledger/partial"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

type ChunkDataPacksSuite struct {
	Suite
}

func (gs *ChunkDataPacksSuite) TestVerificationNodesRequestChunkDataPacks() {
	unittest.SkipUnless(gs.Suite.T(), unittest.TEST_FLAKY, "flaky test")

	// wait for next height finalized (potentially first height), called blockA
	blockA := gs.BlockState.WaitForHighestFinalizedProgress(gs.T())
	gs.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// wait for execution receipt for blockA from execution node 1
	erExe1BlockA := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockA.Header.ID(), gs.exe1ID)
	finalStateErExec1BlockA, err := erExe1BlockA.ExecutionResult.FinalStateCommitment()
	require.NoError(gs.T(), err)
	gs.T().Logf("got erExe1BlockA with SC %x", finalStateErExec1BlockA)

	// assert there were no ChunkDataRequests from the verification node yet
	require.Equal(gs.T(), 0, gs.MsgState.LenFrom(gs.verID),
		"expected no ChunkDataRequest to be sent before a transaction existed")

	// send transaction
	err = gs.AccessClient().DeployContract(context.Background(), sdk.Identifier(gs.net.Root().ID()), common.CounterContract)
	require.NoError(gs.T(), err, "could not deploy counter")

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, _ := common.WaitUntilFinalizedStateCommitmentChanged(gs.T(), gs.BlockState, gs.ReceiptState)
	gs.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// wait for execution receipt for blockB from execution node 1
	erExe1BlockB := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockB.Header.ID(), gs.exe1ID)
	finalStateErExec1BlockB, err := erExe1BlockB.ExecutionResult.FinalStateCommitment()
	require.NoError(gs.T(), err)
	gs.T().Logf("got erExe1BlockB with SC %x", finalStateErExec1BlockB)

	// extract chunk ID from execution receipt
	// expecting the chunk itself plus the system chunk
	require.Len(gs.T(), erExe1BlockB.ExecutionResult.Chunks, 2)
	chunkID := erExe1BlockB.ExecutionResult.Chunks[0].ID()

	// TODO the following is extremely flaky, investigate why and re-activate.
	// wait for ChunkDataPack pushed from execution node
	// msg := gs.MsgState.WaitForMsgFrom(gs.T(), common.MsgIsChunkDataPackResponse, gs.exe1ID)
	// pack := msg.(*messages.ChunkDataResponse)
	// require.Equal(gs.T(), erExe1BlockB.ExecutionResult.Chunks[0].ID(), pack.ChunkDataPack.ChunkID
	// TODO clear messages

	// send a ChunkDataRequest from Ghost node
	err = gs.Ghost().Send(context.Background(), engine.PushReceipts,
		&messages.ChunkDataRequest{ChunkID: chunkID, Nonce: rand.Uint64()},
		[]flow.Identifier{gs.exe1ID}...)
	require.NoError(gs.T(), err)

	// wait for ChunkDataResponse
	msg2 := gs.MsgState.WaitForMsgFrom(gs.T(), common.MsgIsChunkDataPackResponse, gs.exe1ID, "chunk data response from execution node")
	pack2 := msg2.(*messages.ChunkDataResponse)
	require.Equal(gs.T(), chunkID, pack2.ChunkDataPack.ChunkID)
	require.Equal(gs.T(), erExe1BlockB.ExecutionResult.Chunks[0].StartState, pack2.ChunkDataPack.StartState)

	// verify state proofs
	batchProof, err := encoding.DecodeTrieBatchProof(pack2.ChunkDataPack.Proof)
	require.NoError(gs.T(), err)

	isValid := proof.VerifyTrieBatchProof(batchProof, ledger.State(erExe1BlockB.ExecutionResult.Chunks[0].StartState))
	require.NoError(gs.T(), err, "error verifying chunk trie proofs")
	require.True(gs.T(), isValid, "chunk trie proofs are not valid, but must be")

	_, err = partial.NewLedger(pack2.ChunkDataPack.Proof, ledger.State(pack2.ChunkDataPack.StartState), partial.DefaultPathFinderVersion)
	require.NoError(gs.T(), err, "error building PSMT")
}
