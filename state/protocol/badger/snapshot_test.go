package badger_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
	statepkg "github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/prg"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestUnknownReferenceBlock tests queries for snapshots which should be unknown.
// We use this fixture:
//   - Root height: 100
//   - Heights [100, 110] are finalized
//   - Height 111 is unfinalized
func TestUnknownReferenceBlock(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "kvstore: temporary broken")
	rootHeight := uint64(100)
	participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	rootSnapshot := unittest.RootSnapshotFixture(participants, func(block *flow.Block) {
		block.Header.Height = rootHeight
	})
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)

	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		// build some finalized non-root blocks (heights 101-110)
		head := unittest.BlockWithParentFixture(rootSnapshot.Encodable().Head)
		head.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		buildFinalizedBlock(t, state, head)

		const nBlocks = 10
		for i := 1; i < nBlocks; i++ {
			next := unittest.BlockWithParentProtocolState(head)
			buildFinalizedBlock(t, state, next)
			head = next
		}
		// build an unfinalized block (height 111)
		buildBlock(t, state, unittest.BlockWithParentProtocolState(head))

		finalizedHeader, err := state.Final().Head()
		require.NoError(t, err)

		t.Run("below root height", func(t *testing.T) {
			_, err := state.AtHeight(rootHeight - 1).Head()
			assert.ErrorIs(t, err, statepkg.ErrUnknownSnapshotReference)
		})
		t.Run("above finalized height, non-existent height", func(t *testing.T) {
			_, err := state.AtHeight(finalizedHeader.Height + 100).Head()
			assert.ErrorIs(t, err, statepkg.ErrUnknownSnapshotReference)
		})
		t.Run("above finalized height, existent height", func(t *testing.T) {
			_, err := state.AtHeight(finalizedHeader.Height + 1).Head()
			assert.ErrorIs(t, err, statepkg.ErrUnknownSnapshotReference)
		})
		t.Run("unknown block ID", func(t *testing.T) {
			_, err := state.AtBlockID(unittest.IdentifierFixture()).Head()
			assert.ErrorIs(t, err, statepkg.ErrUnknownSnapshotReference)
		})
	})
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
	unittest.SkipUnless(t, unittest.TEST_TODO, "kvstore: temporary broken")
	participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)

	expectedChainID := rootSnapshot.Params().ChainID()
	expectedSporkID := rootSnapshot.Params().SporkID()
	expectedProtocolVersion := rootSnapshot.Params().ProtocolVersion()

	rootHeader, err := rootSnapshot.Head()
	require.NoError(t, err)

	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		// build some non-root blocks
		head := rootHeader
		const nBlocks = 10
		for i := 0; i < nBlocks; i++ {
			next := unittest.BlockWithParentFixture(head)
			next.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			buildFinalizedBlock(t, state, next)
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
				chainID := snapshot.Params().ChainID()
				assert.Equal(t, expectedChainID, chainID)
			})
			t.Run("should be able to get spork ID from snapshot", func(t *testing.T) {
				sporkID := snapshot.Params().SporkID()
				assert.Equal(t, expectedSporkID, sporkID)
			})
			t.Run("should be able to get protocol version from snapshot", func(t *testing.T) {
				protocolVersion := snapshot.Params().ProtocolVersion()
				assert.Equal(t, expectedProtocolVersion, protocolVersion)
			})
		}
	})
}

// TestSnapshot_Descendants builds a sample chain with next structure:
//
//	A (finalized) <- B <- C <- D <- E <- F
//	              <- G <- H <- I <- J
//
// snapshot.Descendants has to return [B, C, D, E, F, G, H, I, J].
func TestSnapshot_Descendants(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "kvstore: temporary broken")
	participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		var expectedBlocks []flow.Identifier
		for i := 5; i > 3; i-- {
			parent := head
			for n := 0; n < i; n++ {
				block := unittest.BlockWithParentFixture(parent)
				block.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
				err := state.Extend(context.Background(), block)
				require.NoError(t, err)
				expectedBlocks = append(expectedBlocks, block.ID())
				parent = block.Header
			}
		}

		pendingBlocks, err := state.AtBlockID(head.ID()).Descendants()
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
			require.NoError(t, err)
			assert.ElementsMatch(t, identities, actual)
		})

		t.Run("single identity", func(t *testing.T) {
			expected := identities[rand.Intn(len(identities))]
			actual, err := state.Final().Identity(expected.NodeID)
			require.NoError(t, err)
			assert.Equal(t, expected, actual)
		})

		t.Run("filtered", func(t *testing.T) {
			sample, err := identities.SamplePct(0.1)
			require.NoError(t, err)
			filters := []flow.IdentityFilter[flow.Identity]{
				filter.HasRole[flow.Identity](flow.RoleCollection),
				filter.HasNodeID[flow.Identity](sample.NodeIDs()...),
				filter.HasInitialWeight[flow.Identity](true),
				filter.IsValidCurrentEpochParticipant,
			}

			for _, filterfunc := range filters {
				expected := identities.Filter(filterfunc)
				actual, err := state.Final().Identities(filterfunc)
				require.NoError(t, err)
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
	qc := unittest.QuorumCertificateFixture(unittest.QCWithRootBlockID(root.ID()))
	setup := result.ServiceEvents[0].Event.(*flow.EpochSetup)
	commit := result.ServiceEvents[1].Event.(*flow.EpochCommit)
	setup.Assignments = unittest.ClusterAssignment(uint(nClusters), collectors.ToSkeleton())
	clusterQCs := unittest.QuorumCertificatesFromAssignments(setup.Assignments)
	commit.ClusterQCs = flow.ClusterQCVoteDatasFromQCs(clusterQCs)
	seal.ResultID = result.ID()
	root.Payload.ProtocolStateID = inmem.ProtocolStateFromEpochServiceEvents(setup, commit).ID()

	rootSnapshot, err := inmem.SnapshotFromBootstrapState(root, result, seal, qc)
	require.NoError(t, err)

	util.RunWithBootstrapState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.State) {
		expectedClusters, err := factory.NewClusterList(setup.Assignments, collectors.ToSkeleton())
		require.NoError(t, err)
		actualClusters, err := state.Final().Epochs().Current().Clustering()
		require.NoError(t, err)

		require.Equal(t, nClusters, len(expectedClusters))
		require.Equal(t, len(expectedClusters), len(actualClusters))

		for i := 0; i < nClusters; i++ {
			expected := expectedClusters[i]
			actual := actualClusters[i]

			assert.Equal(t, len(expected), len(actual))
			assert.Equal(t, expected.ID(), actual.ID())
		}
	})
}

