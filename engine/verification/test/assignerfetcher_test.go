package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/testutil"
	testmock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	vertestutils "github.com/onflow/flow-go/engine/verification/utils/unittest"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAssignerFetcherPipeline evaluates behavior of the pipeline of
// block reader -> block consumer -> assigner engine -> chunks queue -> chunks consumer -> chunks processor (i.e., fetcher engine)
// block reader receives (container) finalized blocks that contain execution receipts preceding (reference) blocks.
// some receipts have duplicate results.
// - in a staked verification node:
// -- for each distinct result assigner engine receives, it does the chunk assignment and passes the
// chunk locators of assigned chunks to chunk queue, which in turn delivers to chunks processor though the chunks consumer.
// - in an unstaked verification node:
// -- execution results are discarded.
// - it does a correct resource clean up of the pipeline after handling all incoming receipts
func TestAssignerFetcherPipeline(t *testing.T) {
	testcases := []struct {
		blockCount      int
		opts            []vertestutils.CompleteExecutionReceiptBuilderOpt
		msg             string
		staked          bool
		eventRepetition int // accounts for consumer being notified of a certain finalized block more than once.
	}{
		{
			// read this test case in this way:
			// one block is passed to block reader. The block contains one
			// execution result that is not duplicate (single copy).
			// The result has only one chunk.
			// The verification node is staked
			blockCount: 1,
			opts: []vertestutils.CompleteExecutionReceiptBuilderOpt{
				vertestutils.WithResults(1),
				vertestutils.WithChunksCount(1),
				vertestutils.WithCopies(1),
			},
			staked:          true,
			eventRepetition: 1,
			msg:             "1 block, 1 result, 1 chunk, no duplicate, staked, no event repetition",
		},
		{
			blockCount: 1,
			opts: []vertestutils.CompleteExecutionReceiptBuilderOpt{
				vertestutils.WithResults(1),
				vertestutils.WithChunksCount(1),
				vertestutils.WithCopies(1),
			},
			staked:          false, // unstaked
			eventRepetition: 1,
			msg:             "1 block, 1 result, 1 chunk, no duplicate, unstaked, no event repetition",
		},
		{
			blockCount: 1,
			opts: []vertestutils.CompleteExecutionReceiptBuilderOpt{
				vertestutils.WithResults(5),
				vertestutils.WithChunksCount(5),
				vertestutils.WithCopies(1),
			},
			staked:          true,
			eventRepetition: 1,
			msg:             "1 block, 5 result, 5 chunks, no duplicate, staked, no event repetition",
		},
		{
			blockCount: 10,
			opts: []vertestutils.CompleteExecutionReceiptBuilderOpt{
				vertestutils.WithResults(2),
				vertestutils.WithChunksCount(2),
				vertestutils.WithCopies(2),
			},
			staked:          true,
			eventRepetition: 1,
			msg:             "10 block, 5 result, 5 chunks, 1 duplicates, staked, no event repetition",
		},
		{
			blockCount: 10,
			opts: []vertestutils.CompleteExecutionReceiptBuilderOpt{
				vertestutils.WithResults(2),
				vertestutils.WithChunksCount(2),
				vertestutils.WithCopies(2),
			},
			staked:          true,
			eventRepetition: 3, // notifies consumer 3 times for each finalized block.
			msg:             "10 block, 5 result, 5 chunks, 1 duplicates, staked, with event repetition",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.msg, func(t *testing.T) {
			withConsumers(t, tc.staked, tc.blockCount, func(
				blockConsumer *blockconsumer.BlockConsumer,
				blocks []*flow.Block,
				resultApprovalsWG *sync.WaitGroup) {

				for i := 0; i < len(blocks)*tc.eventRepetition; i++ {
					// consumer is only required to be "notified" that a new finalized block available.
					// It keeps track of the last finalized block it has read, and read the next height upon
					// getting notified as follows:
					blockConsumer.OnFinalizedBlock(&model.Block{})
				}

				unittest.RequireReturnsBefore(t, resultApprovalsWG.Wait, time.Duration(2*tc.blockCount)*time.Second,
					"could not receive result approvals on time")

			}, tc.opts...)
		})
	}
}

