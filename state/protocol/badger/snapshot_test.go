// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/seed"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestHead(t *testing.T) {
	participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithBootstrapState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.State) {

		t.Run("works with block number", func(t *testing.T) {
			retrieved, err := state.AtHeight(head.Height).Head()
			require.NoError(t, err)
			require.Equal(t, head.ID(), retrieved.ID())
		})

		t.Run("works with block id", func(t *testing.T) {
			retrieved, err := state.AtBlockID(head.ID()).Head()
			require.NoError(t, err)
			require.Equal(t, head.ID(), retrieved.ID())
		})

		t.Run("works with finalized block", func(t *testing.T) {
			retrieved, err := state.Final().Head()
			require.NoError(t, err)
			require.Equal(t, head.ID(), retrieved.ID())
		})
	})
}

// TestSnapshot_Params tests retrieving global protocol state parameters from
// a protocol state snapshot.
func TestSnapshot_Params(t *testing.T) {
	participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	rootSnapshot := unittest.RootSnapshotFixture(participants)

	expectedChainID, err := rootSnapshot.Params().ChainID()
	require.NoError(t, err)
	expectedSporkID, err := rootSnapshot.Params().SporkID()
	require.NoError(t, err)
	expectedProtocolVersion, err := rootSnapshot.Params().ProtocolVersion()
	require.NoError(t, err)

	rootHeader, err := rootSnapshot.Head()
	require.NoError(t, err)

	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.MutableState) {
		// build some non-root blocks
		head := rootHeader
		const nBlocks = 10
		for i := 0; i < nBlocks; i++ {
			next := unittest.BlockWithParentFixture(head)
			err = state.Extend(context.Background(), next)
			require.NoError(t, err)
			err = state.Finalize(context.Background(), next.ID())
			require.NoError(t, err)
			head = next.Header
		}

		// test params from both root, final, and in between
		snapshots := []protocol.Snapshot{
			state.AtHeight(0),
			state.AtHeight(uint64(rand.Intn(nBlocks))),
			state.Final(),
		}
		for _, snapshot := range snapshots {
			t.Run("should be able to get chain ID from snapshot", func(t *testing.T) {
				chainID, err := snapshot.Params().ChainID()
				require.NoError(t, err)
				assert.Equal(t, expectedChainID, chainID)
			})
			t.Run("should be able to get spork ID from snapshot", func(t *testing.T) {
				sporkID, err := snapshot.Params().SporkID()
				require.NoError(t, err)
				assert.Equal(t, expectedSporkID, sporkID)
			})
			t.Run("should be able to get protocol version from snapshot", func(t *testing.T) {
				protocolVersion, err := snapshot.Params().ProtocolVersion()
				require.NoError(t, err)
				assert.Equal(t, expectedProtocolVersion, protocolVersion)
			})
		}
	})
}

// TestSnapshot_Descendants builds a sample chain with next structure:
// A (finalized) <- B <- C <- D <- E <- F
//               <- G <- H <- I <- J
// snapshot.Descendants has to return [B, C, D, E, F, G, H, I, J].
func TestSnapshot_Descendants(t *testing.T) {
	participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.MutableState) {
		var expectedBlocks []flow.Identifier
		for i := 5; i > 3; i-- {
			for _, block := range unittest.ChainFixtureFrom(i, head) {
				err := state.Extend(context.Background(), block)
				require.NoError(t, err)
				expectedBlocks = append(expectedBlocks, block.ID())
			}
		}

		pendingBlocks, err := state.AtBlockID(head.ID()).Descendants()
		require.NoError(t, err)
		require.ElementsMatch(t, expectedBlocks, pendingBlocks)
	})
}

// TestSnapshot_ValidDescendants builds a sample chain with next structure:
// A (finalized) <- B <- C <- D <- E <- F
//               <- G <- H <- I <- J
// snapshot.Descendants has to return [B, C, D, E, G, H, I]. [F, J] should be excluded because they aren't valid
func TestSnapshot_ValidDescendants(t *testing.T) {
	participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.MutableState) {
		var expectedBlocks []flow.Identifier
		for i := 5; i > 3; i-- {
			fork := unittest.ChainFixtureFrom(i, head)
			for blockIndex, block := range fork {
				err := state.Extend(context.Background(), block)
				require.NoError(t, err)
				// skip last block from fork
				if blockIndex < len(fork)-1 {
					err = state.MarkValid(block.ID())
					require.NoError(t, err)
					expectedBlocks = append(expectedBlocks, block.ID())
				}
			}
		}

		pendingBlocks, err := state.AtBlockID(head.ID()).ValidDescendants()
		require.NoError(t, err)
		require.ElementsMatch(t, expectedBlocks, pendingBlocks)
	})
}