// TestSealingSegment tests querying sealing segment with respect to various snapshots.
//
// For each valid sealing segment, we also test bootstrapping with this sealing segment.
func TestSealingSegment(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "kvstore: temporary broken")
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
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
			assert.Empty(t, actual.ExtraBlocks)
			unittest.AssertEqualBlocksLenAndOrder(t, expected.Blocks, actual.Blocks)

			assertSealingSegmentBlocksQueryableAfterBootstrap(t, state.AtBlockID(head.ID()))
		})
	})

	// test sealing segment for non-root segment where the latest seal is the
	// root seal, but the segment contains more than the root block.
	// ROOT <- B1
	// Expected sealing segment: [ROOT, B1], extra blocks: []
	t.Run("non-root with root seal as latest seal", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			// build an extra block on top of root
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			buildFinalizedBlock(t, state, block1)

			segment, err := state.AtBlockID(block1.ID()).SealingSegment()
			require.NoError(t, err)

			// build a valid child B2 to ensure we have a QC
			buildBlock(t, state, unittest.BlockWithParentProtocolState(block1))

			// sealing segment should contain B1 and B2
			// B2 is reference of snapshot, B1 is latest sealed
			unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{rootSnapshot.Encodable().SealingSegment.Sealed(), block1}, segment.Blocks)
			assert.Len(t, segment.ExecutionResults, 1)
			assert.Empty(t, segment.ExtraBlocks)
			assertSealingSegmentBlocksQueryableAfterBootstrap(t, state.AtBlockID(block1.ID()))
		})
	})

	// test sealing segment for non-root segment with simple sealing structure
	// (no blocks in between reference block and latest sealed)
	// ROOT <- B1 <- B2(R1) <- B3(S1)
	// Expected sealing segment: [B1, B2, B3], extra blocks: [ROOT]
	t.Run("non-root", func(t *testing.T) {
		util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState, mutableState protocol.MutableProtocolState) {
			// build a block to seal
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			buildFinalizedBlock(t, state, block1)

			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)

			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receipt1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			buildFinalizedBlock(t, state, block2)

			// build a block sealing block1
			block3 := unittest.BlockWithParentProtocolState(block2)

			seals := []*flow.Seal{seal1}
			block3.SetPayload(flow.Payload{
				Seals:           seals,
				ProtocolStateID: calculateExpectedStateId(t, mutableState)(block3.Header, seals),
			})
			buildFinalizedBlock(t, state, block3)

			segment, err := state.AtBlockID(block3.ID()).SealingSegment()
			require.NoError(t, err)

			require.Len(t, segment.ExtraBlocks, 1)
			assert.Equal(t, segment.ExtraBlocks[0].Header.Height, head.Height)

			// build a valid child B3 to ensure we have a QC
			buildBlock(t, state, unittest.BlockWithParentProtocolState(block3))

			// sealing segment should contain B1, B2, B3
			// B3 is reference of snapshot, B1 is latest sealed
			unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{block1, block2, block3}, segment.Blocks)
			assert.Len(t, segment.ExecutionResults, 1)
			assertSealingSegmentBlocksQueryableAfterBootstrap(t, state.AtBlockID(block3.ID()))
		})
	})

	// test sealing segment for sealing segment with a large number of blocks
	// between the reference block and latest sealed
	// ROOT <- B1 <- .... <- BN(S1)
	// Expected sealing segment: [B1, ..., BN], extra blocks: [ROOT]
	t.Run("long sealing segment", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {

			// build a block to seal
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			buildFinalizedBlock(t, state, block1)

			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)

			parent := block1
			// build a large chain of intermediary blocks
			for i := 0; i < 100; i++ {
				next := unittest.BlockWithParentProtocolState(parent)
				if i == 0 {
					// Repetitions of the same receipt in one fork would be a protocol violation.
					// Hence, we include the result only once in the direct child of B1.
					next.SetPayload(unittest.PayloadFixture(
						unittest.WithReceipts(receipt1),
						unittest.WithProtocolStateID(parent.Payload.ProtocolStateID),
					))
				}
				buildFinalizedBlock(t, state, next)
				parent = next
			}

			// build the block sealing block 1
			blockN := unittest.BlockWithParentFixture(parent.Header)
			blockN.SetPayload(unittest.PayloadFixture(
				unittest.WithSeals(seal1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			buildFinalizedBlock(t, state, blockN)

			segment, err := state.AtBlockID(blockN.ID()).SealingSegment()
			require.NoError(t, err)

			assert.Len(t, segment.ExecutionResults, 1)
			// sealing segment should cover range [B1, BN]
			assert.Len(t, segment.Blocks, 102)
			assert.Len(t, segment.ExtraBlocks, 1)
			assert.Equal(t, segment.ExtraBlocks[0].Header.Height, head.Height)
			// first and last blocks should be B1, BN
			assert.Equal(t, block1.ID(), segment.Blocks[0].ID())
			assert.Equal(t, blockN.ID(), segment.Blocks[101].ID())
			assertSealingSegmentBlocksQueryableAfterBootstrap(t, state.AtBlockID(blockN.ID()))
		})
	})

	// test sealing segment where the segment blocks contain seals for
	// ancestor blocks prior to the sealing segment
	// ROOT <- B1 <- B2(R1) <- B3 <- B4(R2, S1) <- B5 <- B6(S2)
	// Expected sealing segment: [B2, B3, B4], Extra blocks: [ROOT, B1]
	t.Run("overlapping sealing segment", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {

			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			buildFinalizedBlock(t, state, block1)
			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)

			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receipt1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			buildFinalizedBlock(t, state, block2)

			receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)

			block3 := unittest.BlockWithParentProtocolState(block2)
			buildFinalizedBlock(t, state, block3)

			block4 := unittest.BlockWithParentFixture(block3.Header)
			block4.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receipt2),
				unittest.WithSeals(seal1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			buildFinalizedBlock(t, state, block4)

			block5 := unittest.BlockWithParentProtocolState(block4)
			buildFinalizedBlock(t, state, block5)

			block6 := unittest.BlockWithParentFixture(block5.Header)
			block6.SetPayload(unittest.PayloadFixture(
				unittest.WithSeals(seal2),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			buildFinalizedBlock(t, state, block6)

			segment, err := state.AtBlockID(block6.ID()).SealingSegment()
			require.NoError(t, err)

			// build a valid child to ensure we have a QC
			buildBlock(t, state, unittest.BlockWithParentProtocolState(block6))

			// sealing segment should be [B2, B3, B4, B5, B6]
			require.Len(t, segment.Blocks, 5)
			unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{block2, block3, block4, block5, block6}, segment.Blocks)
			unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{block1}, segment.ExtraBlocks[1:])
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
			block1.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receiptA1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))

			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receiptB),
				unittest.WithReceiptsAndNoResults(receiptA2),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			receiptC, sealC := unittest.ReceiptAndSealForBlock(block2)

			block3 := unittest.BlockWithParentFixture(block2.Header)
			block3.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receiptC),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))

			block4 := unittest.BlockWithParentProtocolState(block3)

			block5 := unittest.BlockWithParentFixture(block4.Header)
			block5.SetPayload(unittest.PayloadFixture(
				unittest.WithSeals(sealC),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))

			buildFinalizedBlock(t, state, block1)
			buildFinalizedBlock(t, state, block2)
			buildFinalizedBlock(t, state, block3)
			buildFinalizedBlock(t, state, block4)
			buildFinalizedBlock(t, state, block5)

			segment, err := state.AtBlockID(block5.ID()).SealingSegment()
			require.NoError(t, err)

			// build a valid child to ensure we have a QC
			buildBlock(t, state, unittest.BlockWithParentProtocolState(block5))

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
			block1.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receiptA1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))

			block2 := unittest.BlockWithParentFixture(block1.Header)
			block2.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receiptB),
				unittest.WithReceiptsAndNoResults(receiptA2),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))

			receiptForSeal, seal := unittest.ReceiptAndSealForBlock(block2)

			block3 := unittest.BlockWithParentFixture(block2.Header)
			block3.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receiptForSeal),
				unittest.WithReceiptsAndNoResults(receiptB2, receiptA3),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))

			block4 := unittest.BlockWithParentProtocolState(block3)

			block5 := unittest.BlockWithParentFixture(block4.Header)
			block5.SetPayload(unittest.PayloadFixture(
				unittest.WithSeals(seal),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))

			buildFinalizedBlock(t, state, block1)
			buildFinalizedBlock(t, state, block2)
			buildFinalizedBlock(t, state, block3)
			buildFinalizedBlock(t, state, block4)
			buildFinalizedBlock(t, state, block5)

			segment, err := state.AtBlockID(block5.ID()).SealingSegment()
			require.NoError(t, err)

			// build a valid child to ensure we have a QC
			buildBlock(t, state, unittest.BlockWithParentProtocolState(block5))

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
	// Expected sealing segment: [B1, B2, B3, B4, B5], Extra blocks: [ROOT]
	t.Run("sealing segment where highest block in segment does not seal lowest", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			// build a block to seal
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			buildFinalizedBlock(t, state, block1)

			// build a block sealing block1
			block2 := unittest.BlockWithParentFixture(block1.Header)
			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
			block2.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receipt1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			buildFinalizedBlock(t, state, block2)

			block3 := unittest.BlockWithParentProtocolState(block2)
			buildFinalizedBlock(t, state, block3)

			block4 := unittest.BlockWithParentFixture(block3.Header)
			block4.SetPayload(unittest.PayloadFixture(
				unittest.WithSeals(seal1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			buildFinalizedBlock(t, state, block4)

			block5 := unittest.BlockWithParentProtocolState(block4)
			buildFinalizedBlock(t, state, block5)

			snapshot := state.AtBlockID(block5.ID())

			// build a valid child to ensure we have a QC
			buildFinalizedBlock(t, state, unittest.BlockWithParentProtocolState(block5))

			segment, err := snapshot.SealingSegment()
			require.NoError(t, err)
			// sealing segment should contain B1 and B5
			// B5 is reference of snapshot, B1 is latest sealed
			unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{block1, block2, block3, block4, block5}, segment.Blocks)
			assert.Len(t, segment.ExecutionResults, 1)

			assertSealingSegmentBlocksQueryableAfterBootstrap(t, snapshot)
		})
	})
	// Test the case where the reference block of the snapshot contains seals for blocks that are lower than the lowest sealing segment's block.
	// This test case specifically checks if sealing segment includes both highest and lowest block sealed by head.
	// ROOT <- B1 <- B2 <- B3(Seal_B1) <- B4 <- ... <- LastBlock(Seal_B2, Seal_B3, Seal_B4)
	// Expected sealing segment: [B4, ..., B5], Extra blocks: [B2, B3]
	t.Run("highest block seals outside segment", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			// build a block to seal
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			buildFinalizedBlock(t, state, block1)

			// build a block sealing block1
			block2 := unittest.BlockWithParentFixture(block1.Header)
			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
			block2.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receipt1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			buildFinalizedBlock(t, state, block2)

			receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)
			block3 := unittest.BlockWithParentFixture(block2.Header)
			block3.SetPayload(unittest.PayloadFixture(
				unittest.WithSeals(seal1),
				unittest.WithReceipts(receipt2),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			buildFinalizedBlock(t, state, block3)

			receipt3, seal3 := unittest.ReceiptAndSealForBlock(block3)
			block4 := unittest.BlockWithParentFixture(block3.Header)
			block4.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receipt3),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			buildFinalizedBlock(t, state, block4)

			// build chain, so it's long enough to not target blocks as inside of flow.DefaultTransactionExpiry window.
			parent := block4
			for i := 0; i < 1.5*flow.DefaultTransactionExpiry; i++ {
				next := unittest.BlockWithParentProtocolState(parent)
				next.Header.View = next.Header.Height + 1 // set view so we are still in the same epoch
				buildFinalizedBlock(t, state, next)
				parent = next
			}

			receipt4, seal4 := unittest.ReceiptAndSealForBlock(block4)
			prevLastBlock := unittest.BlockWithParentFixture(parent.Header)
			prevLastBlock.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receipt4),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			buildFinalizedBlock(t, state, prevLastBlock)

			// since result and seal cannot be part of the same block, we need to build another block
			lastBlock := unittest.BlockWithParentFixture(prevLastBlock.Header)
			lastBlock.SetPayload(unittest.PayloadFixture(
				unittest.WithSeals(seal2, seal3, seal4),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			buildFinalizedBlock(t, state, lastBlock)

			snapshot := state.AtBlockID(lastBlock.ID())

			// build a valid child to ensure we have a QC
			buildFinalizedBlock(t, state, unittest.BlockWithParentProtocolState(lastBlock))

			segment, err := snapshot.SealingSegment()
			require.NoError(t, err)
			assert.Equal(t, lastBlock.Header, segment.Highest().Header)
			assert.Equal(t, block4.Header, segment.Sealed().Header)
			unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{block2, block3}, segment.ExtraBlocks)
			assert.Len(t, segment.ExecutionResults, 2)

			assertSealingSegmentBlocksQueryableAfterBootstrap(t, snapshot)
		})
	})
}

