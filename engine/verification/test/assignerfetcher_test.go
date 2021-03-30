package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/testutil"
	testmock "github.com/onflow/flow-go/engine/testutil/mock"
	"github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	"github.com/onflow/flow-go/engine/verification/fetcher/chunkconsumer"
	mockfetcher "github.com/onflow/flow-go/engine/verification/fetcher/mock"
	"github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	bstorage "github.com/onflow/flow-go/storage/badger"
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
		blockCount int
		opts        []utils.CompleteExecutionReceiptBuilderOpt
		msg        string
		staked     bool
	}{
		{
			// read this test case in this way:
			// one block is passed to block reader. The block contains one
			// execution result that is not duplicate (single copy).
			// The result has only one chunk.
			// The verification node is staked
			blockCount: 1,
			ops: []utils.CompleteExecutionReceiptBuilderOpt{
				utils.WithResults(1),
				utils.WithChunks(1),
				utils.WithCopies(1),
			},
			staked: true,
			msg:    "1 block, 1 result, 1 chunk, no duplicate, staked",
		},
		{
			blockCount: 1,
			ops: []utils.CompleteExecutionReceiptBuilderOpt{
				utils.WithResults(1),
				utils.WithChunks(1),
				utils.WithCopies(1),
			},
			staked: false, // unstaked
			msg:    "1 block, 1 result, 1 chunk, no duplicate, unstaked",
		},
		{
			blockCount: 1,
			ops: []utils.CompleteExecutionReceiptBuilderOpt{
				utils.WithResults(5),
				utils.WithChunks(5),
				utils.WithCopies(1),
			},
			staked: true,
			msg:    "1 block, 5 result, 5 chunks, no duplicate, staked",
		},
		{
			blockCount: 10,
			ops: []utils.CompleteExecutionReceiptBuilderOpt{
				utils.WithResults(5),
				utils.WithChunks(5),
				utils.WithCopies(2),
			},
			staked: true,
			msg:    "10 block, 5 result, 5 chunks, 1 duplicates, staked",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.msg, func(t *testing.T) {
			withConsumers(t, tc.staked, tc.blockCount, func(
				blockConsumer *blockconsumer.BlockConsumer,
				chunkConsumer *chunkconsumer.ChunkConsumer,
				blocks []*flow.Block,
				wg *sync.WaitGroup) {

				unittest.RequireCloseBefore(t, chunkConsumer.Ready(), time.Second, "could not start chunk consumer")
				unittest.RequireCloseBefore(t, blockConsumer.Ready(), time.Second, "could not start block consumer")

				for i := 0; i < len(blocks); i++ {
					// consumer is only required to be "notified" that a new finalized block available.
					// It keeps track of the last finalized block it has read, and read the next height upon
					// getting notified as follows:
					blockConsumer.OnFinalizedBlock(&model.Block{})
				}

				unittest.RequireReturnsBefore(t, wg.Wait, time.Second, "could not receive all chunk locators on time")
				unittest.RequireCloseBefore(t, blockConsumer.Done(), time.Second, "could not terminate block consumer")
				unittest.RequireCloseBefore(t, chunkConsumer.Done(), time.Second, "could not terminate chunk consumer")

			}, tc.ops...)
		})
	}
}