func TestIdentities(t *testing.T) {
	identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithBootstrapState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.State) {

		t.Run("no filter", func(t *testing.T) {
			actual, err := state.Final().Identities(filter.Any)
			require.Nil(t, err)
			assert.ElementsMatch(t, identities, actual)
		})

		t.Run("single identity", func(t *testing.T) {
			expected := identities.Sample(1)[0]
			actual, err := state.Final().Identity(expected.NodeID)
			require.Nil(t, err)
			assert.Equal(t, expected, actual)
		})

		t.Run("filtered", func(t *testing.T) {
			filters := []flow.IdentityFilter{
				filter.HasRole(flow.RoleCollection),
				filter.HasNodeID(identities.SamplePct(0.1).NodeIDs()...),
				filter.HasWeight(true),
			}

			for _, filterfunc := range filters {
				expected := identities.Filter(filterfunc)
				actual, err := state.Final().Identities(filterfunc)
				require.Nil(t, err)
				assert.ElementsMatch(t, expected, actual)
			}
		})
	})
}

func TestClusters(t *testing.T) {
	nClusters := 3
	nCollectors := 7

	collectors := unittest.IdentityListFixture(nCollectors, unittest.WithRole(flow.RoleCollection))
	identities := append(unittest.IdentityListFixture(4, unittest.WithAllRolesExcept(flow.RoleCollection)), collectors...)

	root, result, seal := unittest.BootstrapFixture(identities)
	qc := unittest.QuorumCertificateFixture(unittest.QCWithBlockID(root.ID()))
	setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
	commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
	setup.Assignments = unittest.ClusterAssignment(uint(nClusters), collectors)
	clusterQCs := unittest.QuorumCertificatesFixtures(uint(nClusters))
	commit.ClusterQCs = flow.ClusterQCVoteDatasFromQCs(clusterQCs)
	seal.ResultID = result.ID()

	rootSnapshot, err := inmem.SnapshotFromBootstrapState(root, result, seal, qc)
	require.NoError(t, err)

	util.RunWithBootstrapState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.State) {
		expectedClusters, err := flow.NewClusterList(setup.Assignments, collectors)
		require.NoError(t, err)
		actualClusters, err := state.Final().Epochs().Current().Clustering()
		require.NoError(t, err)

		require.Equal(t, nClusters, len(expectedClusters))
		require.Equal(t, len(expectedClusters), len(actualClusters))

		for i := 0; i < nClusters; i++ {
			expected := expectedClusters[i]
			actual := actualClusters[i]

			assert.Equal(t, len(expected), len(actual))
			assert.Equal(t, expected.Fingerprint(), actual.Fingerprint())
		}
	})
}