// TestSealingSegment_FailureCases verifies that SealingSegment construction fails with expected sentinel
// errors in case the caller violates the API contract:
//  1. The lowest block that can serve as head of a SealingSegment is the node's local root block.
//  2. Unfinalized blocks cannot serve as head of a SealingSegment. There are two distinct sub-cases:
//     (2a) A pending block is chosen as head; at this height no block has been finalized.
//     (2b) An orphaned block is chosen as head; at this height a block other than the orphaned has been finalized.
func TestSealingSegment_FailureCases(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "kvstore: temporary broken")
	sporkRootSnapshot := unittest.RootSnapshotFixture(unittest.CompleteIdentitySet())
	rootProtocolStateID := getRootProtocolStateID(t, sporkRootSnapshot)
	sporkRoot, err := sporkRootSnapshot.Head()
	require.NoError(t, err)

	// SCENARIO 1.
	// Here, we want to specifically test correct handling of the edge case, where a block exists in storage
	// that has _lower height_ than the node's local root block. Such blocks are typically contained in the
	// bootstrapping data, such that all entities referenced in the local root block can be resolved.
	// Is is possible to retrieve blocks that are lower than the local root block from storage, directly
	// via their ID. Despite these blocks existing in storage, SealingSegment construction should be
	// because the known history is potentially insufficient when going below the root block.
	t.Run("sealing segment from block below local state root", func(t *testing.T) {
		// Step I: constructing bootstrapping snapshot with some short history:
		//
		//          ╭───── finalized blocks ─────╮
		//    <-  b1  <-  b2(result(b1))  <-  b3(seal(b1))  <-
		//                                    └── head ──┘
		//
		b1 := unittest.BlockWithParentFixture(sporkRoot) // construct block b1, append to state and finalize
		b1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		receipt, seal := unittest.ReceiptAndSealForBlock(b1)
		b2 := unittest.BlockWithParentFixture(b1.Header) // construct block b2, append to state and finalize
		b2.SetPayload(unittest.PayloadFixture(
			unittest.WithReceipts(receipt),
			unittest.WithProtocolStateID(rootProtocolStateID),
		))
		b3 := unittest.BlockWithParentFixture(b2.Header) // construct block b3 with seal for b1, append it to state and finalize
		b3.SetPayload(unittest.PayloadFixture(
			unittest.WithSeals(seal),
			unittest.WithProtocolStateID(rootProtocolStateID),
		))

		multipleBlockSnapshot := snapshotAfter(t, sporkRootSnapshot, func(state *bprotocol.FollowerState, mutableState protocol.MutableProtocolState) protocol.Snapshot {
			for _, b := range []*flow.Block{b1, b2, b3} {
				buildFinalizedBlock(t, state, b)
			}
			b4 := unittest.BlockWithParentProtocolState(b3)
			require.NoError(t, state.ExtendCertified(context.Background(), b4, unittest.CertifyBlock(b4.Header))) // add child of b3 to ensure we have a QC for b3
			return state.AtBlockID(b3.ID())
		})

		// Step 2: bootstrapping new state based on sealing segment whose head is block b3.
		// Thereby, the state should have b3 as its local root block. In addition, the blocks contained in the sealing
		// segment, such as b2 should be stored in the state.
		util.RunWithFollowerProtocolState(t, multipleBlockSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			localStateRootBlock := state.Params().FinalizedRoot()
			assert.Equal(t, b3.ID(), localStateRootBlock.ID())

			// verify that b2 is known to the protocol state, but constructing a sealing segment fails
			_, err = state.AtBlockID(b2.ID()).Head()
			require.NoError(t, err)
			_, err = state.AtBlockID(b2.ID()).SealingSegment()
			assert.ErrorIs(t, err, protocol.ErrSealingSegmentBelowRootBlock)

			// lowest block that allows for sealing segment construction is root block:
			_, err = state.AtBlockID(b3.ID()).SealingSegment()
			require.NoError(t, err)
		})
	})

	// SCENARIO 2a: A pending block is chosen as head; at this height no block has been finalized.
	t.Run("sealing segment from unfinalized, pending block", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, sporkRootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			// add _unfinalized_ blocks b1 and b2 to state (block b5 is necessary, so b1 has a QC, which is a consistency requirement for subsequent finality)
			b1 := unittest.BlockWithParentFixture(sporkRoot)
			b1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			b2 := unittest.BlockWithParentFixture(b1.Header)
			b2.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			require.NoError(t, state.ExtendCertified(context.Background(), b1, b2.Header.QuorumCertificate()))
			require.NoError(t, state.ExtendCertified(context.Background(), b2, unittest.CertifyBlock(b2.Header))) // adding block b5 (providing required QC for b1)

			// consistency check: there should be no finalized block in the protocol state at height `b1.Height`
			_, err := state.AtHeight(b1.Header.Height).Head() // expect statepkg.ErrUnknownSnapshotReference as only finalized blocks are indexed by height
			assert.ErrorIs(t, err, statepkg.ErrUnknownSnapshotReference)

			// requesting a sealing segment from block b1 should fail, as b1 is not yet finalized
			_, err = state.AtBlockID(b1.ID()).SealingSegment()
			assert.True(t, protocol.IsUnfinalizedSealingSegmentError(err))
		})
	})

	// SCENARIO 2b: An orphaned block is chosen as head; at this height a block other than the orphaned has been finalized.
	t.Run("sealing segment from orphaned block", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, sporkRootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			orphaned := unittest.BlockWithParentFixture(sporkRoot)
			orphaned.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			orphanedChild := unittest.BlockWithParentProtocolState(orphaned)
			require.NoError(t, state.ExtendCertified(context.Background(), orphaned, orphanedChild.Header.QuorumCertificate()))
			require.NoError(t, state.ExtendCertified(context.Background(), orphanedChild, unittest.CertifyBlock(orphanedChild.Header)))
			block := unittest.BlockWithParentFixture(sporkRoot)
			block.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			buildFinalizedBlock(t, state, block)

			// consistency check: the finalized block at height `orphaned.Height` should be different than `orphaned`
			h, err := state.AtHeight(orphaned.Header.Height).Head()
			require.NoError(t, err)
			require.NotEqual(t, h.ID(), orphaned.ID())

			// requesting a sealing segment from orphaned block should fail, as it is not finalized
			_, err = state.AtBlockID(orphaned.ID()).SealingSegment()
			assert.True(t, protocol.IsUnfinalizedSealingSegmentError(err))
		})
	})

}

