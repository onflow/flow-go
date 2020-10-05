package execution_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/testutil"
	testmock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test execution node can sync execution state from another EN node.
//
// It creates 2 ENs, and 7 blocks:
// A <- B
//      ^- C (seal_A)
//         ^- D (seal_B, finalized)
//      		  ^- E <- F <- G (finalizes D, seals B)
//
// set the sync threshold as 2, meaning requires 2 sealed and unexecuted
// blocks to trigger state syncing
// set the block execution for EN1 to be fast
// set the block execution for EN2 to be slow
//
// EN1 will have A,B,C,D,E,F,G
// EN2 will receive A,B,C,D,E,F and verify no state syncing is triggered.
// Once EN2 has received G, state syncing will be triggered.
// verify that EN2 has the synced state up to B
// verify that EN2 has executed block up to F
func TestStateSyncFlow(t *testing.T) {
	// create two EN nodes,
	// EN1 is able to execute blocks fast,
	// EN2 is slow to execute any block, it has to rely on state syncing
	// to catch up.
	withNodes(t, func(hub *stub.Hub, EN1, EN2 *testmock.ExecutionNode) {
		log := unittest.Logger()
		log.Debug().Msgf("EN1's ID: %v", EN1.GenericNode.Me.NodeID())
		log.Debug().Msgf("EN2's ID: %v", EN2.GenericNode.Me.NodeID())

		EN2.ExecutionEngine.OnComputeBlock = func(ctx context.Context, block *entity.ExecutableBlock, view *delta.View) {
			if block.Block.Header.Height <= 2 {
				// A and B will take a long time to execute for EN2
				log.Info().Msgf("EN2 is about to compute block: %v, let it be slow...", block.ID())
				time.Sleep(time.Second * 200)
			}
		}

		genesis, err := EN1.State.AtHeight(0).Head()
		require.NoError(t, err, "could not get genesis")

		// create all the blocks needed for this tests:
		// A <- B <- C <- D <- E <- F <- G
		blockA := unittest.BlockWithParentAndSeal(genesis, nil)       // height: 1
		blockB := unittest.BlockWithParentAndSeal(blockA.Header, nil) // height: 2
		blockC := unittest.BlockWithParentAndSeal(blockB.Header, blockA.Header)
		blockD := unittest.BlockWithParentAndSeal(blockC.Header, blockB.Header)
		blockE := unittest.BlockWithParentAndSeal(blockD.Header, blockC.Header)
		blockF := unittest.BlockWithParentAndSeal(blockE.Header, nil)
		blockG := unittest.BlockWithParentAndSeal(blockF.Header, nil)

		logBlocks(log, blockA, blockB, blockC, blockD, blockE, blockF, blockG)

		waitTimeout := 5 * time.Second

		// EN1 receives all the blocks and we wait until EN1 has considered
		// blockB is sealed.
		sendBlockToEN(t, blockA, nil, EN1)    // sealed height 0
		sendBlockToEN(t, blockB, nil, EN1)    // trigger BlockProcessable(BlockA), sealed height 0
		sendBlockToEN(t, blockC, nil, EN1)    // trigger BlockProcessable(BlockB), sealed height 0
		sendBlockToEN(t, blockD, blockA, EN1) // trigger BlockProcessable(BlockC), sealed height 0
		sendBlockToEN(t, blockE, blockB, EN1) // trigger BlockProcessable(BlockD), sealed height 0
		sendBlockToEN(t, blockF, blockC, EN1) // trigger BlockProcessable(BlockE), sealed height 1

		// should not trigger state sync, because sealed height is 1, meaning only 1 block is sealed,
		// which is less than the threshold (2)
		waitUntilBlockIsExecuted(t, hub, blockE, EN1, waitTimeout)
		log.Info().Msgf("EN1 has executed blockE")

		waitUntilBlockSealed(t, hub, blockA, EN1, waitTimeout)
		log.Info().Msgf("EN1 has sealed blockA")

		sendBlockToEN(t, blockG, blockD, EN1) // trigger BlockProcessable(BlockF), sealed height 2,

		// should not trigger state sync, because all sealed blocks have been executed.
		waitUntilBlockIsExecuted(t, hub, blockF, EN1, waitTimeout)
		log.Info().Msgf("EN1 has executed blockG")

		waitUntilBlockSealed(t, hub, blockB, EN1, waitTimeout)
		log.Info().Msgf("EN1 has sealed blockB")

		// send blocks to EN2
		sendBlockToEN(t, blockA, nil, EN2)    // sealed height 0
		sendBlockToEN(t, blockB, nil, EN2)    // trigger BlockProcessable(BlockA), sealed height 0
		sendBlockToEN(t, blockC, nil, EN2)    // trigger BlockProcessable(BlockB), sealed height 0
		sendBlockToEN(t, blockD, blockA, EN2) // trigger BlockProcessable(BlockC), sealed height 0
		sendBlockToEN(t, blockE, blockB, EN2) // trigger BlockProcessable(BlockD), sealed height 0
		sendBlockToEN(t, blockF, blockC, EN2) // trigger BlockProcessable(BlockE), sealed height 1
		sendBlockToEN(t, blockG, blockD, EN2) // trigger BlockProcessable(BlockF), sealed height 2

		// verify that the state syncing is not triggered

		// wait for a short period of time for the statecommitment for G is ready from syncing state.
		// it will timeout if state syncing didn't happen, because the block execution is too long to
		// wait for.
		// block B will be executed, because it has been sealed, and the state delta is available
		// for it
		waitUntilBlockIsExecuted(t, hub, blockB, EN2, waitTimeout)
		log.Info().Msgf("EN2 has synced block B")

		waitUntilBlockIsExecuted(t, hub, blockF, EN2, waitTimeout)
	})
}