// TestSealingSegment tests querying sealing segment with respect to various snapshots.
//
// For each valid sealing segment, we also test bootstrapping with this sealing segment.
func TestSealingSegment(t *testing.T) {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)

	t.Run("root sealing segment", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			expected, err := rootSnapshot.SealingSegment()
			require.NoError(t, err)
			actual, err := state.AtBlockID(head.ID()).SealingSegment()
			require.NoError(t, err)

			assert.Len(t, actual.ExecutionResults, 1)
			assert.Len(t, actual.Blocks, 1)
			unittest.AssertEqualBlocksLenAndOrder(t, expected.Blocks, actual.Blocks)

			assertSealingSegmentBlocksQueryableAfterBootstrap(t, state.AtBlockID(head.ID()))
		})
	})

	// test sealing segment for non-root segment with simple sealing structure
	// (no blocks in between reference block and latest sealed)
	// ROOT <- B1 <- B2(S1)
	// Expected sealing segment: [B1, B2]
	t.Run("non-root", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			// build a block to seal
			block1 := unittest.BlockWithParentFixture(head)
			buildBlock(t, state, block1)

			// build a block sealing block1
			block2 := unittest.BlockWithParentFixture(block1.Header)
			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
			block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt1), unittest.WithSeals(seal1)))
			buildBlock(t, state, block2)

			segment, err := state.AtBlockID(block2.ID()).SealingSegment()
			require.NoError(t, err)

			// build a valid child B3 to ensure we have a QC
			buildBlock(t, state, unittest.BlockWithParentFixture(block2.Header))

			// sealing segment should contain B1 and B2
			// B2 is reference of snapshot, B1 is latest sealed
			unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{block1, block2}, segment.Blocks)
			assert.Len(t, segment.ExecutionResults, 1)
			assertSealingSegmentBlocksQueryableAfterBootstrap(t, state.AtBlockID(block2.ID()))
		})
	})

	// test sealing segment for sealing segment with a large number of blocks
	// between the reference block and latest sealed
	// ROOT <- B1 <- .... <- BN(S1)
	// Expected sealing segment: [B1, ..., BN]
	t.Run("long sealing segment", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {

			// build a block to seal
			block1 := unittest.BlockWithParentFixture(head)
			buildBlock(t, state, block1)

			parent := block1
			// build a large chain of intermediary blocks
			for i := 0; i < 100; i++ {
				next := unittest.BlockWithParentFixture(parent.Header)
				buildBlock(t, state, next)
				parent = next
			}

			// build the block sealing block 1
			blockN := unittest.BlockWithParentFixture(parent.Header)
			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
			blockN.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt1), unittest.WithSeals(seal1)))
			buildBlock(t, state, blockN)

			// build a valid child B3 to ensure we have a QC
			buildBlock(t, state, unittest.BlockWithParentFixture(blockN.Header))

			segment, err := state.AtBlockID(blockN.ID()).SealingSegment()
			require.NoError(t, err)

			assert.Len(t, segment.ExecutionResults, 1)
			// sealing segment should cover range [B1, BN]
			assert.Len(t, segment.Blocks, 102)
			// first and last blocks should be B1, BN
			assert.Equal(t, block1.ID(), segment.Blocks[0].ID())
			assert.Equal(t, blockN.ID(), segment.Blocks[101].ID())
			assertSealingSegmentBlocksQueryableAfterBootstrap(t, state.AtBlockID(blockN.ID()))
		})
	})

	// test sealing segment where the segment blocks contain seals for
	// ancestor blocks prior to the sealing segment
	// ROOT <- B1 <- B2(R1) <- B3 <- B4(R2, S1) <- B5 <- B6(S2)
	// Expected sealing segment: [B2, B3, B4]
	t.Run("overlapping sealing segment", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {

			block1 := unittest.BlockWithParentFixture(head)
			buildBlock(t, state, block1)
			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)

			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt1)))
			buildBlock(t, state, block2)

			receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)

			block3 := unittest.BlockWithParentFixture(block2.Header)
			buildBlock(t, state, block3)

			block4 := unittest.BlockWithParentFixture(block3.Header)
			block4.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt2), unittest.WithSeals(seal1)))
			buildBlock(t, state, block4)

			block5 := unittest.BlockWithParentFixture(block4.Header)
			buildBlock(t, state, block5)

			block6 := unittest.BlockWithParentFixture(block5.Header)
			block6.SetPayload(unittest.PayloadFixture(unittest.WithSeals(seal2)))
			buildBlock(t, state, block6)

			segment, err := state.AtBlockID(block6.ID()).SealingSegment()
			require.NoError(t, err)

			// build a valid child to ensure we have a QC
			buildBlock(t, state, unittest.BlockWithParentFixture(block6.Header))

			// sealing segment should be [B2, B3, B4, B5, B6]
			require.Len(t, segment.Blocks, 5)
			unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{block2, block3, block4, block5, block6}, segment.Blocks)
			require.Len(t, segment.ExecutionResults, 1)

			assertSealingSegmentBlocksQueryableAfterBootstrap(t, state.AtBlockID(block6.ID()))
		})
	})

	// test sealing segment where you have a chain that is 5 blocks long and the block 5 has a seal for block 2
	// block 2 also contains a receipt but no result.
	// ROOT -> B1(Result_A, Receipt_A_1) -> B2(Result_B, Receipt_B, Receipt_A_2) -> B3(Receipt_C, Result_C) -> B4 -> B5(Seal_C)
	// the segment for B5 should be `[B2,B3,B4,B5] + [Result_A]`
	t.Run("sealing segment with 4 blocks and 1 execution result decoupled", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			// simulate scenario where execution result is missing from block payload
			// SealingSegment() should get result from results db and store it on ExecutionReceipts
			// field on SealingSegment
			resultA := unittest.ExecutionResultFixture()
			receiptA1 := unittest.ExecutionReceiptFixture(unittest.WithResult(resultA))
			receiptA2 := unittest.ExecutionReceiptFixture(unittest.WithResult(resultA))

			// receipt b also contains result b
			receiptB := unittest.ExecutionReceiptFixture()

			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receiptA1)))

			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receiptB), unittest.WithReceiptsAndNoResults(receiptA2)))
			receiptC, sealC := unittest.ReceiptAndSealForBlock(block2)

			block3 := unittest.BlockWithParentFixture(block2.Header)
			block3.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receiptC)))

			block4 := unittest.BlockWithParentFixture(block3.Header)

			block5 := unittest.BlockWithParentFixture(block4.Header)
			block5.SetPayload(unittest.PayloadFixture(unittest.WithSeals(sealC)))

			buildBlock(t, state, block1)
			buildBlock(t, state, block2)
			buildBlock(t, state, block3)
			buildBlock(t, state, block4)
			buildBlock(t, state, block5)

			segment, err := state.AtBlockID(block5.ID()).SealingSegment()
			require.NoError(t, err)

			// build a valid child to ensure we have a QC
			buildBlock(t, state, unittest.BlockWithParentFixture(block5.Header))

			require.Len(t, segment.Blocks, 4)
			unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{block2, block3, block4, block5}, segment.Blocks)
			require.Contains(t, segment.ExecutionResults, resultA)
			require.Len(t, segment.ExecutionResults, 2)

			assertSealingSegmentBlocksQueryableAfterBootstrap(t, state.AtBlockID(block5.ID()))
		})
	})

	// test sealing segment where you have a chain that is 5 blocks long and the block 5 has a seal for block 2.
	// even though block2 & block3 both reference ResultA it should be added to the segment execution results list once.
	// block3 also references ResultB, so it should exist in the segment execution results as well.
	// root -> B1[Result_A, Receipt_A_1] -> B2[Result_B, Receipt_B, Receipt_A_2] -> B3[Receipt_B_2, Receipt_for_seal, Receipt_A_3] -> B4 -> B5 (Seal_B2)
	t.Run("sealing segment with 4 blocks and 2 execution result decoupled", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			// simulate scenario where execution result is missing from block payload
			// SealingSegment() should get result from results db and store it on ExecutionReceipts
			// field on SealingSegment
			resultA := unittest.ExecutionResultFixture()

			// 3 execution receipts for Result_A
			receiptA1 := unittest.ExecutionReceiptFixture(unittest.WithResult(resultA))
			receiptA2 := unittest.ExecutionReceiptFixture(unittest.WithResult(resultA))
			receiptA3 := unittest.ExecutionReceiptFixture(unittest.WithResult(resultA))

			// receipt b also contains result b
			receiptB := unittest.ExecutionReceiptFixture()
			// get second receipt for Result_B, now we have 2 receipts for a single execution result
			receiptB2 := unittest.ExecutionReceiptFixture(unittest.WithResult(&receiptB.ExecutionResult))

			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receiptA1)))

			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receiptB), unittest.WithReceiptsAndNoResults(receiptA2)))

			receiptForSeal, seal := unittest.ReceiptAndSealForBlock(block2)

			block3 := unittest.BlockWithParentFixture(block2.Header)
			block3.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receiptForSeal), unittest.WithReceiptsAndNoResults(receiptB2, receiptA3)))

			block4 := unittest.BlockWithParentFixture(block3.Header)

			block5 := unittest.BlockWithParentFixture(block4.Header)
			block5.SetPayload(unittest.PayloadFixture(unittest.WithSeals(seal)))

			buildBlock(t, state, block1)
			buildBlock(t, state, block2)
			buildBlock(t, state, block3)
			buildBlock(t, state, block4)
			buildBlock(t, state, block5)

			segment, err := state.AtBlockID(block5.ID()).SealingSegment()
			require.NoError(t, err)

			// build a valid child to ensure we have a QC
			buildBlock(t, state, unittest.BlockWithParentFixture(block5.Header))

			require.Len(t, segment.Blocks, 4)
			unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{block2, block3, block4, block5}, segment.Blocks)
			require.Contains(t, segment.ExecutionResults, resultA)
			// ResultA should only be added once even though it is referenced in 2 different blocks
			require.Len(t, segment.ExecutionResults, 2)

			assertSealingSegmentBlocksQueryableAfterBootstrap(t, state.AtBlockID(block5.ID()))
		})
	})

	// Test the case where the reference block of the snapshot contains no seal.
	// We should consider the latest seal in a prior block.
	// ROOT <- B1 <- B2(R1) <- B3 <- B4(S1) <- B5
	// Expected sealing segment: [B1, B2, B3, B4, B5]
	t.Run("sealing segment where highest block in segment does not seal lowest", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			// build a block to seal
			block1 := unittest.BlockWithParentFixture(head)
			buildBlock(t, state, block1)

			// build a block sealing block1
			block2 := unittest.BlockWithParentFixture(block1.Header)
			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
			block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt1)))
			buildBlock(t, state, block2)

			block3 := unittest.BlockWithParentFixture(block2.Header)
			buildBlock(t, state, block3)

			block4 := unittest.BlockWithParentFixture(block3.Header)
			block4.SetPayload(unittest.PayloadFixture(unittest.WithSeals(seal1)))
			buildBlock(t, state, block4)

			block5 := unittest.BlockWithParentFixture(block4.Header)
			buildBlock(t, state, block5)

			snapshot := state.AtBlockID(block5.ID())

			// build a valid child to ensure we have a QC
			buildBlock(t, state, unittest.BlockWithParentFixture(block5.Header))

			segment, err := snapshot.SealingSegment()
			require.NoError(t, err)
			// sealing segment should contain B1 and B5
			// B5 is reference of snapshot, B1 is latest sealed
			unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{block1, block2, block3, block4, block5}, segment.Blocks)
			assert.Len(t, segment.ExecutionResults, 1)

			assertSealingSegmentBlocksQueryableAfterBootstrap(t, snapshot)
		})
	})
}