// TestBootstrapSealingSegmentWithExtraBlocks test sealing segment where the segment blocks contain collection
// guarantees referencing blocks prior to the sealing segment. After bootstrapping from sealing segment we should be able to
// extend with B7 with contains a guarantee referring B1.
// ROOT <- B1 <- B2(R1) <- B3 <- B4(S1) <- B5 <- B6(S2)
// Expected sealing segment: [B2, B3, B4, B5, B6], Extra blocks: [ROOT, B1]
func TestBootstrapSealingSegmentWithExtraBlocks(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "kvstore: temporary broken")
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	rootEpoch := rootSnapshot.Epochs().Current()
	cluster, err := rootEpoch.Cluster(0)
	require.NoError(t, err)
	collID := cluster.Members()[0].NodeID
	head, err := rootSnapshot.Head()
	require.NoError(t, err)
	util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		block1 := unittest.BlockWithParentFixture(head)
		block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
		buildFinalizedBlock(t, state, block1)
		receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)

		block2 := unittest.BlockWithParentFixture(block1.Header)
		block2.SetPayload(unittest.PayloadFixture(
			unittest.WithReceipts(receipt1),
			unittest.WithProtocolStateID(rootProtocolStateID),
		))
		buildFinalizedBlock(t, state, block2)

		receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)

		block3 := unittest.BlockWithParentProtocolState(block2)
		buildFinalizedBlock(t, state, block3)

		block4 := unittest.BlockWithParentFixture(block3.Header)
		block4.SetPayload(unittest.PayloadFixture(
			unittest.WithReceipts(receipt2),
			unittest.WithSeals(seal1),
			unittest.WithProtocolStateID(rootProtocolStateID),
		))
		buildFinalizedBlock(t, state, block4)

		block5 := unittest.BlockWithParentProtocolState(block4)
		buildFinalizedBlock(t, state, block5)

		block6 := unittest.BlockWithParentFixture(block5.Header)
		block6.SetPayload(unittest.PayloadFixture(
			unittest.WithSeals(seal2),
			unittest.WithProtocolStateID(rootProtocolStateID),
		))
		buildFinalizedBlock(t, state, block6)

		snapshot := state.AtBlockID(block6.ID())
		segment, err := snapshot.SealingSegment()
		require.NoError(t, err)

		// build a valid child to ensure we have a QC
		buildBlock(t, state, unittest.BlockWithParentProtocolState(block6))

		// sealing segment should be [B2, B3, B4, B5, B6]
		require.Len(t, segment.Blocks, 5)
		unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{block2, block3, block4, block5, block6}, segment.Blocks)
		unittest.AssertEqualBlocksLenAndOrder(t, []*flow.Block{block1}, segment.ExtraBlocks[1:])
		require.Len(t, segment.ExecutionResults, 1)

		assertSealingSegmentBlocksQueryableAfterBootstrap(t, snapshot)

		// bootstrap from snapshot
		util.RunWithFullProtocolState(t, snapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
			block7 := unittest.BlockWithParentFixture(block6.Header)
			guarantee := unittest.CollectionGuaranteeFixture(unittest.WithCollRef(block1.ID()))
			guarantee.ChainID = cluster.ChainID()

			signerIndices, err := signature.EncodeSignersToIndices(
				[]flow.Identifier{collID}, []flow.Identifier{collID})
			require.NoError(t, err)
			guarantee.SignerIndices = signerIndices

			block7.SetPayload(unittest.PayloadFixture(
				unittest.WithGuarantees(guarantee),
				unittest.WithProtocolStateID(block6.Payload.ProtocolStateID),
			))
			buildBlock(t, state, block7)
		})
	})
}