func withNodes(t *testing.T, f func(hub *stub.Hub, en1, en2 *testmock.ExecutionNode)) {
	hub := stub.NewNetworkHub()

	chainID := flow.Mainnet

	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exe1ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	exe2ID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	identities := unittest.CompleteIdentitySet(colID, conID, exe1ID, exe2ID, verID)

	syncThreshold := 2
	exeNode1 := testutil.ExecutionNode(t, hub, exe1ID, identities, syncThreshold, chainID)
	exeNode1.Ready()
	defer exeNode1.Done()
	exeNode2 := testutil.ExecutionNode(t, hub, exe2ID, identities, syncThreshold, chainID)
	exeNode2.Ready()
	defer exeNode2.Done()

	collectionNode := testutil.GenericNode(t, hub, colID, identities, chainID)
	defer collectionNode.Done()
	verificationNode := testutil.GenericNode(t, hub, verID, identities, chainID)
	defer verificationNode.Done()
	consensusNode := testutil.GenericNode(t, hub, conID, identities, chainID)
	defer consensusNode.Done()

	f(hub, &exeNode1, &exeNode2)
}

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func logBlocks(log zerolog.Logger, blocks ...*flow.Block) {
	for i, b := range blocks {
		// support up to 26 blocks
		name := string(alphabet[i])
		log.Debug().Msgf("creating blocks for testing, block %v's ID:%v, height:%v", name, b.ID(), b.Header.Height)
	}
}

func sendBlockToEN(t *testing.T, block *flow.Block, finalizes *flow.Block, en *testmock.ExecutionNode) {
	// simulating block finalization
	err := en.State.Mutate().HeaderExtend(block)
	require.NoError(t, err)

	if finalizes != nil {
		err = en.State.Mutate().Finalize(finalizes.ID())
		require.NoError(t, err)
	}

	// calling MarkValid will trigger the call to BlockProcessable
	err = en.State.Mutate().MarkValid(block.Header.ID())
	require.NoError(t, err)
}

func waitUntilBlockSealed(
	t *testing.T, hub *stub.Hub, block *flow.Block, en *testmock.ExecutionNode, timeout time.Duration) {
	blockID := block.ID()
	require.Eventually(t, func() bool {
		hub.DeliverAll()
		sealed, err := en.GenericNode.State.Sealed().Head()
		require.NoError(t, err)
		en.Log.Debug().Msgf("waiting for block %v (height: %v) to be sealed. current sealed: %v", blockID, block.Header.Height, sealed.Height)
		return sealed.Height >= block.Header.Height
	}, timeout, time.Millisecond*500,
		fmt.Sprintf("expect block %v to be sealed, but timeout", blockID))
}

func waitUntilBlockIsExecuted(
	t *testing.T, hub *stub.Hub, block *flow.Block, en *testmock.ExecutionNode, timeout time.Duration) {
	blockID := block.ID()
	require.Eventually(t, func() bool {
		hub.DeliverAll()
		_, err := en.ExecutionState.StateCommitmentByBlockID(context.Background(), blockID)
		return err == nil
	}, timeout, time.Millisecond*500,
		fmt.Sprintf("expect block %v to be executed, but timeout", block.ID()))
}