func TestLatestSealedResult(t *testing.T) {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)

	t.Run("root snapshot", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			gotResult, gotSeal, err := state.Final().SealedResult()
			require.NoError(t, err)
			expectedResult, expectedSeal, err := rootSnapshot.SealedResult()
			require.NoError(t, err)

			assert.Equal(t, expectedResult, gotResult)
			assert.Equal(t, expectedSeal, gotSeal)
		})
	})

	t.Run("non-root snapshot", func(t *testing.T) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			block1 := unittest.BlockWithParentFixture(head)
			err = state.Extend(context.Background(), block1)
			require.NoError(t, err)

			block2 := unittest.BlockWithParentFixture(block1.Header)
			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
			block2.SetPayload(unittest.PayloadFixture(unittest.WithSeals(seal1), unittest.WithReceipts(receipt1)))
			err = state.Extend(context.Background(), block2)
			require.NoError(t, err)

			// B1 <- B2(R1,S1)
			// querying B2 should return result R1, seal S1
			t.Run("reference block contains seal", func(t *testing.T) {
				gotResult, gotSeal, err := state.AtBlockID(block2.ID()).SealedResult()
				require.NoError(t, err)
				assert.Equal(t, block2.Payload.Results[0], gotResult)
				assert.Equal(t, block2.Payload.Seals[0], gotSeal)
			})

			block3 := unittest.BlockWithParentFixture(block2.Header)
			err = state.Extend(context.Background(), block3)
			require.NoError(t, err)

			// B1 <- B2(R1,S1) <- B3
			// querying B3 should still return (R1,S1) even though they are in parent block
			t.Run("reference block contains no seal", func(t *testing.T) {
				gotResult, gotSeal, err := state.AtBlockID(block2.ID()).SealedResult()
				require.NoError(t, err)
				assert.Equal(t, &receipt1.ExecutionResult, gotResult)
				assert.Equal(t, seal1, gotSeal)
			})

			// B1 <- B2(R1,S1) <- B3 <- B4(R2,S2,R3,S3)
			// There are two seals in B4 - should return latest by height (S3,R3)
			t.Run("reference block contains multiple seals", func(t *testing.T) {
				receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)
				receipt3, seal3 := unittest.ReceiptAndSealForBlock(block3)
				block4 := unittest.BlockWithParentFixture(block3.Header)
				block4.SetPayload(unittest.PayloadFixture(
					unittest.WithReceipts(receipt2, receipt3),
					unittest.WithSeals(seal2, seal3),
				))
				err = state.Extend(context.Background(), block4)
				require.NoError(t, err)

				gotResult, gotSeal, err := state.AtBlockID(block4.ID()).SealedResult()
				require.NoError(t, err)
				assert.Equal(t, &receipt3.ExecutionResult, gotResult)
				assert.Equal(t, seal3, gotSeal)
			})
		})
	})
}