func TestLatestSealedResult(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "kvstore: temporary broken")
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)

	t.Run("root snapshot", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			gotResult, gotSeal, err := state.Final().SealedResult()
			require.NoError(t, err)
			expectedResult, expectedSeal, err := rootSnapshot.SealedResult()
			require.NoError(t, err)

			assert.Equal(t, expectedResult.ID(), gotResult.ID())
			assert.Equal(t, expectedSeal, gotSeal)
		})
	})

	t.Run("non-root snapshot", func(t *testing.T) {
		head, err := rootSnapshot.Head()
		require.NoError(t, err)

		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))

			block2 := unittest.BlockWithParentProtocolState(block1)

			receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
			block2.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receipt1),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			block3 := unittest.BlockWithParentFixture(block2.Header)
			block3.SetPayload(unittest.PayloadFixture(
				unittest.WithSeals(seal1),
				unittest.WithProtocolStateID(rootProtocolStateID)))

			receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)
			receipt3, seal3 := unittest.ReceiptAndSealForBlock(block3)
			block4 := unittest.BlockWithParentFixture(block3.Header)
			block4.SetPayload(unittest.PayloadFixture(
				unittest.WithReceipts(receipt2, receipt3),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))
			block5 := unittest.BlockWithParentFixture(block4.Header)
			block5.SetPayload(unittest.PayloadFixture(
				unittest.WithSeals(seal2, seal3),
				unittest.WithProtocolStateID(rootProtocolStateID),
			))

			err = state.ExtendCertified(context.Background(), block1, block2.Header.QuorumCertificate())
			require.NoError(t, err)

			err = state.ExtendCertified(context.Background(), block2, block3.Header.QuorumCertificate())
			require.NoError(t, err)

			err = state.ExtendCertified(context.Background(), block3, block4.Header.QuorumCertificate())
			require.NoError(t, err)

			// B1 <- B2(R1) <- B3(S1)
			// querying B3 should return result R1, seal S1
			t.Run("reference block contains seal", func(t *testing.T) {
				gotResult, gotSeal, err := state.AtBlockID(block3.ID()).SealedResult()
				require.NoError(t, err)
				assert.Equal(t, block2.Payload.Results[0], gotResult)
				assert.Equal(t, block3.Payload.Seals[0], gotSeal)
			})

			err = state.ExtendCertified(context.Background(), block4, block5.Header.QuorumCertificate())
			require.NoError(t, err)

			// B1 <- B2(S1) <- B3(S1) <- B4(R2,R3)
			// querying B3 should still return (R1,S1) even though they are in parent block
			t.Run("reference block contains no seal", func(t *testing.T) {
				gotResult, gotSeal, err := state.AtBlockID(block4.ID()).SealedResult()
				require.NoError(t, err)
				assert.Equal(t, &receipt1.ExecutionResult, gotResult)
				assert.Equal(t, seal1, gotSeal)
			})

			// B1 <- B2(R1) <- B3(S1) <- B4(R2,R3) <- B5(S2,S3)
			// There are two seals in B5 - should return latest by height (S3,R3)
			t.Run("reference block contains multiple seals", func(t *testing.T) {
				err = state.ExtendCertified(context.Background(), block5, unittest.CertifyBlock(block5.Header))
				require.NoError(t, err)

				gotResult, gotSeal, err := state.AtBlockID(block5.ID()).SealedResult()
				require.NoError(t, err)
				assert.Equal(t, &receipt3.ExecutionResult, gotResult)
				assert.Equal(t, seal3, gotSeal)
			})
		})
	})
}