//// withConsumers is a test helper that sets up the following pipeline:
//// block reader -> block consumer (3 workers) -> assigner engine -> chunks queue -> chunks consumer (3 workers) -> mock chunk processor
////
//// The block consumer operates on a block reader with a chain of specified number of finalized blocks
//// ready to read.
//func withConsumers(t *testing.T,
//	staked bool,
//	blockCount int,
//	withConsumers func(*blockconsumer.BlockConsumer,
//		*chunkconsumer.ChunkConsumer,
//		[]*flow.Block,
//		*sync.WaitGroup), ops ...utils.CompleteExecutionReceiptBuilderOpt) {
//	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
//		maxProcessing := int64(3)
//
//		// bootstraps
//		s, me, verId := bootstrapSystem(t, staked)
//
//		// generates a chain of blocks in the form of root <- R1 <- C1 <- R2 <- C2 <- ... where Rs are distinct reference
//		// blocks (i.e., containing guarantees), and Cs are container blocks for their preceding reference block,
//		// Container blocks only contain receipts of their preceding reference blocks. But they do not
//		// hold any guarantees.
//		root, err := s.State.Final().Head()
//		require.NoError(t, err)
//		completeERs := utils.CompleteExecutionReceiptChainFixture(t, root, blockCount, ops...)
//		blocks := ExtendStateWithFinalizedBlocks(t, completeERs, s.State)
//
//		// mocks chunk assigner to assign even chunk indices to this verification node
//		chunkAssigner := &mock.ChunkAssigner{}
//		expectedLocatorIds := MockChunkAssignmentFixture(chunkAssigner, flow.IdentityList{&verId}, completeERs, evenChunkIndexAssigner)
//
//		// chunk consumer and processor
//		processedIndex := bstorage.NewConsumerProgress(db, module.ConsumeProgressVerificationChunkIndex)
//		chunksQueue := bstorage.NewChunkQueue(db)
//		ok, err := chunksQueue.Init(chunkconsumer.DefaultJobIndex)
//		require.NoError(t, err)
//		require.True(t, ok)
//
//		chunkProcessor, chunksWg := mockChunkProcessor(t, expectedLocatorIds, staked)
//		chunkConsumer := chunkconsumer.NewChunkConsumer(
//			unittest.Logger(),
//			processedIndex,
//			chunksQueue,
//			chunkProcessor,
//			maxProcessing)
//
//		// assigner engine
//		collector := &metrics.NoopCollector{}
//		tracer := &trace.NoopTracer{}
//		assignerEng := assigner.New(
//			unittest.Logger(),
//			collector,
//			tracer,
//			me,
//			s.State,
//			chunkAssigner,
//			chunksQueue,
//			chunkConsumer)
//
//		// block consumer
//		processedHeight := bstorage.NewConsumerProgress(db, module.ConsumeProgressVerificationBlockHeight)
//		blockConsumer, _, err := blockconsumer.NewBlockConsumer(
//			unittest.Logger(),
//			processedHeight,
//			s.Storage.Blocks,
//			s.State,
//			assignerEng,
//			maxProcessing)
//		require.NoError(t, err)
//
//		withConsumers(blockConsumer, chunkConsumer, blocks, chunksWg)
//	})
//}