// test retrieving quorum certificate and seed
func TestQuorumCertificate(t *testing.T) {
	identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)

	// should not be able to get QC or random beacon seed from a block with no children
	t.Run("no children", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {

			// create a block to query
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(flow.EmptyPayload())
			err := state.Extend(context.Background(), block1)
			require.Nil(t, err)

			_, err = state.AtBlockID(block1.ID()).QuorumCertificate()
			assert.Error(t, err)

			_, err = state.AtBlockID(block1.ID()).RandomSource()
			assert.Error(t, err)
		})
	})

	// should not be able to get random beacon seed from a block with only invalid
	// or unvalidated children
	t.Run("un-validated child", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {

			// create a block to query
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(flow.EmptyPayload())
			err := state.Extend(context.Background(), block1)
			require.Nil(t, err)

			// add child
			unvalidatedChild := unittest.BlockWithParentFixture(head)
			unvalidatedChild.SetPayload(flow.EmptyPayload())
			err = state.Extend(context.Background(), unvalidatedChild)
			assert.Nil(t, err)

			_, err = state.AtBlockID(block1.ID()).QuorumCertificate()
			assert.Error(t, err)

			_, err = state.AtBlockID(block1.ID()).RandomSource()
			assert.Error(t, err)
		})
	})

	// should be able to get QC and random beacon seed from root block
	t.Run("root block", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			// since we bootstrap with a root snapshot, this will be the root block
			_, err := state.AtBlockID(head.ID()).QuorumCertificate()
			assert.NoError(t, err)
			randomSeed, err := state.AtBlockID(head.ID()).RandomSource()
			assert.NoError(t, err)
			assert.Equal(t, len(randomSeed), seed.RandomSourceLength)
		})
	})

	// should be able to get QC and random beacon seed from a block with a valid child
	t.Run("valid child", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {

			// add a block so we aren't testing against root
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(flow.EmptyPayload())
			err := state.Extend(context.Background(), block1)
			require.Nil(t, err)
			err = state.MarkValid(block1.ID())
			require.Nil(t, err)

			// add a valid child to block1
			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(flow.EmptyPayload())
			err = state.Extend(context.Background(), block2)
			require.Nil(t, err)
			err = state.MarkValid(block2.ID())
			require.Nil(t, err)

			// should be able to get QC/seed
			qc, err := state.AtBlockID(block1.ID()).QuorumCertificate()
			assert.Nil(t, err)
			// should have signatures from valid child (block 2)
			assert.Equal(t, block2.Header.ParentVoterIDs, qc.SignerIDs)
			assert.Equal(t, block2.Header.ParentVoterSigData, qc.SigData)
			// should have view matching block1 view
			assert.Equal(t, block1.Header.View, qc.View)

			_, err = state.AtBlockID(block1.ID()).RandomSource()
			require.Nil(t, err)
		})
	})
}