// test retrieving quorum certificate and seed
func TestQuorumCertificate(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "kvstore: temporary broken")
	identities := unittest.IdentityListFixture(5, unittest.WithAllRoles())
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	rootProtocolStateID := getRootProtocolStateID(t, rootSnapshot)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)

	// should not be able to get QC or random beacon seed from a block with no children
	t.Run("no QC available", func(t *testing.T) {
		util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {

			// create a block to query
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			err := state.Extend(context.Background(), block1)
			require.NoError(t, err)

			_, err = state.AtBlockID(block1.ID()).QuorumCertificate()
			assert.ErrorIs(t, err, storage.ErrNotFound)

			_, err = state.AtBlockID(block1.ID()).RandomSource()
			assert.ErrorIs(t, err, storage.ErrNotFound)
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
			assert.Equal(t, len(randomSeed), prg.RandomSourceLength)
		})
	})

	// should be able to get QC and random beacon seed from a certified block
	t.Run("follower-block-processable", func(t *testing.T) {
		util.RunWithFollowerProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.FollowerState) {

			// add a block so we aren't testing against root
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			certifyingQC := unittest.CertifyBlock(block1.Header)
			err := state.ExtendCertified(context.Background(), block1, certifyingQC)
			require.NoError(t, err)

			// should be able to get QC/seed
			qc, err := state.AtBlockID(block1.ID()).QuorumCertificate()
			assert.NoError(t, err)

			assert.Equal(t, certifyingQC.SignerIndices, qc.SignerIndices)
			assert.Equal(t, certifyingQC.SigData, qc.SigData)
			assert.Equal(t, block1.Header.View, qc.View)

			_, err = state.AtBlockID(block1.ID()).RandomSource()
			require.NoError(t, err)
		})
	})

	// should be able to get QC and random beacon seed from a block with child(has to be certified)
	t.Run("participant-block-processable", func(t *testing.T) {
		util.RunWithFullProtocolState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
			// create a block to query
			block1 := unittest.BlockWithParentFixture(head)
			block1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)))
			err := state.Extend(context.Background(), block1)
			require.NoError(t, err)

			_, err = state.AtBlockID(block1.ID()).QuorumCertificate()
			assert.ErrorIs(t, err, storage.ErrNotFound)

			block2 := unittest.BlockWithParentProtocolState(block1)
			err = state.Extend(context.Background(), block2)
			require.NoError(t, err)

			qc, err := state.AtBlockID(block1.ID()).QuorumCertificate()
			require.NoError(t, err)

			// should have view matching block1 view
			assert.Equal(t, block1.Header.View, qc.View)
			assert.Equal(t, block1.ID(), qc.BlockID)
		})
	})
}