// withConsumers is a test helper that sets up the following pipeline:
// block reader -> block consumer (3 workers) -> assigner engine -> chunks queue -> chunks consumer (3 workers) -> mock chunk processor
//
// The block consumer operates on a block reader with a chain of specified number of finalized blocks
// ready to read.
func withConsumers(t *testing.T,
	staked bool,
	blockCount int,
	withConsumers func(*blockconsumer.BlockConsumer,
		*chunkconsumer.ChunkConsumer,
		[]*flow.Block,
		*sync.WaitGroup), ops ...utils.CompleteExecutionReceiptBuilderOpt) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		maxProcessing := int64(3)

		// bootstraps
		s, me, verId := bootstrapSystem(t, staked)

		// generates a chain of blocks in the form of root <- R1 <- C1 <- R2 <- C2 <- ... where Rs are distinct reference
		// blocks (i.e., containing guarantees), and Cs are container blocks for their preceding reference block,
		// Container blocks only contain receipts of their preceding reference blocks. But they do not
		// hold any guarantees.
		root, err := s.State.Final().Head()
		require.NoError(t, err)
		completeERs := utils.CompleteExecutionReceiptChainFixture(t, root, blockCount, ops...)
		blocks := ExtendStateWithFinalizedBlocks(t, completeERs, s.State)

		// mocks chunk assigner to assign even chunk indices to this verification node
		chunkAssigner := &mock.ChunkAssigner{}
		expectedLocatorIds := MockChunkAssignmentFixture(chunkAssigner, flow.IdentityList{&verId}, completeERs, evenChunkIndexAssigner)

		// chunk consumer and processor
		processedIndex := bstorage.NewConsumerProgress(db, module.ConsumeProgressVerificationChunkIndex)
		chunksQueue := bstorage.NewChunkQueue(db)
		ok, err := chunksQueue.Init(chunkconsumer.DefaultJobIndex)
		require.NoError(t, err)
		require.True(t, ok)

		chunkProcessor, chunksWg := mockChunkProcessor(t, expectedLocatorIds, staked)
		chunkConsumer := chunkconsumer.NewChunkConsumer(
			unittest.Logger(),
			processedIndex,
			chunksQueue,
			chunkProcessor,
			maxProcessing)

		// assigner engine
		collector := &metrics.NoopCollector{}
		tracer := &trace.NoopTracer{}
		assignerEng := assigner.New(
			unittest.Logger(),
			collector,
			tracer,
			me,
			s.State,
			chunkAssigner,
			chunksQueue,
			chunkConsumer)

		// block consumer
		processedHeight := bstorage.NewConsumerProgress(db, module.ConsumeProgressVerificationBlockHeight)
		blockConsumer, _, err := blockconsumer.NewBlockConsumer(
			unittest.Logger(),
			processedHeight,
			s.Storage.Blocks,
			s.State,
			assignerEng,
			maxProcessing)
		require.NoError(t, err)

		withConsumers(blockConsumer, chunkConsumer, blocks, chunksWg)
	})
}

// mockChunkProcessor sets up a mock chunk processor that asserts the followings:
// - in a staked verification node:
// -- that a set of chunk locators are delivered to it.
// -- that each chunk locator is delivered only once.
// - in an unstaked verification node:
// -- no chunk locator is passed to it.
//
// mockChunkProcessor returns the mock chunk processor and a wait group that unblocks when all expected locators received.
func mockChunkProcessor(t testing.TB, expectedLocatorIDs flow.IdentifierList,
	staked bool) (*mockfetcher.AssignedChunkProcessor, *sync.WaitGroup) {
	processor := &mockfetcher.AssignedChunkProcessor{}

	// keeps track of which locators it has received
	receivedLocators := make(map[flow.Identifier]struct{})

	var (
		// decrements the wait group per distinct chunk locator received
		wg sync.WaitGroup
		// serializes processing locators (just for sake of test)
		mu sync.Mutex
	)

	if staked {
		// in staked mode, it expects chunk locators coming
		wg.Add(len(expectedLocatorIDs))
	}

	var notifier module.ProcessingNotifier
	processor.On("WithChunkConsumerNotifier", testifymock.Anything).Run(func(args testifymock.Arguments) {
		processingNotifier, ok := args[0].(module.ProcessingNotifier)
		require.True(t, ok)
		notifier = processingNotifier
	})

	processor.On("ProcessAssignedChunk", testifymock.Anything).Run(func(args testifymock.Arguments) {
		mu.Lock()
		defer mu.Unlock()

		// chunk processor (i.e., fetcher engine) should only receive a locator if the verification node is staked.
		require.True(t, staked, "unstaked fetcher engine received chunk locator")

		// the received entity should be an chunk locator
		locator, ok := args[0].(*chunks.Locator)
		assert.True(t, ok)

		locatorID := locator.ID()

		// verifies that it has not seen this locator
		_, duplicate := receivedLocators[locatorID]
		require.False(t, duplicate, fmt.Sprintf("chunk processor received duplicate locator: %x", locatorID))

		// ensures the received locator matches one we expect
		require.Contains(t, expectedLocatorIDs, locatorID, fmt.Sprintf("chunk processor unexpected locator: %x", locatorID))

		notifier.Notify(locatorID)

		wg.Done()
	})

	return processor, &wg
}

// bootstrapSystem is a test helper that bootstraps a flow system with one node of each main roles.
// If staked set to true, it bootstraps verification node as an staked one.
// Otherwise, it bootstraps the verification node as unstaked in current epoch.
//
// As the return values, it returns the state, local module, and identity of verification node.
func bootstrapSystem(t *testing.T, staked bool) (*testmock.StateFixture, module.Local, flow.Identity) {
	// creates identities to bootstrap system with
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	identities := unittest.CompleteIdentitySet(verID)

	// bootstraps the system
	collector := &metrics.NoopCollector{}
	tracer := &trace.NoopTracer{}
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

	me := testutil.LocalFixture(t, verID)

	return stateFixture, me, *verID
}
