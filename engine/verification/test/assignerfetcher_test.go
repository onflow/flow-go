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
	"github.com/onflow/flow-go/engine/verification/assigner"
	"github.com/onflow/flow-go/engine/verification/assigner/blockconsumer"
	"github.com/onflow/flow-go/engine/verification/fetcher"
	mockfetcher "github.com/onflow/flow-go/engine/verification/fetcher/mock"
	"github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAssignerFetcherPipeline(t *testing.T) {
	testcases := []struct {
		blockCount,
		resultCount,
		chunkCount int
	}{
		{
			blockCount:  1,
			resultCount: 1,
			chunkCount:  1,
		},
		{
			blockCount:  1,
			resultCount: 1,
			chunkCount:  10,
		},
		{
			blockCount:  1,
			resultCount: 1,
			chunkCount:  10,
		},
		{
			blockCount:  10,
			resultCount: 1,
			chunkCount:  10,
		},
	}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%d-blocks %d-results %d-chunks", tc.blockCount, tc.resultCount, tc.chunkCount), func(t *testing.T) {
			withBlockConsumer(t, tc.blockCount, tc.resultCount, tc.chunkCount, func(blockConsumer *blockconsumer.BlockConsumer,
				chunkConsumer *fetcher.ChunkConsumer,
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
			})
		})
	}
}

func withBlockConsumer(t *testing.T, blockCount int, resultCount, chunkCount int,
	withConsumers func(*blockconsumer.BlockConsumer,
		*fetcher.ChunkConsumer,
		[]*flow.Block,
		*sync.WaitGroup)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		maxProcessing := int64(3)

		collector := &metrics.NoopCollector{}
		tracer := &trace.NoopTracer{}
		participants := unittest.IdentityListFixture(5, unittest.WithAllRoles())
		s := testutil.CompleteStateFixture(t, collector, tracer, participants)
		verId := participants.Filter(filter.HasRole(flow.RoleVerification))[0]
		me := testutil.LocalFixture(t, verId)
		chunkAssigner := &mock.ChunkAssigner{}

		// generates a chain of blocks in the form of root <- R1 <- C1 <- R2 <- C2 <- ... where Rs are distinct reference
		// blocks (i.e., containing guarantees), and Cs are container blocks for their preceding reference block,
		// Container blocks only contain receipts of their preceding reference blocks. But they do not
		// hold any guarantees.
		root, err := s.State.Params().Root()
		require.NoError(t, err)
		completeERs := utils.CompleteExecutionReceiptChainFixture(t, root, blockCount, chunkCount)
		blocks := ExtendStateWithFinalizedBlocks(t, completeERs, s.State)

		// mocks chunk assigner to assign even chunk indices to this verification node
		expectedLocatorIds := MockChunkAssignmentFixture(chunkAssigner, flow.IdentityList{verId}, completeERs, evenChunkIndexAssigner)

		// chunk consumer and processor
		processedIndex := bstorage.NewConsumerProgress(db, module.ConsumeProgressVerificationChunkIndex)
		chunksQueue := bstorage.NewChunkQueue(db)
		ok, err := chunksQueue.Init(fetcher.DefaultJobIndex)
		require.NoError(t, err)
		require.True(t, ok)

		chunkProcessor, chunksWg := mockChunkProcessor(t, expectedLocatorIds, true)
		chunkConsumer := fetcher.NewChunkConsumer(
			unittest.Logger(),
			processedIndex,
			chunksQueue,
			chunkProcessor,
			maxProcessing)

		// assigner engine
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
		fmt.Println("count: ", len(expectedLocatorIDs))
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
		fmt.Println("done")
	})

	return processor, &wg
}