// test that we can query current/next/previous epochs from a snapshot
func TestSnapshot_EpochQuery(t *testing.T) {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	result, _, err := rootSnapshot.SealedResult()
	require.NoError(t, err)

	util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState, mutableState protocol.MutableProtocolState) {
		epoch1Counter := result.ServiceEvents[0].Event.(*flow.EpochSetup).Counter
		epoch2Counter := epoch1Counter + 1

		epochBuilder := unittest.NewEpochBuilder(t, mutableState, state)
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
					require.NoError(t, err)
					assert.Equal(t, epoch1Counter, counter)
				}
			})

			t.Run("epoch 2", func(t *testing.T) {
				for _, height := range epoch2.Range() {
					counter, err := state.AtHeight(height).Epochs().Current().Counter()
					require.NoError(t, err)
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
					require.NoError(t, err)
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
					require.NoError(t, err)
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

	util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState, mutableState protocol.MutableProtocolState) {

		epochBuilder := unittest.NewEpochBuilder(t, mutableState, state)
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
					require.NoError(t, err)
					assert.Equal(t, epoch1FirstView, actualFirstView)
				}
			})

			// test w.r.t. epoch 2 snapshot
			t.Run("Previous", func(t *testing.T) {
				for _, height := range epoch2.Range() {
					actualFirstView, err := state.AtHeight(height).Epochs().Previous().FirstView()
					require.NoError(t, err)
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
					require.NoError(t, err)
					assert.Equal(t, epoch2FirstView, actualFirstView)
				}
			})

			// test w.r.t. epoch 2 snapshot
			t.Run("Current", func(t *testing.T) {
				for _, height := range epoch2.Range() {
					actualFirstView, err := state.AtHeight(height).Epochs().Current().FirstView()
					require.NoError(t, err)
					assert.Equal(t, epoch2FirstView, actualFirstView)
				}
			})
		})
	})
}