// withConsumers is a test helper that sets up the following pipeline:
// block reader -> block consumer (3 workers) -> assigner engine -> chunks queue -> chunks consumer (3 workers) -> mock chunk processor
//
// The block consumer operates on a block reader with a chain of specified number of finalized blocks
// ready to read.
func withConsumers(t *testing.T, staked bool, blockCount int,
	withBlockConsumer func(*blockconsumer.BlockConsumer, []*flow.Block, *sync.WaitGroup),
	ops ...vertestutils.CompleteExecutionReceiptBuilderOpt) {

	collector := &metrics.NoopCollector{}
	tracer := &trace.NoopTracer{}
	lg := unittest.Logger().With().Str("role", "verification").Logger()
	chainID := flow.Testnet

	// bootstraps system with one node of each role.
	s, verID, participants := bootstrapSystem(t, collector, tracer, staked)
	exeID := participants.Filter(filter.HasRole(flow.RoleExecution))[0]
	conID := participants.Filter(filter.HasRole(flow.RoleConsensus))[0]
	ops = append(ops, vertestutils.WithExecutorIDs(
		participants.Filter(filter.HasRole(flow.RoleExecution)).NodeIDs()))

	// generates a chain of blocks in the form of root <- R1 <- C1 <- R2 <- C2 <- ... where Rs are distinct reference
	// blocks (i.e., containing guarantees), and Cs are container blocks for their preceding reference block,
	// Container blocks only contain receipts of their preceding reference blocks. But they do not
	// hold any guarantees.
	root, err := s.State.Final().Head()
	require.NoError(t, err)
	completeERs := vertestutils.CompleteExecutionReceiptChainFixture(t, root, blockCount, ops...)
	blocks := vertestutils.ExtendStateWithFinalizedBlocks(t, completeERs, s.State)

	// chunk assignment
	chunkAssigner := &mock.ChunkAssigner{}
	assignedChunkIDs := flow.IdentifierList{}
	if staked {
		// only staked verification node has some chunks assigned to it.
		_, assignedChunkIDs = vertestutils.MockChunkAssignmentFixture(chunkAssigner,
			flow.IdentityList{verID},
			completeERs,
			vertestutils.EvenChunkIndexAssigner)
	}

	fmt.Println("total assigned chunks: ", len(assignedChunkIDs))

	hub := stub.NewNetworkHub()
	receiptsLimit := 100
	genericNode := testutil.GenericNodeWithStateFixture(t,
		s,
		hub,
		verID,
		lg,
		collector,
		tracer,
		chainID)

	// execution node
	exeNode, _ := vertestutils.SetupChunkDataPackProvider(t,
		hub,
		exeID,
		participants,
		chainID,
		completeERs,
		assignedChunkIDs,
		vertestutils.RespondChunkDataPackRequest)

	// consensus node
	conNode, _, conWG := vertestutils.SetupMockConsensusNode(t,
		hub,
		conID,
		flow.IdentityList{verID},
		participants,
		completeERs,
		chainID,
		assignedChunkIDs)

	verNode := testutil.NewVerificationNode(t,
		hub,
		verID,
		participants,
		chunkAssigner,
		uint(receiptsLimit),
		chainID,
		collector,
		collector,
		testutil.WithGenericNode(&genericNode))

	// turns on components and network
	verNet, ok := hub.GetNetwork(verID.NodeID)
	assert.True(t, ok)
	unittest.RequireReturnsBefore(t, func() {
		verNet.StartConDev(100*time.Millisecond, true)
	}, 100*time.Millisecond, "failed to start verification network")

	unittest.RequireComponentsReadyBefore(t, 1*time.Second,
		verNode.BlockConsumer,
		verNode.ChunkConsumer,
		verNode.AssignerEngine,
		verNode.FetcherEngine,
		verNode.RequesterEngine,
		verNode.VerifierEngine)

	withBlockConsumer(verNode.BlockConsumer, blocks, conWG)

	// tears down engines and nodes
	unittest.RequireReturnsBefore(t, verNet.StopConDev, 100*time.Millisecond, "failed to stop verification network")
	unittest.RequireComponentsDoneBefore(t, 100*time.Millisecond,
		verNode.BlockConsumer,
		verNode.ChunkConsumer,
		verNode.AssignerEngine,
		verNode.FetcherEngine,
		verNode.RequesterEngine,
		verNode.VerifierEngine)

	testmock.RequireGenericNodesDoneBefore(t, 1*time.Second,
		conNode,
		exeNode)
}

// bootstrapSystem is a test helper that bootstraps a flow system with node of each main roles (except execution nodes that are two).
// If staked set to true, it bootstraps verification node as an staked one.
// Otherwise, it bootstraps the verification node as unstaked in current epoch.
//
// As the return values, it returns the state, local module, and list of identities in system.
func bootstrapSystem(t *testing.T, collector *metrics.NoopCollector, tracer module.Tracer, staked bool) (*testmock.StateFixture, *flow.Identity,
	flow.IdentityList) {
	// creates identities to bootstrap system with
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	identities := unittest.CompleteIdentitySet(verID)
	identities = append(identities, unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))) // adds extra execution node

	// bootstraps the system
	stateFixture := testutil.CompleteStateFixture(t, collector, tracer, identities)

	if !staked {
		// creates a new verification node identity that is unstaked for this epoch
		verID = unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		identities = identities.Union(flow.IdentityList{verID})

		epochBuilder := unittest.NewEpochBuilder(t, stateFixture.State)
		epochBuilder.
			UsingSetupOpts(unittest.WithParticipants(identities)).
			BuildEpoch()
	}

	return stateFixture, verID, identities
}