// test that we can query current/next/previous epochs from a snapshot
func TestSnapshot_EpochQuery(t *testing.T) {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	result, _, err := rootSnapshot.SealedResult()
	require.NoError(t, err)

	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.MutableState) {
		epoch1Counter := result.ServiceEvents[0].Event.(*flow.EpochSetup).Counter
		epoch2Counter := epoch1Counter + 1

		epochBuilder := unittest.NewEpochBuilder(t, state)
		// build epoch 1 (prepare epoch 2)
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()
		// build epoch 2 (prepare epoch 3)
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(t, ok)
		epoch2, ok := epochBuilder.EpochHeights(2)
		require.True(t, ok)

		// we should be able to query the current epoch from any block
		t.Run("Current", func(t *testing.T) {
			t.Run("epoch 1", func(t *testing.T) {
				for _, height := range epoch1.Range() {
					counter, err := state.AtHeight(height).Epochs().Current().Counter()
					require.Nil(t, err)
					assert.Equal(t, epoch1Counter, counter)
				}
			})

			t.Run("epoch 2", func(t *testing.T) {
				for _, height := range epoch2.Range() {
					counter, err := state.AtHeight(height).Epochs().Current().Counter()
					require.Nil(t, err)
					assert.Equal(t, epoch2Counter, counter)
				}
			})
		})

		// we should be unable to query next epoch before it is defined by EpochSetup
		// event, afterward we should be able to query next epoch
		t.Run("Next", func(t *testing.T) {
			t.Run("epoch 1: before next epoch available", func(t *testing.T) {
				for _, height := range epoch1.StakingRange() {
					_, err := state.AtHeight(height).Epochs().Next().Counter()
					assert.Error(t, err)
					assert.True(t, errors.Is(err, protocol.ErrNextEpochNotSetup))
				}
			})

			t.Run("epoch 2: after next epoch available", func(t *testing.T) {
				for _, height := range append(epoch1.SetupRange(), epoch1.CommittedRange()...) {
					counter, err := state.AtHeight(height).Epochs().Next().Counter()
					require.Nil(t, err)
					assert.Equal(t, epoch2Counter, counter)
				}
			})
		})

		// we should get a sentinel error when querying previous epoch from the
		// first epoch after the root block, otherwise we should always be able
		// to query previous epoch
		t.Run("Previous", func(t *testing.T) {
			t.Run("epoch 1", func(t *testing.T) {
				for _, height := range epoch1.Range() {
					_, err := state.AtHeight(height).Epochs().Previous().Counter()
					assert.Error(t, err)
					assert.True(t, errors.Is(err, protocol.ErrNoPreviousEpoch))
				}
			})

			t.Run("epoch 2", func(t *testing.T) {
				for _, height := range epoch2.Range() {
					counter, err := state.AtHeight(height).Epochs().Previous().Counter()
					require.Nil(t, err)
					assert.Equal(t, epoch1Counter, counter)
				}
			})
		})
	})
}