// TestSnapshot_EpochHeightBoundaries tests querying epoch height boundaries in various conditions.
//   - FirstHeight should be queryable as soon as the epoch's first block is finalized,
//     otherwise should return protocol.ErrEpochTransitionNotFinalized
//   - FinalHeight should be queryable as soon as the next epoch's first block is finalized,
//     otherwise should return protocol.ErrEpochTransitionNotFinalized
func TestSnapshot_EpochHeightBoundaries(t *testing.T) {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	head, err := rootSnapshot.Head()
	require.NoError(t, err)

	util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState, mutableState protocol.MutableProtocolState) {

		epochBuilder := unittest.NewEpochBuilder(t, mutableState, state)

		epoch1FirstHeight := head.Height
		t.Run("first epoch - EpochStaking phase", func(t *testing.T) {
			// first height of started current epoch should be known
			firstHeight, err := state.Final().Epochs().Current().FirstHeight()
			require.NoError(t, err)
			assert.Equal(t, epoch1FirstHeight, firstHeight)
			// final height of not completed current epoch should be unknown
			_, err = state.Final().Epochs().Current().FinalHeight()
			assert.ErrorIs(t, err, protocol.ErrEpochTransitionNotFinalized)
		})

		// build first epoch (but don't complete it yet)
		epochBuilder.BuildEpoch()

		t.Run("first epoch - EpochCommitted phase", func(t *testing.T) {
			// first height of started current epoch should be known
			firstHeight, err := state.Final().Epochs().Current().FirstHeight()
			require.NoError(t, err)
			assert.Equal(t, epoch1FirstHeight, firstHeight)
			// final height of not completed current epoch should be unknown
			_, err = state.Final().Epochs().Current().FinalHeight()
			assert.ErrorIs(t, err, protocol.ErrEpochTransitionNotFinalized)
			// first and final height of not started next epoch should be unknown
			_, err = state.Final().Epochs().Next().FirstHeight()
			assert.ErrorIs(t, err, protocol.ErrEpochTransitionNotFinalized)
			_, err = state.Final().Epochs().Next().FinalHeight()
			assert.ErrorIs(t, err, protocol.ErrEpochTransitionNotFinalized)
		})

		// complete epoch 1 (enter epoch 2)
		epochBuilder.CompleteEpoch()
		epoch1Heights, ok := epochBuilder.EpochHeights(1)
		require.True(t, ok)
		epoch1FinalHeight := epoch1Heights.FinalHeight()
		epoch2FirstHeight := epoch1FinalHeight + 1

		t.Run("second epoch - EpochStaking phase", func(t *testing.T) {
			// first and final height of completed previous epoch should be known
			firstHeight, err := state.Final().Epochs().Previous().FirstHeight()
			require.NoError(t, err)
			assert.Equal(t, epoch1FirstHeight, firstHeight)
			finalHeight, err := state.Final().Epochs().Previous().FinalHeight()
			require.NoError(t, err)
			assert.Equal(t, epoch1FinalHeight, finalHeight)

			// first height of started current epoch should be known
			firstHeight, err = state.Final().Epochs().Current().FirstHeight()
			require.NoError(t, err)
			assert.Equal(t, epoch2FirstHeight, firstHeight)
			// final height of not completed current epoch should be unknown
			_, err = state.Final().Epochs().Current().FinalHeight()
			assert.ErrorIs(t, err, protocol.ErrEpochTransitionNotFinalized)
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
	removedAtEpoch2 := epoch1Identities[rand.Intn(len(epoch1Identities))]
	// epoch 2 has partial overlap with epoch 1
	epoch2Identities := append(
		epoch1Identities.Filter(filter.Not(filter.HasNodeID[flow.Identity](removedAtEpoch2.NodeID))),
		addedAtEpoch2)
	// epoch 3 has no overlap with epoch 2
	epoch3Identities := unittest.IdentityListFixture(10, unittest.WithAllRoles())

	rootSnapshot := unittest.RootSnapshotFixture(epoch1Identities)
	util.RunWithFullProtocolStateAndMutator(t, rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState, mutableState protocol.MutableProtocolState) {

		epochBuilder := unittest.NewEpochBuilder(t, mutableState, state)
		// build epoch 1 (prepare epoch 2)
		epochBuilder.
			UsingSetupOpts(unittest.WithParticipants(epoch2Identities.ToSkeleton())).
			BuildEpoch().
			CompleteEpoch()
		// build epoch 2 (prepare epoch 3)
		epochBuilder.
			UsingSetupOpts(unittest.WithParticipants(epoch3Identities.ToSkeleton())).
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(t, ok)
		epoch2, ok := epochBuilder.EpochHeights(2)
		require.True(t, ok)

		t.Run("should be able to query at root block", func(t *testing.T) {
			root := state.Params().FinalizedRoot()
			snapshot := state.AtHeight(root.Height)
			identities, err := snapshot.Identities(filter.Any)
			require.NoError(t, err)

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
				require.NoError(t, err)

				t.Run("phase: "+phase.String(), func(t *testing.T) {
					identities, err := snapshot.Identities(filter.Any)
					require.NoError(t, err)

					// should have the right number of identities
					assert.Equal(t, len(epoch1Identities)+1, len(identities))
					// all current epoch identities should match configuration from EpochSetup event
					assert.ElementsMatch(t, epoch1Identities, identities.Filter(epoch1Identities.Selector()))

					// should contain single identity for next epoch with status `flow.EpochParticipationStatusJoining`
					nextEpochIdentity := identities.Filter(filter.HasNodeID[flow.Identity](addedAtEpoch2.NodeID))[0]
					assert.Equal(t, flow.EpochParticipationStatusJoining, nextEpochIdentity.EpochParticipationStatus,
						"expect joining status since we are in setup & commit phase")
					assert.Equal(t, addedAtEpoch2.IdentitySkeleton, nextEpochIdentity.IdentitySkeleton,
						"expect skeleton to be identical")
				})
			}
		})

		t.Run("should include previous epoch in staking phase", func(t *testing.T) {

			// get a snapshot from staking phase of epoch 2
			snapshot := state.AtHeight(epoch2.Staking)
			identities, err := snapshot.Identities(filter.Any)
			require.NoError(t, err)

			// should have the right number of identities
			assert.Equal(t, len(epoch2Identities)+1, len(identities))
			// all current epoch identities should match configuration from EpochSetup event
			assert.ElementsMatch(t, epoch2Identities, identities.Filter(epoch2Identities.Selector()))

			// should contain single identity from previous epoch with status `flow.EpochParticipationStatusLeaving`
			lastEpochIdentity := identities.Filter(filter.HasNodeID[flow.Identity](removedAtEpoch2.NodeID))[0]
			assert.Equal(t, flow.EpochParticipationStatusLeaving, lastEpochIdentity.EpochParticipationStatus,
				"expect leaving status since we are in staking phase")
			assert.Equal(t, removedAtEpoch2.IdentitySkeleton, lastEpochIdentity.IdentitySkeleton,
				"expect skeleton to be identical")
		})

		t.Run("should not include previous epoch after staking phase", func(t *testing.T) {

			// get a snapshot from setup phase and commit phase of epoch 2
			snapshots := []protocol.Snapshot{state.AtHeight(epoch2.Setup), state.AtHeight(epoch2.Committed)}

			for _, snapshot := range snapshots {
				phase, err := snapshot.Phase()
				require.NoError(t, err)

				t.Run("phase: "+phase.String(), func(t *testing.T) {
					identities, err := snapshot.Identities(filter.Any)
					require.NoError(t, err)

					// should have the right number of identities
					assert.Equal(t, len(epoch2Identities)+len(epoch3Identities), len(identities))
					// all current epoch identities should match configuration from EpochSetup event
					assert.ElementsMatch(t, epoch2Identities, identities.Filter(epoch2Identities.Selector()))

					// should contain next epoch's identities with status `flow.EpochParticipationStatusJoining`
					for _, expected := range epoch3Identities {
						actual, exists := identities.ByNodeID(expected.NodeID)
						require.True(t, exists)
						assert.Equal(t, flow.EpochParticipationStatusJoining, actual.EpochParticipationStatus,
							"expect joining status since we are in setup & commit phase")
						assert.Equal(t, expected.IdentitySkeleton, actual.IdentitySkeleton,
							"expect skeleton to be identical")
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
	qc := unittest.QuorumCertificateFixture(unittest.QCWithRootBlockID(root.ID()))

	rootSnapshot, err := inmem.SnapshotFromBootstrapState(root, result, seal, qc)
	require.NoError(t, err)

	util.RunWithBootstrapState(t, rootSnapshot, func(db *badger.DB, state *bprotocol.State) {
		actual, err := state.Final().Identities(filter.Any)
		require.NoError(t, err)
		assert.ElementsMatch(t, expected, actual)
	})
}