// test that querying the first view of an epoch returns the appropriate value
func TestSnapshot_EpochFirstView(t *testing.T) {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	result, _, err := rootSnapshot.SealedResult()
	require.NoError(t, err)

	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.MutableState) {

		epochBuilder := unittest.NewEpochBuilder(t, state)
		// build epoch 1 (prepare epoch 2)
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()
		// build epoch 2 (prepare epoch 3)
		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(t, ok)
		epoch2, ok := epochBuilder.EpochHeights(2)
		require.True(t, ok)

		// figure out the expected first views of the epochs
		epoch1FirstView := head.View
		epoch2FirstView := result.ServiceEvents[0].Event.(*flow.EpochSetup).FinalView + 1

		// check first view for snapshots within epoch 1, with respect to a
		// snapshot in either epoch 1 or epoch 2 (testing Current and Previous)
		t.Run("epoch 1", func(t *testing.T) {

			// test w.r.t. epoch 1 snapshot
			t.Run("Current", func(t *testing.T) {
				for _, height := range epoch1.Range() {
					actualFirstView, err := state.AtHeight(height).Epochs().Current().FirstView()
					require.Nil(t, err)
					assert.Equal(t, epoch1FirstView, actualFirstView)
				}
			})

			// test w.r.t. epoch 2 snapshot
			t.Run("Previous", func(t *testing.T) {
				for _, height := range epoch2.Range() {
					actualFirstView, err := state.AtHeight(height).Epochs().Previous().FirstView()
					require.Nil(t, err)
					assert.Equal(t, epoch1FirstView, actualFirstView)
				}
			})
		})

		// check first view for snapshots within epoch 2, with respect to a
		// snapshot in either epoch 1 or epoch 2 (testing Next and Current)
		t.Run("epoch 2", func(t *testing.T) {

			// test w.r.t. epoch 1 snapshot
			t.Run("Next", func(t *testing.T) {
				for _, height := range append(epoch1.SetupRange(), epoch1.CommittedRange()...) {
					actualFirstView, err := state.AtHeight(height).Epochs().Next().FirstView()
					require.Nil(t, err)
					assert.Equal(t, epoch2FirstView, actualFirstView)
				}
			})

			// test w.r.t. epoch 2 snapshot
			t.Run("Current", func(t *testing.T) {
				for _, height := range epoch2.Range() {
					actualFirstView, err := state.AtHeight(height).Epochs().Current().FirstView()
					require.Nil(t, err)
					assert.Equal(t, epoch2FirstView, actualFirstView)
				}
			})
		})
	})
}

// Test querying identities in different epoch phases. During staking phase we
// should see identities from last epoch and current epoch. After staking phase
// we should see identities from current epoch and next epoch. Identities from
// a non-current epoch should have weight 0. Identities that exist in consecutive
// epochs should be de-duplicated.
func TestSnapshot_CrossEpochIdentities(t *testing.T) {

	// start with 20 identities in epoch 1
	epoch1Identities := unittest.IdentityListFixture(20, unittest.WithAllRoles())
	// 1 identity added at epoch 2 that was not present in epoch 1
	addedAtEpoch2 := unittest.IdentityFixture()
	// 1 identity removed in epoch 2 that was present in epoch 1
	removedAtEpoch2 := epoch1Identities.Sample(1)[0]
	// epoch 2 has partial overlap with epoch 1
	epoch2Identities := append(
		epoch1Identities.Filter(filter.Not(filter.HasNodeID(removedAtEpoch2.NodeID))),
		addedAtEpoch2)
	// epoch 3 has no overlap with epoch 2
	epoch3Identities := unittest.IdentityListFixture(10, unittest.WithAllRoles())

	rootSnapshot := unittest.RootSnapshotFixture(epoch1Identities)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.MutableState) {

		epochBuilder := unittest.NewEpochBuilder(t, state)
		// build epoch 1 (prepare epoch 2)
		epochBuilder.
			UsingSetupOpts(unittest.WithParticipants(epoch2Identities)).
			BuildEpoch().
			CompleteEpoch()
		// build epoch 2 (prepare epoch 3)
		epochBuilder.
			UsingSetupOpts(unittest.WithParticipants(epoch3Identities)).
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(t, ok)
		epoch2, ok := epochBuilder.EpochHeights(2)
		require.True(t, ok)

		t.Run("should be able to query at root block", func(t *testing.T) {
			root, err := state.Params().Root()
			require.NoError(t, err)
			snapshot := state.AtHeight(root.Height)
			identities, err := snapshot.Identities(filter.Any)
			require.Nil(t, err)

			// should have the right number of identities
			assert.Equal(t, len(epoch1Identities), len(identities))
			// should have all epoch 1 identities
			assert.ElementsMatch(t, epoch1Identities, identities)
		})

		t.Run("should include next epoch after staking phase", func(t *testing.T) {

			// get a snapshot from setup phase and commit phase of epoch 1
			snapshots := []protocol.Snapshot{state.AtHeight(epoch1.Setup), state.AtHeight(epoch1.Committed)}

			for _, snapshot := range snapshots {
				phase, err := snapshot.Phase()
				require.Nil(t, err)

				t.Run("phase: "+phase.String(), func(t *testing.T) {
					identities, err := snapshot.Identities(filter.Any)
					require.Nil(t, err)

					// should have the right number of identities
					assert.Equal(t, len(epoch1Identities)+1, len(identities))
					// all current epoch identities should match configuration from EpochSetup event
					assert.ElementsMatch(t, epoch1Identities, identities.Filter(epoch1Identities.Selector()))

					// should contain single next epoch identity with 0 weight
					nextEpochIdentity := identities.Filter(filter.HasNodeID(addedAtEpoch2.NodeID))[0]
					assert.Equal(t, uint64(0), nextEpochIdentity.Weight) // should have 0 weight
					nextEpochIdentity.Weight = addedAtEpoch2.Weight
					assert.Equal(t, addedAtEpoch2, nextEpochIdentity) // should be equal besides weight
				})
			}
		})

		t.Run("should include previous epoch in staking phase", func(t *testing.T) {

			// get a snapshot from staking phase of epoch 2
			snapshot := state.AtHeight(epoch2.Staking)
			identities, err := snapshot.Identities(filter.Any)
			require.Nil(t, err)

			// should have the right number of identities
			assert.Equal(t, len(epoch2Identities)+1, len(identities))
			// all current epoch identities should match configuration from EpochSetup event
			assert.ElementsMatch(t, epoch2Identities, identities.Filter(epoch2Identities.Selector()))

			// should contain single previous epoch identity with 0 weight
			lastEpochIdentity := identities.Filter(filter.HasNodeID(removedAtEpoch2.NodeID))[0]
			assert.Equal(t, uint64(0), lastEpochIdentity.Weight) // should have 0 weight
			lastEpochIdentity.Weight = removedAtEpoch2.Weight    // overwrite weight
			assert.Equal(t, removedAtEpoch2, lastEpochIdentity)  // should be equal besides weight
		})

		t.Run("should not include previous epoch after staking phase", func(t *testing.T) {

			// get a snapshot from setup phase and commit phase of epoch 2
			snapshots := []protocol.Snapshot{state.AtHeight(epoch2.Setup), state.AtHeight(epoch2.Committed)}

			for _, snapshot := range snapshots {
				phase, err := snapshot.Phase()
				require.Nil(t, err)

				t.Run("phase: "+phase.String(), func(t *testing.T) {
					identities, err := snapshot.Identities(filter.Any)
					require.Nil(t, err)

					// should have the right number of identities
					assert.Equal(t, len(epoch2Identities)+len(epoch3Identities), len(identities))
					// all current epoch identities should match configuration from EpochSetup event
					assert.ElementsMatch(t, epoch2Identities, identities.Filter(epoch2Identities.Selector()))

					// should contain next epoch identities with 0 weight
					for _, expected := range epoch3Identities {
						actual, exists := identities.ByNodeID(expected.NodeID)
						require.True(t, exists)
						assert.Equal(t, uint64(0), actual.Weight) // should have 0 weight
						actual.Weight = expected.Weight           // overwrite weight
						assert.Equal(t, expected, actual)         // should be equal besides weight
					}
				})
			}
		})
	})
}

// test that we can retrieve identities after a spork where the parent ID of the
// root block is non-nil
func TestSnapshot_PostSporkIdentities(t *testing.T) {
	expected := unittest.CompleteIdentitySet()
	root, result, seal := unittest.BootstrapFixture(expected, func(block *flow.Block) {
		block.Header.ParentID = unittest.IdentifierFixture()
	})
	qc := unittest.QuorumCertificateFixture(unittest.QCWithBlockID(root.ID()))

	rootSnapshot, err := inmem.SnapshotFromBootstrapState(root, result, seal, qc)
	require.NoError(t, err)

	util.RunWithBootstrapState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.State) {
		actual, err := state.Final().Identities(filter.Any)
		require.Nil(t, err)
		assert.ElementsMatch(t, expected, actual)
	})
}
